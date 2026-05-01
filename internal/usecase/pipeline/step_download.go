package pipeline

import (
	"context"
	"fmt"

	"github.com/um4ru-ch4n/youtube-analyzer-mcp/internal/model"
	"github.com/um4ru-ch4n/youtube-analyzer-mcp/pkg/logger"
)

func (r *Runner) download(ctx context.Context, state *State) error {
	logger.InfoKV(ctx, "downloading video", "task_id", state.TaskID, "url", state.VideoURL)

	result, err := r.downloader.Download(ctx, state.VideoURL, state.TempDir)
	if err != nil {
		return fmt.Errorf("download video: %w", err)
	}

	if r.cfg.MaxVideoDurationSec > 0 && result.Meta.DurationSeconds > float64(r.cfg.MaxVideoDurationSec) {
		return model.ErrVideoTooLong
	}

	state.DownloadResult = result
	state.VideoMeta = result.Meta

	logger.InfoKV(ctx, "download complete",
		"task_id", state.TaskID,
		"title", result.Meta.Title,
		"duration_sec", result.Meta.DurationSeconds,
	)

	return nil
}
