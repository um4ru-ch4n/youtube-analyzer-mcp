package mcp

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"

	"github.com/um4ru-ch4n/youtube-analyzer-mcp/internal/model"
	"github.com/um4ru-ch4n/youtube-analyzer-mcp/pkg/logger"
)

type resultResponse struct {
	TaskID          string                `json:"task_id"`
	VideoURL        string                `json:"video_url"`
	VideoTitle      string                `json:"video_title"`
	DurationSeconds float64               `json:"duration_seconds"`
	TotalChunks     int                   `json:"total_chunks"`
	TotalArtifacts  int                   `json:"total_artifacts"`
	Chunks          []resultChunkResponse `json:"chunks"`
}

type resultChunkResponse struct {
	Index          int                      `json:"index"`
	TimeStart      float64                  `json:"time_start"`
	TimeEnd        float64                  `json:"time_end"`
	Summary        string                   `json:"summary"`
	ArtifactCount  int                      `json:"artifact_count"`
	Artifacts      []resultArtifactResponse `json:"artifacts,omitempty"`
}

type resultArtifactResponse struct {
	TimestampSec float64 `json:"timestamp_sec"`
	FrameType    string  `json:"frame_type"`
	OCRText      string  `json:"ocr_text,omitempty"`
	HasImage     bool    `json:"has_image,omitempty"`
}

func (h *Handler) registerGetResult(s *server.MCPServer) {
	s.AddTool(
		mcp.NewTool("get_result",
			mcp.WithDescription(
				"Get the text analysis result for a completed video task. "+
					"Returns JSON with video metadata, time-based chunks with text summaries, "+
					"and artifact metadata (timestamps, types, OCR text). "+
					"Images are NOT included — use get_artifacts with a chunk index to fetch "+
					"visual artifacts for specific chunks that need deeper analysis.",
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

	formatted, err := json.MarshalIndent(resp, "", "  ")
	if err != nil {
		return toErrorResult(fmt.Errorf("marshal response: %w", err)), nil
	}

	return mcp.NewToolResultText(string(formatted)), nil
}

func buildResultResponse(taskID string, result model.TaskResult) resultResponse {
	totalArtifacts := 0
	chunks := make([]resultChunkResponse, 0, len(result.Chunks))

	for _, c := range result.Chunks {
		artifacts := make([]resultArtifactResponse, 0, len(c.Artifacts))
		for _, a := range c.Artifacts {
			artifacts = append(artifacts, resultArtifactResponse{
				TimestampSec: a.TimestampSec,
				FrameType:    string(a.Type),
				OCRText:      a.OCRText,
				HasImage:     a.ImagePath != "",
			})
		}

		totalArtifacts += len(artifacts)

		chunks = append(chunks, resultChunkResponse{
			Index:         c.Index,
			TimeStart:     c.TimeStart,
			TimeEnd:       c.TimeEnd,
			Summary:       c.Summary,
			ArtifactCount: len(artifacts),
			Artifacts:     artifacts,
		})
	}

	return resultResponse{
		TaskID:          taskID,
		VideoURL:        result.VideoMeta.URL,
		VideoTitle:      result.VideoMeta.Title,
		DurationSeconds: result.DurationSeconds,
		TotalChunks:     len(chunks),
		TotalArtifacts:  totalArtifacts,
		Chunks:          chunks,
	}
}
