package task

import (
	"context"
	"fmt"
	"net/url"
	"os"
	"path/filepath"
	"time"

	"github.com/google/uuid"

	"github.com/um4ru-ch4n/youtube-analyzer-mcp/internal/model"
	"github.com/um4ru-ch4n/youtube-analyzer-mcp/pkg/logger"
)

const defaultQueueSize = 100

// Manager implements the TaskService interface: task submission, status tracking,
// result retrieval, and deletion with a background worker pool.
type Manager struct {
	repo        TaskRepository
	pipeline    PipelineRunner
	queue       chan string
	workerCount int
	dataDir     string
}

// New creates a new Manager with the given dependencies.
func New(repo TaskRepository, pipeline PipelineRunner, workerCount int, dataDir string) *Manager {
	return &Manager{
		repo:        repo,
		pipeline:    pipeline,
		queue:       make(chan string, defaultQueueSize),
		workerCount: workerCount,
		dataDir:     dataDir,
	}
}

// Start launches the worker goroutines that process tasks from the queue.
// It blocks until ctx is cancelled, so call it in a separate goroutine.
func (m *Manager) Start(ctx context.Context) {
	logger.InfoKV(ctx, "starting task manager workers", "worker_count", m.workerCount)

	for i := 0; i < m.workerCount; i++ {
		go m.runWorker(ctx, i)
	}
}

// Submit creates a new task for the given video URL and enqueues it for processing.
func (m *Manager) Submit(ctx context.Context, videoURL string) (string, error) {
	taskID := uuid.New().String()

	now := time.Now().UTC()
	task := model.Task{
		ID:        taskID,
		VideoURL:  videoURL,
		Status:    model.TaskStatusQueued,
		Progress:  string(model.TaskStatusQueued),
		CreatedAt: now,
		UpdatedAt: now,
	}

	err := m.repo.Create(ctx, task)
	if err != nil {
		return "", fmt.Errorf("create task: %w", err)
	}

	select {
	case m.queue <- taskID:
		logger.InfoKV(ctx, "task enqueued", "task_id", taskID, "video_url", videoURL)
	case <-ctx.Done():
		return "", fmt.Errorf("enqueue task: %w", ctx.Err())
	}

	return taskID, nil
}

// SubmitLocal queues an already-uploaded local file for analysis. The path is
// encoded as `file://<path>#title=<urlencoded>` so the pipeline runner can
// recover both fields without widening the Task model or the worker queue.
func (m *Manager) SubmitLocal(ctx context.Context, path, title string) (string, error) {
	taskID := uuid.New().String()

	encoded := "file://" + path
	if title != "" {
		v := url.Values{}
		v.Set("title", title)
		encoded += "#" + v.Encode()
	}

	now := time.Now().UTC()
	task := model.Task{
		ID:        taskID,
		VideoURL:  encoded,
		Status:    model.TaskStatusQueued,
		Progress:  string(model.TaskStatusQueued),
		CreatedAt: now,
		UpdatedAt: now,
	}

	err := m.repo.Create(ctx, task)
	if err != nil {
		return "", fmt.Errorf("create local task: %w", err)
	}

	select {
	case m.queue <- taskID:
		logger.InfoKV(ctx, "local task enqueued", "task_id", taskID, "path", path, "title", title)
	case <-ctx.Done():
		return "", fmt.Errorf("enqueue local task: %w", ctx.Err())
	}

	return taskID, nil
}

// GetStatus returns the current state of the task.
func (m *Manager) GetStatus(ctx context.Context, taskID string) (model.Task, error) {
	return m.repo.Get(ctx, taskID)
}

// GetResult returns the analysis result for a completed task.
func (m *Manager) GetResult(ctx context.Context, taskID string) (model.TaskResult, error) {
	task, err := m.repo.Get(ctx, taskID)
	if err != nil {
		return model.TaskResult{}, err
	}

	if task.Status != model.TaskStatusCompleted {
		return model.TaskResult{}, model.ErrTaskNotCompleted
	}

	return m.repo.GetResult(ctx, taskID)
}

// Delete removes a task and its artifacts from storage and filesystem.
// Only tasks in a final state (completed or failed) can be deleted.
func (m *Manager) Delete(ctx context.Context, taskID string) error {
	task, err := m.repo.Get(ctx, taskID)
	if err != nil {
		return err
	}

	if !task.Status.IsFinal() {
		return model.ErrTaskNotDeletable
	}

	err = m.repo.Delete(ctx, taskID)
	if err != nil {
		return fmt.Errorf("delete task from repo: %w", err)
	}

	taskDir := filepath.Join(m.dataDir, "tasks", taskID)
	err = os.RemoveAll(taskDir)
	if err != nil {
		logger.WarnKV(ctx, "failed to remove task directory",
			"task_id", taskID,
			"path", taskDir,
			"error", err.Error(),
		)
	}

	logger.InfoKV(ctx, "task deleted", "task_id", taskID)

	return nil
}

// ListVideos returns all completed tasks with video metadata for cross-session discovery.
func (m *Manager) ListVideos(ctx context.Context, limit, offset int) ([]model.VideoSummary, error) {
	return m.repo.ListCompleted(ctx, limit, offset)
}

// Retry re-enqueues a failed task for processing. The pipeline will resume
// from the last checkpoint, skipping already completed steps.
func (m *Manager) Retry(ctx context.Context, taskID string) error {
	task, err := m.repo.Get(ctx, taskID)
	if err != nil {
		return err
	}

	if task.Status != model.TaskStatusFailed {
		return fmt.Errorf("can only retry failed tasks, current status: %s", task.Status)
	}

	if err := m.repo.UpdateStatus(ctx, taskID, model.TaskStatusQueued, "queued"); err != nil {
		return fmt.Errorf("reset task status: %w", err)
	}

	select {
	case m.queue <- taskID:
		logger.InfoKV(ctx, "task re-enqueued for retry", "task_id", taskID)
	case <-ctx.Done():
		return fmt.Errorf("enqueue retry: %w", ctx.Err())
	}

	return nil
}
