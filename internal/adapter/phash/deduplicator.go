package phash

import (
	"context"
	"fmt"
	"image"
	_ "image/jpeg"
	_ "image/png"
	"os"

	"github.com/corona10/goimagehash"
	"go.uber.org/zap"

	"github.com/um4ru-ch4n/youtube-analyzer-mcp/internal/model"
)

type Deduplicator struct {
	threshold float64
	logger    *zap.Logger
}

func New(threshold float64, logger *zap.Logger) *Deduplicator {
	return &Deduplicator{threshold: threshold, logger: logger}
}

func (d *Deduplicator) FilterDuplicates(_ context.Context, frames []model.Frame) ([]model.Frame, error) {
	if len(frames) == 0 {
		return frames, nil
	}

	hashes := make([]*goimagehash.ImageHash, 0, len(frames))
	for _, f := range frames {
		h, err := computeHash(f.FilePath)
		if err != nil {
			return nil, fmt.Errorf("hash %s: %w", f.FilePath, err)
		}
		hashes = append(hashes, h)
	}

	filtered := []model.Frame{frames[0]}
	lastKeptIdx := 0

	for i := 1; i < len(frames); i++ {
		dist, err := hashes[lastKeptIdx].Distance(hashes[i])
		if err != nil {
			d.logger.Warn("hash distance error, keeping frame", zap.Error(err))
			filtered = append(filtered, frames[i])
			lastKeptIdx = i
			continue
		}

		if float64(dist) <= d.threshold {
			d.logger.Debug("dropping duplicate frame",
				zap.String("path", frames[i].FilePath),
				zap.Int("distance", dist),
			)
			continue
		}

		filtered = append(filtered, frames[i])
		lastKeptIdx = i
	}

	d.logger.Info("deduplication complete",
		zap.Int("before", len(frames)),
		zap.Int("after", len(filtered)),
	)

	return filtered, nil
}

func computeHash(path string) (*goimagehash.ImageHash, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, fmt.Errorf("open image: %w", err)
	}
	defer f.Close()

	img, _, err := image.Decode(f)
	if err != nil {
		return nil, fmt.Errorf("decode image: %w", err)
	}

	return goimagehash.PerceptionHash(img)
}
