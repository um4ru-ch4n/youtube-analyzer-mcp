package task

import (
	"context"
	"reflect"
	"testing"
	"time"

	"github.com/um4ru-ch4n/youtube-analyzer-mcp/internal/model"
)

func TestTaskOptionsRoundTrip(t *testing.T) {
	t.Parallel()

	repo := newTestRepo(t)
	ctx := context.Background()
	now := time.Now().UTC().Truncate(time.Second)

	opts := model.TaskOptions{
		SkipSummary:         true,
		Language:            "ru",
		Glossary:            []string{"Uniswap", "USDC"},
		InitialPrompt:       "Урок про Uniswap.",
		FrameDedupThreshold: 0.98,
	}

	err := repo.Create(ctx, model.Task{
		ID: "opt-1", VideoURL: "file:///data/uploads/a.mp4", Status: model.TaskStatusQueued,
		Options: opts, CreatedAt: now, UpdatedAt: now,
	})
	if err != nil {
		t.Fatalf("Create: %v", err)
	}

	got, err := repo.Get(ctx, "opt-1")
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	if !reflect.DeepEqual(got.Options, opts) {
		t.Errorf("Get options = %+v, want %+v", got.Options, opts)
	}

	list, err := repo.List(ctx, 10, 0)
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	if len(list) != 1 || !reflect.DeepEqual(list[0].Options, opts) {
		t.Errorf("List options = %+v, want %+v", list, opts)
	}
}

func TestResultSegmentsRoundTrip(t *testing.T) {
	t.Parallel()

	repo := newTestRepo(t)
	ctx := context.Background()
	now := time.Now().UTC()

	err := repo.Create(ctx, model.Task{ID: "seg-1", VideoURL: "u", Status: model.TaskStatusQueued, CreatedAt: now, UpdatedAt: now})
	if err != nil {
		t.Fatalf("Create: %v", err)
	}

	result := model.TaskResult{
		FullTranscript: "привет мир",
		Segments:       []model.TranscriptSegment{{StartSec: 0, EndSec: 1.5, Text: "привет"}, {StartSec: 1.5, EndSec: 3, Text: "мир"}},
		Language:       "ru",
	}

	err = repo.SaveResult(ctx, "seg-1", result, time.Second)
	if err != nil {
		t.Fatalf("SaveResult: %v", err)
	}

	got, err := repo.GetResult(ctx, "seg-1")
	if err != nil {
		t.Fatalf("GetResult: %v", err)
	}
	if !reflect.DeepEqual(got.Segments, result.Segments) || got.Language != "ru" {
		t.Errorf("GetResult = %+v, want segments %+v", got, result.Segments)
	}
}
