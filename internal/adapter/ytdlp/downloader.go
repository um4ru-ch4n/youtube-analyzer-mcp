package ytdlp

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"os/exec"
	"path/filepath"

	"go.uber.org/zap"

	"github.com/um4ru-ch4n/youtube-analyzer-mcp/internal/model"
)

type Downloader struct {
	logger *zap.Logger
}

func New(logger *zap.Logger) *Downloader {
	return &Downloader{logger: logger}
}

type ytdlpMeta struct {
	Title    string  `json:"title"`
	Duration float64 `json:"duration"`
}

func (d *Downloader) Download(ctx context.Context, url, outputDir string) (model.DownloadResult, error) {
	videoPath := filepath.Join(outputDir, "video.mp4")
	audioPath := filepath.Join(outputDir, "audio.wav")

	videoArgs := []string{
		"-f", "bestvideo[height<=1080]+bestaudio/best",
		"-o", videoPath,
		url,
	}
	if err := d.runCmd(ctx, "yt-dlp", videoArgs); err != nil {
		return model.DownloadResult{}, fmt.Errorf("download video: %w", err)
	}

	audioArgs := []string{
		"-x", "--audio-format", "wav",
		"-o", audioPath,
		url,
	}
	if err := d.runCmd(ctx, "yt-dlp", audioArgs); err != nil {
		return model.DownloadResult{}, fmt.Errorf("extract audio: %w", err)
	}

	meta, err := d.fetchMeta(ctx, url)
	if err != nil {
		return model.DownloadResult{}, fmt.Errorf("fetch metadata: %w", err)
	}

	return model.DownloadResult{
		VideoPath: videoPath,
		AudioPath: audioPath,
		Meta: model.VideoMeta{
			Title:           meta.Title,
			DurationSeconds: meta.Duration,
			URL:             url,
		},
	}, nil
}

func (d *Downloader) fetchMeta(ctx context.Context, url string) (ytdlpMeta, error) {
	args := []string{"--dump-json", url}

	d.logger.Info("running yt-dlp metadata", zap.Strings("args", args))

	cmd := exec.CommandContext(ctx, "yt-dlp", args...)
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	if err := cmd.Run(); err != nil {
		d.logger.Error("yt-dlp metadata failed", zap.String("stderr", stderr.String()), zap.Error(err))
		return ytdlpMeta{}, fmt.Errorf("yt-dlp --dump-json: %w", err)
	}

	var meta ytdlpMeta
	if err := json.Unmarshal(stdout.Bytes(), &meta); err != nil {
		return ytdlpMeta{}, fmt.Errorf("parse yt-dlp json: %w", err)
	}

	return meta, nil
}

func (d *Downloader) runCmd(ctx context.Context, name string, args []string) error {
	d.logger.Info("running command", zap.String("cmd", name), zap.Strings("args", args))

	cmd := exec.CommandContext(ctx, name, args...)
	var stderr bytes.Buffer
	cmd.Stderr = &stderr

	if err := cmd.Run(); err != nil {
		d.logger.Error("command failed", zap.String("cmd", name), zap.String("stderr", stderr.String()), zap.Error(err))
		return fmt.Errorf("%s: %w", name, err)
	}

	return nil
}
