package task

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/um4ru-ch4n/youtube-analyzer-mcp/internal/model"
)

func newTestRepo(t *testing.T) *Repository {
	t.Helper()
	dir := t.TempDir()
	dbPath := filepath.Join(dir, "test.db")

	repo, err := New(dbPath)
	if err != nil {
		t.Fatalf("failed to create repo: %v", err)
	}

	t.Cleanup(func() { repo.Close() })
	return repo
}

func TestFullLifecycle(t *testing.T) {
	repo := newTestRepo(t)
	ctx := context.Background()
	now := time.Now().UTC().Truncate(time.Second)

	// Create
	task := model.Task{
		ID:        "task-001",
		VideoURL:  "https://youtube.com/watch?v=abc123",
		Status:    model.TaskStatusQueued,
		Progress:  "",
		Error:     "",
		Warnings:  []model.Warning{},
		CreatedAt: now,
		UpdatedAt: now,
	}

	if err := repo.Create(ctx, task); err != nil {
		t.Fatalf("Create: %v", err)
	}

	// Get
	got, err := repo.Get(ctx, "task-001")
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	if got.ID != task.ID {
		t.Errorf("ID = %q, want %q", got.ID, task.ID)
	}
	if got.VideoURL != task.VideoURL {
		t.Errorf("VideoURL = %q, want %q", got.VideoURL, task.VideoURL)
	}
	if got.Status != model.TaskStatusQueued {
		t.Errorf("Status = %q, want %q", got.Status, model.TaskStatusQueued)
	}

	// UpdateStatus
	if err := repo.UpdateStatus(ctx, "task-001", model.TaskStatusDownloading, "downloading video"); err != nil {
		t.Fatalf("UpdateStatus: %v", err)
	}

	got, err = repo.Get(ctx, "task-001")
	if err != nil {
		t.Fatalf("Get after UpdateStatus: %v", err)
	}
	if got.Status != model.TaskStatusDownloading {
		t.Errorf("Status = %q, want %q", got.Status, model.TaskStatusDownloading)
	}
	if got.Progress != "downloading video" {
		t.Errorf("Progress = %q, want %q", got.Progress, "downloading video")
	}

	// SaveResult
	result := model.TaskResult{
		VideoMeta: model.VideoMeta{
			Title:           "Test Video",
			DurationSeconds: 120.5,
			URL:             "https://youtube.com/watch?v=abc123",
		},
		Chunks: []model.ChunkSummary{
			{
				Index:     0,
				TimeStart: 0,
				TimeEnd:   45,
				Summary:   "First chunk summary",
			},
		},
		FullTranscript:  "full transcript here",
		TotalFrames:     10,
		DurationSeconds: 120.5,
	}

	if err := repo.SaveResult(ctx, "task-001", result, 5*time.Minute); err != nil {
		t.Fatalf("SaveResult: %v", err)
	}

	got, err = repo.Get(ctx, "task-001")
	if err != nil {
		t.Fatalf("Get after SaveResult: %v", err)
	}
	if got.Status != model.TaskStatusCompleted {
		t.Errorf("Status = %q, want %q", got.Status, model.TaskStatusCompleted)
	}

	// Delete
	if err := repo.Delete(ctx, "task-001"); err != nil {
		t.Fatalf("Delete: %v", err)
	}

	// Get (not found)
	_, err = repo.Get(ctx, "task-001")
	if !errors.Is(err, model.ErrTaskNotFound) {
		t.Errorf("Get after Delete: got err = %v, want ErrTaskNotFound", err)
	}
}

func TestGetNotFound(t *testing.T) {
	repo := newTestRepo(t)
	ctx := context.Background()

	_, err := repo.Get(ctx, "nonexistent")
	if !errors.Is(err, model.ErrTaskNotFound) {
		t.Errorf("Get nonexistent: got err = %v, want ErrTaskNotFound", err)
	}
}

func TestUpdateStatusNotFound(t *testing.T) {
	repo := newTestRepo(t)
	ctx := context.Background()

	err := repo.UpdateStatus(ctx, "nonexistent", model.TaskStatusDownloading, "")
	if !errors.Is(err, model.ErrTaskNotFound) {
		t.Errorf("UpdateStatus nonexistent: got err = %v, want ErrTaskNotFound", err)
	}
}

func TestDeleteNotFound(t *testing.T) {
	repo := newTestRepo(t)
	ctx := context.Background()

	err := repo.Delete(ctx, "nonexistent")
	if !errors.Is(err, model.ErrTaskNotFound) {
		t.Errorf("Delete nonexistent: got err = %v, want ErrTaskNotFound", err)
	}
}

func TestListMultipleTasks(t *testing.T) {
	repo := newTestRepo(t)
	ctx := context.Background()
	now := time.Now().UTC().Truncate(time.Second)

	for i := 0; i < 5; i++ {
		task := model.Task{
			ID:        "task-" + string(rune('A'+i)),
			VideoURL:  "https://youtube.com/watch?v=vid" + string(rune('A'+i)),
			Status:    model.TaskStatusQueued,
			Warnings:  []model.Warning{},
			CreatedAt: now.Add(time.Duration(i) * time.Second),
			UpdatedAt: now.Add(time.Duration(i) * time.Second),
		}
		if err := repo.Create(ctx, task); err != nil {
			t.Fatalf("Create task %d: %v", i, err)
		}
	}

	// List all
	tasks, err := repo.List(ctx, 10, 0)
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	if len(tasks) != 5 {
		t.Fatalf("List count = %d, want 5", len(tasks))
	}

	// Should be ordered by created_at DESC
	if tasks[0].ID != "task-E" {
		t.Errorf("first task ID = %q, want %q", tasks[0].ID, "task-E")
	}
	if tasks[4].ID != "task-A" {
		t.Errorf("last task ID = %q, want %q", tasks[4].ID, "task-A")
	}

	// List with limit and offset
	tasks, err = repo.List(ctx, 2, 1)
	if err != nil {
		t.Fatalf("List with limit/offset: %v", err)
	}
	if len(tasks) != 2 {
		t.Fatalf("List count = %d, want 2", len(tasks))
	}
	if tasks[0].ID != "task-D" {
		t.Errorf("first task ID = %q, want %q", tasks[0].ID, "task-D")
	}
}

func TestUpdateWarnings(t *testing.T) {
	repo := newTestRepo(t)
	ctx := context.Background()
	now := time.Now().UTC().Truncate(time.Second)

	task := model.Task{
		ID:        "task-warn",
		VideoURL:  "https://youtube.com/watch?v=warn",
		Status:    model.TaskStatusQueued,
		Warnings:  []model.Warning{},
		CreatedAt: now,
		UpdatedAt: now,
	}
	if err := repo.Create(ctx, task); err != nil {
		t.Fatalf("Create: %v", err)
	}

	warnings := []model.Warning{
		{Step: "ocr", Message: "OCR failed on frame at 12.5s", Timestamp: now},
		{Step: "clip", Message: "Low confidence classification", Timestamp: now},
	}

	if err := repo.UpdateWarnings(ctx, "task-warn", warnings); err != nil {
		t.Fatalf("UpdateWarnings: %v", err)
	}

	got, err := repo.Get(ctx, "task-warn")
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	if len(got.Warnings) != 2 {
		t.Fatalf("Warnings count = %d, want 2", len(got.Warnings))
	}
	if got.Warnings[0].Step != "ocr" {
		t.Errorf("Warnings[0].Step = %q, want %q", got.Warnings[0].Step, "ocr")
	}
	if got.Warnings[1].Message != "Low confidence classification" {
		t.Errorf("Warnings[1].Message = %q, want %q", got.Warnings[1].Message, "Low confidence classification")
	}
}

func TestUpdateWarningsNotFound(t *testing.T) {
	repo := newTestRepo(t)
	ctx := context.Background()

	err := repo.UpdateWarnings(ctx, "nonexistent", []model.Warning{})
	if !errors.Is(err, model.ErrTaskNotFound) {
		t.Errorf("UpdateWarnings nonexistent: got err = %v, want ErrTaskNotFound", err)
	}
}

func TestNewInvalidPath(t *testing.T) {
	// Try opening a DB in a nonexistent deep directory that we can't create.
	_, err := New("/nonexistent/deep/path/test.db")
	if err == nil {
		t.Error("expected error for invalid path, got nil")
	}
}

func TestDbFileCreated(t *testing.T) {
	dir := t.TempDir()
	dbPath := filepath.Join(dir, "created.db")

	repo, err := New(dbPath)
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	defer repo.Close()

	if _, err := os.Stat(dbPath); os.IsNotExist(err) {
		t.Error("expected db file to be created")
	}
}
