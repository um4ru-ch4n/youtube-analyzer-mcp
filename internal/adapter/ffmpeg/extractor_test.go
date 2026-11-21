package ffmpeg

import (
	"context"
	"os/exec"
	"path/filepath"
	"testing"

	"go.uber.org/zap"
)

func TestExtractWithIntervalTimestamps(t *testing.T) {
	t.Parallel()

	if _, err := exec.LookPath("ffmpeg"); err != nil {
		t.Skip("ffmpeg not installed")
	}

	dir := t.TempDir()
	video := filepath.Join(dir, "in.mp4")
	err := exec.Command("ffmpeg", "-v", "error", "-f", "lavfi",
		"-i", "testsrc=size=64x64:rate=25:duration=21", "-pix_fmt", "yuv420p", video).Run()
	if err != nil {
		t.Fatalf("generate video: %v", err)
	}

	e := New(Config{FallbackInterval: 5}, zap.NewNop())

	frames, err := e.extractWithInterval(context.Background(), video, dir)
	if err != nil {
		t.Fatalf("extractWithInterval: %v", err)
	}

	want := []float64{0, 5, 10, 15, 20}
	if len(frames) != len(want) {
		t.Fatalf("got %d frames, want %d: %+v", len(frames), len(want), frames)
	}
	for i, f := range frames {
		if d := f.TimestampSec - want[i]; d < -0.1 || d > 0.1 {
			t.Errorf("frame %d timestamp = %.2f, want %.0f", i, f.TimestampSec, want[i])
		}
	}
}
