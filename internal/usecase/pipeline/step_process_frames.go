package pipeline

import (
	"context"
	"fmt"
	"math"
	"sync"
	"time"

	"github.com/um4ru-ch4n/youtube-analyzer-mcp/internal/model"
	"github.com/um4ru-ch4n/youtube-analyzer-mcp/pkg/logger"
)

// classifiedFrame holds a frame paired with its CLIP classification result.
type classifiedFrame struct {
	frame          model.Frame
	classification model.FrameClassification
}

func (r *Runner) processFrames(ctx context.Context, state *State) error {
	logger.InfoKV(ctx, "processing frames", "task_id", state.TaskID, "frame_count", len(state.Frames))

	if len(state.Frames) == 0 {
		return nil
	}

	maxConcurrent := r.cfg.MaxConcurrentFrames
	if maxConcurrent <= 0 {
		maxConcurrent = 4
	}

	// Phase 1: classify all frames + collect embeddings (one CLIP forward pass per frame).
	classified := r.classifyAll(ctx, state, maxConcurrent)

	// Phase 2: dedup by cosine similarity on embeddings.
	deduped := r.dedupByEmbedding(ctx, state.TaskID, classified)

	// Phase 3: heavy work (OCR / Vision API) only on survivors.
	processed := r.runHeavySteps(ctx, state, deduped, maxConcurrent)

	state.ProcessedFrames = processed

	usefulCount := 0
	for _, pf := range processed {
		if pf.Useful {
			usefulCount++
		}
	}

	logger.InfoKV(ctx, "frame processing complete",
		"task_id", state.TaskID,
		"total", len(processed),
		"useful", usefulCount,
	)

	// Free GPU memory on sidecars before the summarization step kicks in —
	// Ollama needs ~6GB on the same card for qwen3:8b, and PyTorch's reserved
	// memory pool will otherwise hold it hostage.
	r.releaseSidecarVRAM(ctx)

	return nil
}

func (r *Runner) releaseSidecarVRAM(ctx context.Context) {
	if releaser, ok := r.frameClassifier.(VRAMReleaser); ok {
		if err := releaser.ReleaseVRAM(ctx); err != nil {
			logger.WarnKV(ctx, "clip release_vram failed", "error", err.Error())
		}
	}
	if releaser, ok := r.ocrReader.(VRAMReleaser); ok {
		if err := releaser.ReleaseVRAM(ctx); err != nil {
			logger.WarnKV(ctx, "ocr release_vram failed", "error", err.Error())
		}
	}
}

// classifyAll runs CLIP /classify_with_embedding concurrently on every frame.
// Frames whose classification fails are passed through with FrameTypeOther
// and a warning, so the dedup/heavy stages can still handle them.
func (r *Runner) classifyAll(ctx context.Context, state *State, maxConcurrent int) []classifiedFrame {
	sem := make(chan struct{}, maxConcurrent)
	results := make([]classifiedFrame, len(state.Frames))
	var mu sync.Mutex
	var wg sync.WaitGroup

	for i, frame := range state.Frames {
		wg.Add(1)
		go func(idx int, f model.Frame) {
			defer wg.Done()
			sem <- struct{}{}
			defer func() { <-sem }()

			cls, err := r.frameClassifier.ClassifyWithEmbedding(ctx, f.FilePath)
			if err != nil {
				logger.WarnKV(ctx, "classify failed, treating as other",
					"task_id", state.TaskID,
					"frame_path", f.FilePath,
					"error", err.Error(),
				)
				mu.Lock()
				state.Warnings = append(state.Warnings, model.Warning{
					Step:      "process_frames",
					Message:   fmt.Sprintf("frame at %.1fs (%s): classify frame: %v", f.TimestampSec, f.FilePath, err),
					Timestamp: time.Now().UTC(),
				})
				mu.Unlock()
				cls = model.FrameClassification{Type: model.FrameTypeOther}
			}

			results[idx] = classifiedFrame{frame: f, classification: cls}
		}(i, frame)
	}

	wg.Wait()
	return results
}

// dedupByEmbedding drops frames whose CLIP embedding is too similar to the
// previously kept frame. Uses a greedy sequential pass — same strategy as the
// pHash deduplicator, but operating on semantic embeddings so visually
// similar slides with a moving talking-head overlay collapse to one frame.
//
// Frames without an embedding (classification failed) and talking_head frames
// are passed through untouched: talking_head gets dropped later (Useful=false)
// and we don't want to use its embedding as a dedup anchor against real slides.
func (r *Runner) dedupByEmbedding(ctx context.Context, taskID string, frames []classifiedFrame) []classifiedFrame {
	threshold := r.cfg.EmbeddingDedupThreshold
	if threshold <= 0 || len(frames) == 0 {
		return frames
	}

	kept := make([]classifiedFrame, 0, len(frames))
	var lastEmbedding []float32
	dropped := 0

	for _, cf := range frames {
		emb := cf.classification.Embedding

		// Talking heads aren't useful anchors — keep, but don't update anchor.
		if cf.classification.Type == model.FrameTypeTalkingHead {
			kept = append(kept, cf)
			continue
		}

		if len(emb) == 0 || lastEmbedding == nil {
			kept = append(kept, cf)
			if len(emb) > 0 {
				lastEmbedding = emb
			}
			continue
		}

		sim := cosineSimilarity(lastEmbedding, emb)
		if sim >= threshold {
			dropped++
			continue
		}

		kept = append(kept, cf)
		lastEmbedding = emb
	}

	logger.InfoKV(ctx, "embedding dedup complete",
		"task_id", taskID,
		"before", len(frames),
		"after", len(kept),
		"dropped", dropped,
		"threshold", threshold,
	)

	return kept
}

// runHeavySteps applies the type-specific work (OCR for slide/code, Vision
// API for diagram/other) to frames that survived dedup.
func (r *Runner) runHeavySteps(ctx context.Context, state *State, frames []classifiedFrame, maxConcurrent int) []model.ProcessedFrame {
	sem := make(chan struct{}, maxConcurrent)
	processed := make([]model.ProcessedFrame, 0, len(frames))
	var mu sync.Mutex
	var wg sync.WaitGroup

	for _, cf := range frames {
		wg.Add(1)
		go func(cf classifiedFrame) {
			defer wg.Done()
			sem <- struct{}{}
			defer func() { <-sem }()

			pf, err := r.applyHeavyStep(ctx, cf)
			if err != nil {
				logger.WarnKV(ctx, "frame heavy step failed, assuming useful",
					"task_id", state.TaskID,
					"frame_path", cf.frame.FilePath,
					"error", err.Error(),
				)
				mu.Lock()
				state.Warnings = append(state.Warnings, model.Warning{
					Step:      "process_frames",
					Message:   fmt.Sprintf("frame at %.1fs (%s): %v", cf.frame.TimestampSec, cf.frame.FilePath, err),
					Timestamp: time.Now().UTC(),
				})
				processed = append(processed, model.ProcessedFrame{
					TimestampSec: cf.frame.TimestampSec,
					Type:         model.FrameTypeOther,
					ImagePath:    cf.frame.FilePath,
					Useful:       true,
				})
				mu.Unlock()
				return
			}

			mu.Lock()
			processed = append(processed, pf)
			mu.Unlock()
		}(cf)
	}

	wg.Wait()
	return processed
}

func (r *Runner) applyHeavyStep(ctx context.Context, cf classifiedFrame) (model.ProcessedFrame, error) {
	frame := cf.frame
	frameType := cf.classification.Type

	if frameType == model.FrameTypeTalkingHead {
		return model.ProcessedFrame{
			TimestampSec: frame.TimestampSec,
			Type:         frameType,
			ImagePath:    frame.FilePath,
			Useful:       false,
		}, nil
	}

	if frameType == model.FrameTypeSlide || frameType == model.FrameTypeCode {
		text, ocrErr := r.ocrReader.ReadText(ctx, frame.FilePath)
		if ocrErr != nil {
			return model.ProcessedFrame{}, fmt.Errorf("OCR failed: %w", ocrErr)
		}

		return model.ProcessedFrame{
			TimestampSec: frame.TimestampSec,
			Type:         frameType,
			ImagePath:    frame.FilePath,
			OCRText:      text,
			Useful:       true,
		}, nil
	}

	useful, visionErr := r.visionAnalyzer.IsUsefulFrame(ctx, frame.FilePath)
	if visionErr != nil {
		logger.WarnKV(ctx, "vision usefulness check failed, assuming useful",
			"frame_path", frame.FilePath,
			"error", visionErr.Error(),
		)
		useful = true
	}

	return model.ProcessedFrame{
		TimestampSec: frame.TimestampSec,
		Type:         frameType,
		ImagePath:    frame.FilePath,
		Useful:       useful,
	}, nil
}

// cosineSimilarity returns the cosine similarity between two L2-normalized
// vectors. CLIP returns already-normalized embeddings, so this is just a dot
// product, but we don't assume normalization for safety.
func cosineSimilarity(a, b []float32) float64 {
	if len(a) != len(b) || len(a) == 0 {
		return 0
	}
	var dot, normA, normB float64
	for i := range a {
		af := float64(a[i])
		bf := float64(b[i])
		dot += af * bf
		normA += af * af
		normB += bf * bf
	}
	if normA == 0 || normB == 0 {
		return 0
	}
	return dot / (math.Sqrt(normA) * math.Sqrt(normB))
}
