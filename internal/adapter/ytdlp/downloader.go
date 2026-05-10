package ytdlp

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"go.uber.org/zap"

	"github.com/um4ru-ch4n/youtube-analyzer-mcp/internal/model"
)

type Downloader struct {
	logger      *zap.Logger
	cookiesFile string
}

func New(logger *zap.Logger) *Downloader {
	return &Downloader{
		logger:      logger,
		cookiesFile: os.Getenv("YTDLP_COOKIES"),
	}
}

func (d *Downloader) baseArgs() []string {
	args := []string{"--js-runtimes", "node", "--remote-components", "ejs:github"}
	if d.cookiesFile != "" {
		args = append(args, "--cookies", d.cookiesFile)
	}
	return args
}

type ytdlpMeta struct {
	Title    string  `json:"title"`
	Duration float64 `json:"duration"`
}

func (d *Downloader) Download(ctx context.Context, url, outputDir string) (model.DownloadResult, error) {
	videoPath := filepath.Join(outputDir, "video.mp4")
	audioPath := filepath.Join(outputDir, "audio.wav")

	videoArgs := append(d.baseArgs(), []string{
		"-f", "bestvideo[height<=1080]+bestaudio/best",
		"--merge-output-format", "mp4",
		"-o", videoPath,
		url,
	}...)
	if err := d.runCmd(ctx, "yt-dlp", videoArgs); err != nil {
		return model.DownloadResult{}, fmt.Errorf("download video: %w", err)
	}

	audioArgs := append(d.baseArgs(), []string{
		"-x", "--audio-format", "wav",
		"-o", audioPath,
		url,
	}...)
	if err := d.runCmd(ctx, "yt-dlp", audioArgs); err != nil {
		return model.DownloadResult{}, fmt.Errorf("extract audio: %w", err)
	}

	meta, err := d.fetchMeta(ctx, url)
	if err != nil {
		return model.DownloadResult{}, fmt.Errorf("fetch metadata: %w", err)
	}

	// Try to download subtitles (auto-subs or manual) — fast, saves Whisper time.
	subsPath := d.downloadSubtitles(ctx, url, outputDir)

	return model.DownloadResult{
		VideoPath:     videoPath,
		AudioPath:     audioPath,
		SubtitlesPath: subsPath,
		Meta: model.VideoMeta{
			Title:           meta.Title,
			DurationSeconds: meta.Duration,
			URL:             url,
		},
	}, nil
}

// downloadSubtitles tries to download subtitles via yt-dlp.
// Returns path to VTT file if successful, empty string otherwise.
func (d *Downloader) downloadSubtitles(ctx context.Context, url, outputDir string) string {
	subsBase := filepath.Join(outputDir, "subs")

	args := append(d.baseArgs(), []string{
		"--write-subs",
		"--write-auto-subs",
		"--sub-lang", "en,ru",
		"--sub-format", "vtt",
		"--skip-download",
		"-o", subsBase,
		url,
	}...)

	d.logger.Info("attempting subtitle download", zap.Strings("args", args))

	// Ignore exit code — yt-dlp may fail on some languages but still download others.
	_ = d.runCmd(ctx, "yt-dlp", args)

	// Check if any VTT files were created regardless of exit code.
	entries, err := os.ReadDir(outputDir)
	if err != nil {
		return ""
	}

	for _, e := range entries {
		if strings.HasPrefix(e.Name(), "subs.") && strings.HasSuffix(e.Name(), ".vtt") {
			vttPath := filepath.Join(outputDir, e.Name())
			d.logger.Info("subtitles downloaded", zap.String("path", vttPath))
			return vttPath
		}
	}

	d.logger.Info("no subtitle files found after download")
	return ""
}

func (d *Downloader) fetchMeta(ctx context.Context, url string) (ytdlpMeta, error) {
	args := append(d.baseArgs(), "--dump-json", url)

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
