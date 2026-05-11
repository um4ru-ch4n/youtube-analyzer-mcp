package task

import (
	"context"
	"time"

	"github.com/um4ru-ch4n/youtube-analyzer-mcp/internal/model"
)

// TaskRepository defines persistence operations for tasks.
type TaskRepository interface {
	Create(ctx context.Context, task model.Task) error
	Get(ctx context.Context, taskID string) (model.Task, error)
	UpdateStatus(ctx context.Context, taskID string, status model.TaskStatus, progress string) error
	UpdateWarnings(ctx context.Context, taskID string, warnings []model.Warning) error
	SaveResult(ctx context.Context, taskID string, result model.TaskResult, processingDuration time.Duration) error
	GetResult(ctx context.Context, taskID string) (model.TaskResult, error)
	Delete(ctx context.Context, taskID string) error
	List(ctx context.Context, limit, offset int) ([]model.Task, error)
	ListCompleted(ctx context.Context, limit, offset int) ([]model.VideoSummary, error)
}

// PipelineRunner executes the full analysis pipeline for a given task.
type PipelineRunner interface {
	Run(ctx context.Context, task model.Task) (model.TaskResult, error)
}
