package pipeline

import (
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/um4ru-ch4n/youtube-analyzer-mcp/internal/model"
	"github.com/um4ru-ch4n/youtube-analyzer-mcp/pkg/logger"
)

// prepareLocal substitutes the download step for already-uploaded files: copies
// the source into the task dir, extracts audio with ffmpeg, probes duration,
// and populates DownloadResult / VideoMeta so downstream steps stay uniform.
func (r *Runner) prepareLocal(ctx context.Context, state *State) error {
	logger.InfoKV(ctx, "preparing local video",
		"task_id", state.TaskID,
		"path", state.LocalPath,
	)

	src, err := os.Stat(state.LocalPath)
	if err != nil {
		return fmt.Errorf("stat local file: %w", err)
	}
	if src.IsDir() {
		return fmt.Errorf("local path is a directory: %s", state.LocalPath)
	}

	videoPath := filepath.Join(state.TempDir, "video.mp4")
	if err := copyFile(state.LocalPath, videoPath); err != nil {
		return fmt.Errorf("copy local video: %w", err)
	}

	audioPath := filepath.Join(state.TempDir, "audio.wav")
	if err := r.frameExtractor.ExtractAudio(ctx, videoPath, audioPath); err != nil {
		return fmt.Errorf("extract audio: %w", err)
	}

	duration := r.frameExtractor.GetDuration(ctx, videoPath)
	if r.cfg.MaxVideoDurationSec > 0 && duration > float64(r.cfg.MaxVideoDurationSec) {
		return model.ErrVideoTooLong
	}

	title := state.LocalTitle
	if title == "" {
		base := filepath.Base(state.LocalPath)
		title = strings.TrimSuffix(base, filepath.Ext(base))
	}

	meta := model.VideoMeta{
		Title:           title,
		DurationSeconds: duration,
		URL:             "file://" + state.LocalPath,
	}

	state.DownloadResult = model.DownloadResult{
		VideoPath: videoPath,
		AudioPath: audioPath,
		Meta:      meta,
	}
	state.VideoMeta = meta

	logger.InfoKV(ctx, "local video prepared",
		"task_id", state.TaskID,
		"title", title,
		"duration_sec", duration,
	)

	return nil
}

func copyFile(src, dst string) error {
	in, err := os.Open(src)
	if err != nil {
		return fmt.Errorf("open source: %w", err)
	}
	defer in.Close()

	out, err := os.OpenFile(dst, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0o644)
	if err != nil {
		return fmt.Errorf("create destination: %w", err)
	}
	defer out.Close()

	if _, err := io.Copy(out, in); err != nil {
		return fmt.Errorf("copy bytes: %w", err)
	}

	return out.Sync()
}
