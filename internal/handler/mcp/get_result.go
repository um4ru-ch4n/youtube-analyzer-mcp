package mcp

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"

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
	Chunks          []resultChunkResponse `json:"chunks"`
}

type resultChunkResponse struct {
	Index     int                       `json:"index"`
	TimeStart float64                   `json:"time_start"`
	TimeEnd   float64                   `json:"time_end"`
	Summary   string                    `json:"summary"`
	Artifacts []resultArtifactResponse  `json:"artifacts,omitempty"`
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
				"Get the full analysis result for a completed video task. "+
					"Only works when task status is 'completed' (check with get_status first). "+
					"Returns structured data: video metadata, time-based chunks with text summaries, "+
					"and visual artifacts (images + optional OCR text from slides/code). "+
					"Artifact images are returned as embedded base64 for direct viewing.",
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

	return buildMCPResult(taskID, result), nil
}

// buildMCPResult creates an MCP response with text JSON + embedded images for artifacts.
func buildMCPResult(taskID string, result model.TaskResult) *mcp.CallToolResult {
	resp := buildResultResponse(taskID, result)

	formatted, err := json.MarshalIndent(resp, "", "  ")
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("Error: failed to marshal response: %v", err))
	}

	// Start with text content.
	contents := []mcp.Content{
		mcp.NewTextContent(string(formatted)),
	}

	// Attach artifact images as embedded base64.
	for _, chunk := range result.Chunks {
		for _, artifact := range chunk.Artifacts {
			if artifact.ImagePath == "" {
				continue
			}

			imgData, readErr := os.ReadFile(artifact.ImagePath)
			if readErr != nil {
				logger.Logger().Warnw("failed to read artifact image",
					"path", artifact.ImagePath,
					"error", readErr.Error(),
				)
				continue
			}

			mimeType := detectMimeType(artifact.ImagePath)
			b64 := base64.StdEncoding.EncodeToString(imgData)

			label := fmt.Sprintf("[%s artifact at %.1fs", artifact.Type, artifact.TimestampSec)
			if artifact.OCRText != "" {
				label += " — OCR text included in JSON"
			}
			label += "]"

			contents = append(contents,
				mcp.NewTextContent(label),
				mcp.NewImageContent(b64, mimeType),
			)
		}
	}

	return &mcp.CallToolResult{
		Content: contents,
	}
}

func buildResultResponse(taskID string, result model.TaskResult) resultResponse {
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

		chunks = append(chunks, resultChunkResponse{
			Index:     c.Index,
			TimeStart: c.TimeStart,
			TimeEnd:   c.TimeEnd,
			Summary:   c.Summary,
			Artifacts: artifacts,
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

func detectMimeType(path string) string {
	ext := strings.ToLower(filepath.Ext(path))
	if ext == ".png" {
		return "image/png"
	}
	return "image/jpeg"
}
