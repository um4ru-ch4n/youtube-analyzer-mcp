package mcp

import (
	"context"

	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"

	"github.com/um4ru-ch4n/youtube-analyzer-mcp/pkg/logger"
)

// statusResponse is the JSON structure returned by the get_status tool.
type statusResponse struct {
	TaskID   string   `json:"task_id"`
	Status   string   `json:"status"`
	Progress string   `json:"progress"`
	Error    string   `json:"error,omitempty"`
	Warnings []string `json:"warnings,omitempty"`
}

func (h *Handler) registerGetStatus(s *server.MCPServer) {
	s.AddTool(
		mcp.NewTool("get_status",
			mcp.WithDescription(
				"Get the current status of a video analysis task. "+
					"Use this to poll task progress after calling add_video. "+
					"Possible statuses: queued, downloading, transcribing, extracting_frames, "+
					"processing_frames, chunking, summarizing, completed, failed. "+
					"When status is 'completed', use get_result to retrieve the analysis.",
			),
			mcp.WithString("task_id",
				mcp.Required(),
				mcp.Description("Task ID returned by add_video"),
			),
		),
		h.handleGetStatus,
	)
}

func (h *Handler) handleGetStatus(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	taskID, err := requireString(req, "task_id")
	if err != nil {
		return toErrorResult(err), nil
	}

	logger.Logger().Infow("get_status: fetching status", "task_id", taskID)

	task, err := h.taskService.GetStatus(ctx, taskID)
	if err != nil {
		logger.Logger().Errorw("get_status: failed to get status", "task_id", taskID, "error", err.Error())
		return toErrorResult(err), nil
	}

	resp := statusResponse{
		TaskID:   task.ID,
		Status:   string(task.Status),
		Progress: task.Progress,
		Error:    task.Error,
	}

	for _, w := range task.Warnings {
		resp.Warnings = append(resp.Warnings, w.Message)
	}

	return toJSONResult(resp), nil
}
