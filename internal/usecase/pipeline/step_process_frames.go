package pipeline

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/um4ru-ch4n/youtube-analyzer-mcp/internal/model"
	"github.com/um4ru-ch4n/youtube-analyzer-mcp/pkg/logger"
)

func (r *Runner) processFrames(ctx context.Context, state *State) error {
	logger.InfoKV(ctx, "processing frames", "task_id", state.TaskID, "frame_count", len(state.Frames))

	if len(state.Frames) == 0 {
		return nil
	}

	maxConcurrent := r.cfg.MaxConcurrentFrames
	if maxConcurrent <= 0 {
		maxConcurrent = 4
	}

	sem := make(chan struct{}, maxConcurrent)
	var mu sync.Mutex
	processed := make([]model.ProcessedFrame, 0, len(state.Frames))

	var wg sync.WaitGroup

	for i, frame := range state.Frames {
		wg.Add(1)

		go func(idx int, f model.Frame) {
			defer wg.Done()

			sem <- struct{}{}
			defer func() { <-sem }()

			pf, err := r.processOneFrame(ctx, f)
			if err != nil {
				logger.WarnKV(ctx, "frame processing failed, assuming useful",
					"task_id", state.TaskID,
					"frame_index", idx,
					"frame_path", f.FilePath,
					"frame_timestamp", f.TimestampSec,
					"error", err.Error(),
				)

				mu.Lock()
				state.Warnings = append(state.Warnings, model.Warning{
					Step:      "process_frames",
					Message:   fmt.Sprintf("frame at %.1fs (%s): %v", f.TimestampSec, f.FilePath, err),
					Timestamp: time.Now().UTC(),
				})
				// On error, assume useful so Claude can decide later.
				processed = append(processed, model.ProcessedFrame{
					TimestampSec: f.TimestampSec,
					Type:         model.FrameTypeOther,
					ImagePath:    f.FilePath,
					Useful:       true,
				})
				mu.Unlock()

				return
			}

			mu.Lock()
			processed = append(processed, pf)
			mu.Unlock()
		}(i, frame)
	}

	wg.Wait()

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

	return nil
}

func (r *Runner) processOneFrame(ctx context.Context, frame model.Frame) (model.ProcessedFrame, error) {
	classification, err := r.frameClassifier.Classify(ctx, frame.FilePath)
	if err != nil {
		return model.ProcessedFrame{}, fmt.Errorf("classify frame: %w", err)
	}

	frameType := classification.Type

	// talking_head -> not useful, skip
	if frameType == model.FrameTypeTalkingHead {
		return model.ProcessedFrame{
			TimestampSec: frame.TimestampSec,
			Type:         frameType,
			ImagePath:    frame.FilePath,
			Useful:       false,
		}, nil
	}

	// slide/code -> run OCR, mark useful
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

	// diagram/other -> ask Vision API if useful; if unavailable, assume useful
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
