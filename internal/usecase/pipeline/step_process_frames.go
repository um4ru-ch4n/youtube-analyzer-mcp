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
	contents := make([]model.FrameContent, 0, len(state.Frames))

	var wg sync.WaitGroup

	for i, frame := range state.Frames {
		wg.Add(1)

		go func(idx int, f model.Frame) {
			defer wg.Done()

			sem <- struct{}{}
			defer func() { <-sem }()

			content, err := r.processOneFrame(ctx, f)
			if err != nil {
				logger.WarnKV(ctx, "frame processing failed, skipping",
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
				mu.Unlock()

				return
			}

			mu.Lock()
			contents = append(contents, content)
			mu.Unlock()
		}(i, frame)
	}

	wg.Wait()

	state.FrameContents = contents

	logger.InfoKV(ctx, "frame processing complete",
		"task_id", state.TaskID,
		"processed", len(contents),
		"skipped", len(state.Frames)-len(contents),
	)

	return nil
}

func (r *Runner) processOneFrame(ctx context.Context, frame model.Frame) (model.FrameContent, error) {
	classification, err := r.frameClassifier.Classify(ctx, frame.FilePath)
	if err != nil {
		return model.FrameContent{}, fmt.Errorf("classify frame: %w", err)
	}

	frameType := classification.Type

	var content string

	if frameType == model.FrameTypeTalkingHead {
		content = "Talking head, no visual information"
	}

	if frameType == model.FrameTypeSlide || frameType == model.FrameTypeCode {
		text, ocrErr := r.ocrReader.ReadText(ctx, frame.FilePath)
		if ocrErr != nil {
			return model.FrameContent{}, fmt.Errorf("OCR failed: %w", ocrErr)
		}
		content = text
	}

	if frameType == model.FrameTypeDiagram || frameType == model.FrameTypeOther {
		prompt := "Describe the visual content of this image in detail. Focus on diagrams, charts, schemas, or any informational content."
		text, visionErr := r.visionAnalyzer.AnalyzeImage(ctx, frame.FilePath, prompt)
		if visionErr != nil {
			return model.FrameContent{}, fmt.Errorf("vision analysis failed: %w", visionErr)
		}
		content = text
	}

	return model.FrameContent{
		TimestampSec: frame.TimestampSec,
		Type:         frameType,
		Content:      content,
	}, nil
}
