package mcp

import (
	"context"

	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"

	"github.com/um4ru-ch4n/youtube-analyzer-mcp/pkg/logger"
)

func (h *Handler) registerDeleteTask(s *server.MCPServer) {
	s.AddTool(
		mcp.NewTool("delete_task",
			mcp.WithDescription(
				"Delete a video analysis task and all its artifacts (frames, audio, results). "+
					"Only works for tasks in 'completed' or 'failed' status. "+
					"Use this to free disk space after retrieving results.",
			),
			mcp.WithString("task_id",
				mcp.Required(),
				mcp.Description("Task ID of a completed or failed task to delete"),
			),
		),
		h.handleDeleteTask,
	)
}

func (h *Handler) handleDeleteTask(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	taskID, err := requireString(req, "task_id")
	if err != nil {
		return toErrorResult(err), nil
	}

	logger.Logger().Infow("delete_task: deleting task", "task_id", taskID)

	err = h.taskService.Delete(ctx, taskID)
	if err != nil {
		logger.Logger().Errorw("delete_task: failed to delete task", "task_id", taskID, "error", err.Error())
		return toErrorResult(err), nil
	}

	logger.Logger().Infow("delete_task: task deleted", "task_id", taskID)

	return toJSONResult(map[string]bool{
		"success": true,
	}), nil
}
