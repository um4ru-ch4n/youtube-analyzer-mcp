package mcp

import (
	"context"
	"fmt"
	"strings"

	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"

	"github.com/um4ru-ch4n/youtube-analyzer-mcp/internal/model"
	"github.com/um4ru-ch4n/youtube-analyzer-mcp/pkg/logger"
)

type transcriptResponse struct {
	TaskID         string                    `json:"task_id"`
	Language       string                    `json:"language,omitempty"`
	Source         string                    `json:"source"`
	Segments       []model.TranscriptSegment `json:"segments,omitempty"`
	FullTranscript string                    `json:"full_transcript,omitempty"`
}

func (h *Handler) registerGetTranscript(s *server.MCPServer) {
	s.AddTool(
		mcp.NewTool("get_transcript",
			mcp.WithDescription(
				"Get the full transcript of a completed task. "+
					"Returns timestamped segments (source=segments); tasks completed before segments "+
					"were stored return only plain text (source=plain). "+
					"format=text returns one \"[HH:MM:SS] text\" line per segment.",
			),
			mcp.WithString("task_id",
				mcp.Required(),
				mcp.Description("Task ID returned by add_video / add_local_video"),
			),
			mcp.WithString("format",
				mcp.Description("json (default) or text"),
				mcp.Enum("json", "text"),
			),
		),
		h.handleGetTranscript,
	)
}

func (h *Handler) handleGetTranscript(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	taskID, err := requireString(req, "task_id")
	if err != nil {
		return toErrorResult(err), nil
	}

	format, _ := req.GetArguments()["format"].(string)

	logger.Logger().Infow("get_transcript: fetching", "task_id", taskID, "format", format)

	result, err := h.taskService.GetResult(ctx, taskID)
	if err != nil {
		return toErrorResult(err), nil
	}

	resp := buildTranscriptResponse(taskID, result)

	if format == "text" {
		return mcp.NewToolResultText(transcriptText(resp)), nil
	}

	return toJSONResult(resp), nil
}

func buildTranscriptResponse(taskID string, result model.TaskResult) transcriptResponse {
	resp := transcriptResponse{
		TaskID:   taskID,
		Language: result.Language,
	}

	if len(result.Segments) > 0 {
		resp.Source = "segments"
		resp.Segments = result.Segments
		return resp
	}

	resp.Source = "plain"
	resp.FullTranscript = result.FullTranscript
	return resp
}

func transcriptText(resp transcriptResponse) string {
	if resp.Source != "segments" {
		return resp.FullTranscript
	}

	var sb strings.Builder
	for _, seg := range resp.Segments {
		sb.WriteString(fmt.Sprintf("[%s] %s\n", formatTimestamp(seg.StartSec), strings.TrimSpace(seg.Text)))
	}
	return sb.String()
}

func formatTimestamp(sec float64) string {
	s := int(sec)
	return fmt.Sprintf("%02d:%02d:%02d", s/3600, s%3600/60, s%60)
}
