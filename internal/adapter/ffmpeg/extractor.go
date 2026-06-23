package ffmpeg

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"

	"go.uber.org/zap"

	"github.com/um4ru-ch4n/youtube-analyzer-mcp/internal/model"
)

type Config struct {
	SceneThreshold   float64
	MinFrames        int
	MaxFrames        int
	FallbackInterval int
}

type Extractor struct {
	cfg    Config
	logger *zap.Logger
}

func New(cfg Config, logger *zap.Logger) *Extractor {
	return &Extractor{cfg: cfg, logger: logger}
}

func (e *Extractor) ExtractFrames(ctx context.Context, videoPath, outputDir string) ([]model.Frame, error) {
	// Get video duration to calculate dynamic min frames.
	durationSec := e.getVideoDuration(ctx, videoPath)
	minFrames := e.dynamicMinFrames(durationSec)

	e.logger.Info("frame extraction starting",
		zap.Float64("video_duration_sec", durationSec),
		zap.Int("dynamic_min_frames", minFrames),
	)

	frames, err := e.extractWithSceneDetect(ctx, videoPath, outputDir)
	if err != nil {
		e.logger.Warn("scene detection failed, falling back to interval mode", zap.Error(err))
		return e.extractWithInterval(ctx, videoPath, outputDir)
	}

	if len(frames) < minFrames || len(frames) > e.cfg.MaxFrames {
		e.logger.Info("scene detection frame count out of range, falling back to interval mode",
			zap.Int("got", len(frames)),
			zap.Int("min", minFrames),
			zap.Int("max", e.cfg.MaxFrames),
		)
		if err := e.cleanFrames(outputDir); err != nil {
			e.logger.Warn("failed to clean frames before fallback", zap.Error(err))
		}
		return e.extractWithInterval(ctx, videoPath, outputDir)
	}

	return frames, nil
}

// dynamicMinFrames calculates minimum expected frames based on video duration.
// At least 1 frame per FallbackInterval seconds (same as interval mode would produce).
func (e *Extractor) dynamicMinFrames(durationSec float64) int {
	if durationSec <= 0 {
		return e.cfg.MinFrames
	}

	// Expect at least 1 frame per interval, with a floor of config MinFrames.
	expected := int(durationSec / float64(e.cfg.FallbackInterval))
	if expected < e.cfg.MinFrames {
		return e.cfg.MinFrames
	}
	return expected
}

// GetDuration uses ffprobe to read video duration in seconds. Returns 0 on failure.
func (e *Extractor) GetDuration(ctx context.Context, videoPath string) float64 {
	return e.getVideoDuration(ctx, videoPath)
}

// ExtractAudio decodes video audio into a 16 kHz mono PCM .wav matching the
// format yt-dlp produces, so the rest of the pipeline can stay source-agnostic.
func (e *Extractor) ExtractAudio(ctx context.Context, videoPath, outputPath string) error {
	args := []string{
		"-y",
		"-i", videoPath,
		"-vn",
		"-acodec", "pcm_s16le",
		"-ar", "16000",
		"-ac", "1",
		outputPath,
	}

	cmd := exec.CommandContext(ctx, "ffmpeg", args...)
	var stderr bytes.Buffer
	cmd.Stderr = &stderr

	if err := cmd.Run(); err != nil {
		e.logger.Error("ffmpeg audio extraction failed", zap.String("stderr", stderr.String()))
		return fmt.Errorf("ffmpeg extract audio: %w", err)
	}

	return nil
}

// getVideoDuration uses ffprobe to get video duration in seconds.
func (e *Extractor) getVideoDuration(ctx context.Context, videoPath string) float64 {
	args := []string{
		"-v", "error",
		"-show_entries", "format=duration",
		"-of", "default=noprint_wrappers=1:nokey=1",
		videoPath,
	}

	cmd := exec.CommandContext(ctx, "ffprobe", args...)
	var stdout bytes.Buffer
	cmd.Stdout = &stdout

	if err := cmd.Run(); err != nil {
		e.logger.Warn("ffprobe failed, using 0 duration", zap.Error(err))
		return 0
	}

	duration, err := strconv.ParseFloat(strings.TrimSpace(stdout.String()), 64)
	if err != nil {
		return 0
	}

	return duration
}

func (e *Extractor) extractWithSceneDetect(ctx context.Context, videoPath, outputDir string) ([]model.Frame, error) {
	pattern := filepath.Join(outputDir, "frame_%04d.jpg")
	filter := fmt.Sprintf("select='gt(scene,%f)',showinfo", e.cfg.SceneThreshold)

	args := []string{
		"-y",
		"-i", videoPath,
		"-vf", filter,
		"-vsync", "vfr",
		pattern,
	}

	e.logger.Info("running ffmpeg scene detection", zap.Strings("args", args))

	cmd := exec.CommandContext(ctx, "ffmpeg", args...)
	var stderr bytes.Buffer
	cmd.Stderr = &stderr

	if err := cmd.Run(); err != nil {
		e.logger.Debug("ffmpeg scene detect stderr", zap.String("stderr", stderr.String()))
		return nil, fmt.Errorf("ffmpeg scene detect: %w", err)
	}

	return e.collectFrames(outputDir, stderr.String())
}

func (e *Extractor) extractWithInterval(ctx context.Context, videoPath, outputDir string) ([]model.Frame, error) {
	pattern := filepath.Join(outputDir, "frame_%04d.jpg")
	filter := fmt.Sprintf("fps=1/%d", e.cfg.FallbackInterval)

	args := []string{
		"-y",
		"-i", videoPath,
		"-vf", filter,
		pattern,
	}

	e.logger.Info("running ffmpeg interval extraction", zap.Strings("args", args))

	cmd := exec.CommandContext(ctx, "ffmpeg", args...)
	var stderr bytes.Buffer
	cmd.Stderr = &stderr

	if err := cmd.Run(); err != nil {
		e.logger.Error("ffmpeg interval extraction failed", zap.String("stderr", stderr.String()))
		return nil, fmt.Errorf("ffmpeg interval extract: %w", err)
	}

	return e.collectFramesFromFilenames(outputDir)
}

var ptsTimeRegex = regexp.MustCompile(`pts_time:(\d+\.?\d*)`)

func (e *Extractor) collectFrames(outputDir, stderrOutput string) ([]model.Frame, error) {
	entries, err := os.ReadDir(outputDir)
	if err != nil {
		return nil, fmt.Errorf("read output dir: %w", err)
	}

	timestamps := ptsTimeRegex.FindAllStringSubmatch(stderrOutput, -1)

	var frames []model.Frame
	frameIdx := 0
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		matched, _ := filepath.Match("frame_*.jpg", entry.Name())
		if !matched {
			continue
		}

		var ts float64
		if frameIdx < len(timestamps) {
			ts, _ = strconv.ParseFloat(timestamps[frameIdx][1], 64)
		}

		frames = append(frames, model.Frame{
			TimestampSec: ts,
			FilePath:     filepath.Join(outputDir, entry.Name()),
		})
		frameIdx++
	}

	sort.Slice(frames, func(i, j int) bool {
		return frames[i].TimestampSec < frames[j].TimestampSec
	})

	return frames, nil
}

func (e *Extractor) collectFramesFromFilenames(outputDir string) ([]model.Frame, error) {
	entries, err := os.ReadDir(outputDir)
	if err != nil {
		return nil, fmt.Errorf("read output dir: %w", err)
	}

	var frames []model.Frame
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		matched, _ := filepath.Match("frame_*.jpg", entry.Name())
		if !matched {
			continue
		}

		var idx int
		_, _ = fmt.Sscanf(entry.Name(), "frame_%d.jpg", &idx)
		ts := float64((idx - 1) * e.cfg.FallbackInterval)

		frames = append(frames, model.Frame{
			TimestampSec: ts,
			FilePath:     filepath.Join(outputDir, entry.Name()),
		})
	}

	sort.Slice(frames, func(i, j int) bool {
		return frames[i].TimestampSec < frames[j].TimestampSec
	})

	return frames, nil
}

func (e *Extractor) cleanFrames(dir string) error {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return err
	}

	for _, entry := range entries {
		matched, _ := filepath.Match("frame_*.jpg", entry.Name())
		if !matched {
			continue
		}
		_ = os.Remove(filepath.Join(dir, entry.Name()))
	}

	return nil
}
