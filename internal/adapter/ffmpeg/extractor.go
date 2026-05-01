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
	frames, err := e.extractWithSceneDetect(ctx, videoPath, outputDir)
	if err != nil {
		e.logger.Warn("scene detection failed, falling back to interval mode", zap.Error(err))
		return e.extractWithInterval(ctx, videoPath, outputDir)
	}

	if len(frames) < e.cfg.MinFrames || len(frames) > e.cfg.MaxFrames {
		e.logger.Info("scene detection frame count out of range, falling back to interval mode",
			zap.Int("got", len(frames)),
			zap.Int("min", e.cfg.MinFrames),
			zap.Int("max", e.cfg.MaxFrames),
		)
		if err := e.cleanFrames(outputDir); err != nil {
			e.logger.Warn("failed to clean frames before fallback", zap.Error(err))
		}
		return e.extractWithInterval(ctx, videoPath, outputDir)
	}

	return frames, nil
}

func (e *Extractor) extractWithSceneDetect(ctx context.Context, videoPath, outputDir string) ([]model.Frame, error) {
	pattern := filepath.Join(outputDir, "frame_%04d.jpg")
	filter := fmt.Sprintf("select='gt(scene,%f)',showinfo", e.cfg.SceneThreshold)

	args := []string{
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
		return nil, fmt.Errorf("ffmpeg scene detect: %w", err)
	}

	return e.collectFrames(outputDir, stderr.String())
}

func (e *Extractor) extractWithInterval(ctx context.Context, videoPath, outputDir string) ([]model.Frame, error) {
	pattern := filepath.Join(outputDir, "frame_%04d.jpg")
	filter := fmt.Sprintf("fps=1/%d", e.cfg.FallbackInterval)

	args := []string{
		"-i", videoPath,
		"-vf", filter,
		pattern,
	}

	e.logger.Info("running ffmpeg interval extraction", zap.Strings("args", args))

	cmd := exec.CommandContext(ctx, "ffmpeg", args...)
	var stderr bytes.Buffer
	cmd.Stderr = &stderr

	if err := cmd.Run(); err != nil {
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
