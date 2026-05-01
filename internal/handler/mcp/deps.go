package mcp

import (
	"context"

	"github.com/um4ru-ch4n/youtube-analyzer-mcp/internal/model"
)

// TaskService defines the business logic operations that the MCP handler layer depends on.
type TaskService interface {
	Submit(ctx context.Context, videoURL string) (taskID string, err error)
	GetStatus(ctx context.Context, taskID string) (model.Task, error)
	GetResult(ctx context.Context, taskID string) (model.TaskResult, error)
	Delete(ctx context.Context, taskID string) error
}
