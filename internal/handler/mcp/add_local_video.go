package mcp

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"

	"github.com/um4ru-ch4n/youtube-analyzer-mcp/pkg/logger"
)

const localUploadsPrefix = "/data/uploads/"

var allowedLocalExts = map[string]struct{}{
	".mp4":  {},
	".mkv":  {},
	".mov":  {},
	".webm": {},
}

func (h *Handler) registerAddLocalVideo(s *server.MCPServer) {
	s.AddTool(
		mcp.NewTool("add_local_video",
			mcp.WithDescription(
				"Add an already-uploaded local video file to the analysis queue. "+
					"The file must live inside the server's uploads directory "+
					"(path must start with /data/uploads/). Skips the download step "+
					"and runs the same transcription, frame extraction, and summarization "+
					"pipeline as add_video. Returns a task_id to track progress.",
			),
			mcp.WithString("path",
				mcp.Required(),
				mcp.Description("Absolute path to the uploaded file inside the container, e.g. /data/uploads/meeting.mp4"),
			),
			mcp.WithString("title",
				mcp.Description("Optional human-readable title; defaults to the filename without extension."),
			),
		),
		h.handleAddLocalVideo,
	)
}

func (h *Handler) handleAddLocalVideo(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	path, err := requireString(req, "path")
	if err != nil {
		return toErrorResult(err), nil
	}

	if err := validateLocalPath(path); err != nil {
		return toErrorResult(err), nil
	}

	title, _ := req.GetArguments()["title"].(string)

	logger.Logger().Infow("add_local_video: submitting", "path", path, "title", title)

	taskID, err := h.taskService.SubmitLocal(ctx, path, title)
	if err != nil {
		logger.Logger().Errorw("add_local_video: submit failed", "path", path, "error", err.Error())
		return toErrorResult(err), nil
	}

	logger.Logger().Infow("add_local_video: submitted", "path", path, "task_id", taskID)

	return toJSONResult(map[string]string{
		"task_id": taskID,
	}), nil
}

func validateLocalPath(path string) error {
	if !filepath.IsAbs(path) {
		return fmt.Errorf("path must be absolute")
	}
	if !strings.HasPrefix(path, localUploadsPrefix) {
		return fmt.Errorf("path must be under %s", localUploadsPrefix)
	}
	ext := strings.ToLower(filepath.Ext(path))
	if _, ok := allowedLocalExts[ext]; !ok {
		return fmt.Errorf("unsupported extension %q; allowed: .mp4 .mkv .mov .webm", ext)
	}
	info, err := os.Stat(path)
	if err != nil {
		return fmt.Errorf("stat: %w", err)
	}
	if !info.Mode().IsRegular() {
		return fmt.Errorf("not a regular file: %s", path)
	}
	return nil
}
