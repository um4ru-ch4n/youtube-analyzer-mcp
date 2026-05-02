package mcp

import (
	"context"

	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"

	"github.com/um4ru-ch4n/youtube-analyzer-mcp/pkg/logger"
)

func (h *Handler) registerRetryTask(s *server.MCPServer) {
	s.AddTool(
		mcp.NewTool("retry_task",
			mcp.WithDescription(
				"Retry a failed video analysis task. The pipeline resumes from the last "+
					"completed step (checkpoint), skipping download/frames/OCR if already done. "+
					"Only works for tasks with status 'failed'.",
			),
			mcp.WithString("task_id",
				mcp.Required(),
				mcp.Description("Task ID of a failed task to retry"),
			),
		),
		h.handleRetryTask,
	)
}

func (h *Handler) handleRetryTask(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	taskID, err := requireString(req, "task_id")
	if err != nil {
		return toErrorResult(err), nil
	}

	logger.Logger().Infow("retry_task: retrying task", "task_id", taskID)

	err = h.taskService.Retry(ctx, taskID)
	if err != nil {
		logger.Logger().Errorw("retry_task: failed", "task_id", taskID, "error", err.Error())
		return toErrorResult(err), nil
	}

	logger.Logger().Infow("retry_task: task re-enqueued", "task_id", taskID)

	return toJSONResult(map[string]any{
		"task_id": taskID,
		"status":  "queued",
		"message": "Task re-enqueued, will resume from last checkpoint",
	}), nil
}
