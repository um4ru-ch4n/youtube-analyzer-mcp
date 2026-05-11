package mcp

import (
	"context"
	"time"

	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"

	"github.com/um4ru-ch4n/youtube-analyzer-mcp/pkg/logger"
)

type videoSummaryResponse struct {
	TaskID            string  `json:"task_id"`
	VideoURL          string  `json:"video_url"`
	VideoTitle        string  `json:"video_title"`
	DurationSeconds   float64 `json:"duration_seconds"`
	ChunkCount        int     `json:"chunk_count"`
	ProcessingSeconds float64 `json:"processing_seconds"`
	AnalyzedAt        string  `json:"analyzed_at"`
}

type listVideosResponse struct {
	Total  int                    `json:"total"`
	Videos []videoSummaryResponse `json:"videos"`
}

func (h *Handler) registerListVideos(s *server.MCPServer) {
	s.AddTool(
		mcp.NewTool("list_videos",
			mcp.WithDescription(
				"List all previously analyzed videos. Call this at the start of a new session "+
					"to discover videos that have already been processed — no need to re-analyze them. "+
					"Returns task_id, video title, duration, chunk count, and processing time for each video. "+
					"Use get_result with a task_id to retrieve the full analysis and summaries.",
			),
			mcp.WithNumber("limit",
				mcp.Description("Maximum number of results to return (default: 50, max: 200)"),
			),
			mcp.WithNumber("offset",
				mcp.Description("Number of results to skip for pagination (default: 0)"),
			),
		),
		h.handleListVideos,
	)
}

func (h *Handler) handleListVideos(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	limit := 50
	offset := 0

	if v, ok := req.GetArguments()["limit"]; ok {
		if n, ok := v.(float64); ok && n > 0 {
			limit = int(n)
			if limit > 200 {
				limit = 200
			}
		}
	}

	if v, ok := req.GetArguments()["offset"]; ok {
		if n, ok := v.(float64); ok && n >= 0 {
			offset = int(n)
		}
	}

	logger.Logger().Infow("list_videos called", "limit", limit, "offset", offset)

	summaries, err := h.taskService.ListVideos(ctx, limit, offset)
	if err != nil {
		logger.Logger().Errorw("list_videos failed", "error", err.Error())
		return toErrorResult(err), nil
	}

	videos := make([]videoSummaryResponse, 0, len(summaries))
	for _, s := range summaries {
		title := s.VideoTitle
		if title == "" {
			title = "(untitled)"
		}
		videos = append(videos, videoSummaryResponse{
			TaskID:            s.TaskID,
			VideoURL:          s.VideoURL,
			VideoTitle:        title,
			DurationSeconds:   s.DurationSeconds,
			ChunkCount:        s.ChunkCount,
			ProcessingSeconds: s.ProcessingSeconds,
			AnalyzedAt:        s.AnalyzedAt.UTC().Format(time.RFC3339),
		})
	}

	return toJSONResult(listVideosResponse{
		Total:  len(videos),
		Videos: videos,
	}), nil
}
