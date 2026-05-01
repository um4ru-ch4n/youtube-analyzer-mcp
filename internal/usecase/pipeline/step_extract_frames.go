package pipeline

import (
	"context"
	"fmt"

	"github.com/um4ru-ch4n/youtube-analyzer-mcp/pkg/logger"
)

func (r *Runner) extractFrames(ctx context.Context, state *State) error {
	logger.InfoKV(ctx, "extracting frames", "task_id", state.TaskID, "video_path", state.DownloadResult.VideoPath)

	frames, err := r.frameExtractor.ExtractFrames(ctx, state.DownloadResult.VideoPath, state.TempDir)
	if err != nil {
		return fmt.Errorf("extract frames: %w", err)
	}

	logger.InfoKV(ctx, "frames extracted before dedup", "task_id", state.TaskID, "count", len(frames))

	dedupedFrames, err := r.deduplicator.FilterDuplicates(ctx, frames)
	if err != nil {
		return fmt.Errorf("filter duplicate frames: %w", err)
	}

	state.Frames = dedupedFrames

	logger.InfoKV(ctx, "frame extraction complete",
		"task_id", state.TaskID,
		"total_extracted", len(frames),
		"after_dedup", len(dedupedFrames),
	)

	return nil
}
