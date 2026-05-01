package mcp

import (
	"context"

	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"

	"github.com/um4ru-ch4n/youtube-analyzer-mcp/pkg/logger"
)

func (h *Handler) registerAddVideo(s *server.MCPServer) {
	s.AddTool(
		mcp.NewTool("add_video",
			mcp.WithDescription(
				"Add a YouTube video to the analysis queue. "+
					"The server will download the video, extract audio transcript and key frames, "+
					"then produce a structured summary with visual content analysis. "+
					"Returns a task_id to track progress via get_status and retrieve results via get_result.",
			),
			mcp.WithString("url",
				mcp.Required(),
				mcp.Description("YouTube video URL (e.g. https://youtube.com/watch?v=...)"),
			),
		),
		h.handleAddVideo,
	)
}

func (h *Handler) handleAddVideo(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	url, err := requireString(req, "url")
	if err != nil {
		return toErrorResult(err), nil
	}

	logger.Logger().Infow("add_video: submitting video", "url", url)

	taskID, err := h.taskService.Submit(ctx, url)
	if err != nil {
		logger.Logger().Errorw("add_video: failed to submit video", "url", url, "error", err.Error())
		return toErrorResult(err), nil
	}

	logger.Logger().Infow("add_video: video submitted", "url", url, "task_id", taskID)

	return toJSONResult(map[string]string{
		"task_id": taskID,
	}), nil
}
