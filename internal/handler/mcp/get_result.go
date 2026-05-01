package mcp

import (
	"context"

	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"

	"github.com/um4ru-ch4n/youtube-analyzer-mcp/internal/model"
	"github.com/um4ru-ch4n/youtube-analyzer-mcp/pkg/logger"
)

// resultResponse is the JSON structure returned by the get_result tool,
// matching the SPEC.md section 3 output format.
type resultResponse struct {
	TaskID          string               `json:"task_id"`
	VideoURL        string               `json:"video_url"`
	VideoTitle      string               `json:"video_title"`
	DurationSeconds float64              `json:"duration_seconds"`
	Chunks          []resultChunkResponse `json:"chunks"`
}

// resultChunkResponse represents a single chunk in the get_result output.
type resultChunkResponse struct {
	Index     int                  `json:"index"`
	TimeStart float64             `json:"time_start"`
	TimeEnd   float64             `json:"time_end"`
	Summary   string              `json:"summary"`
	Frames    []resultFrameResponse `json:"frames"`
}

// resultFrameResponse represents a single frame in the get_result output.
type resultFrameResponse struct {
	TimestampSec float64 `json:"timestamp_sec"`
	FrameType    string  `json:"frame_type"`
	Content      string  `json:"content"`
}

func (h *Handler) registerGetResult(s *server.MCPServer) {
	s.AddTool(
		mcp.NewTool("get_result",
			mcp.WithDescription(
				"Get the full analysis result for a completed video task. "+
					"Only works when task status is 'completed' (check with get_status first). "+
					"Returns structured data: video metadata, time-based chunks with summaries, "+
					"and extracted visual content (OCR text from slides/code, descriptions of diagrams).",
			),
			mcp.WithString("task_id",
				mcp.Required(),
				mcp.Description("Task ID returned by add_video"),
			),
		),
		h.handleGetResult,
	)
}

func (h *Handler) handleGetResult(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	taskID, err := requireString(req, "task_id")
	if err != nil {
		return toErrorResult(err), nil
	}

	logger.Logger().Infow("get_result: fetching result", "task_id", taskID)

	result, err := h.taskService.GetResult(ctx, taskID)
	if err != nil {
		logger.Logger().Errorw("get_result: failed to get result", "task_id", taskID, "error", err.Error())
		return toErrorResult(err), nil
	}

	resp := buildResultResponse(taskID, result)

	return toJSONResult(resp), nil
}

func buildResultResponse(taskID string, result model.TaskResult) resultResponse {
	chunks := make([]resultChunkResponse, 0, len(result.Chunks))
	for _, c := range result.Chunks {
		frames := make([]resultFrameResponse, 0, len(c.Frames))
		for _, f := range c.Frames {
			frames = append(frames, resultFrameResponse{
				TimestampSec: f.TimestampSec,
				FrameType:    string(f.Type),
				Content:      f.Content,
			})
		}

		chunks = append(chunks, resultChunkResponse{
			Index:     c.Index,
			TimeStart: c.TimeStart,
			TimeEnd:   c.TimeEnd,
			Summary:   c.Summary,
			Frames:    frames,
		})
	}

	return resultResponse{
		TaskID:          taskID,
		VideoURL:        result.VideoMeta.URL,
		VideoTitle:      result.VideoMeta.Title,
		DurationSeconds: result.DurationSeconds,
		Chunks:          chunks,
	}
}
