package mcp

import (
	"context"
	"encoding/base64"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"

	"github.com/um4ru-ch4n/youtube-analyzer-mcp/pkg/logger"
)

func (h *Handler) registerGetArtifacts(s *server.MCPServer) {
	s.AddTool(
		mcp.NewTool("get_artifacts",
			mcp.WithDescription(
				"Get visual artifacts (images) for a specific chunk of a completed video task. "+
					"Use this after get_result to fetch images for chunks that need deeper visual analysis. "+
					"Returns embedded base64 images with labels showing timestamp and frame type. "+
					"Each artifact also includes OCR text if available.",
			),
			mcp.WithString("task_id",
				mcp.Required(),
				mcp.Description("Task ID returned by add_video"),
			),
			mcp.WithNumber("chunk_index",
				mcp.Required(),
				mcp.Description("Index of the chunk to get artifacts for (from get_result response)"),
			),
		),
		h.handleGetArtifacts,
	)
}

func (h *Handler) handleGetArtifacts(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	taskID, err := requireString(req, "task_id")
	if err != nil {
		return toErrorResult(err), nil
	}

	chunkIndexRaw, ok := req.GetArguments()["chunk_index"]
	if !ok {
		return toErrorResult(fmt.Errorf("missing required parameter: chunk_index")), nil
	}

	chunkIndex, ok := chunkIndexRaw.(float64)
	if !ok {
		return toErrorResult(fmt.Errorf("chunk_index must be a number")), nil
	}

	idx := int(chunkIndex)

	logger.Logger().Infow("get_artifacts: fetching", "task_id", taskID, "chunk_index", idx)

	result, err := h.taskService.GetResult(ctx, taskID)
	if err != nil {
		return toErrorResult(err), nil
	}

	if idx < 0 || idx >= len(result.Chunks) {
		return toErrorResult(fmt.Errorf("chunk_index %d out of range (0-%d)", idx, len(result.Chunks)-1)), nil
	}

	chunk := result.Chunks[idx]

	if len(chunk.Artifacts) == 0 {
		return mcp.NewToolResultText(fmt.Sprintf("No artifacts for chunk %d [%.0fs - %.0fs]", idx, chunk.TimeStart, chunk.TimeEnd)), nil
	}

	contents := []mcp.Content{
		mcp.NewTextContent(fmt.Sprintf("Artifacts for chunk %d [%.0fs - %.0fs] — %d images:", idx, chunk.TimeStart, chunk.TimeEnd, len(chunk.Artifacts))),
	}

	for _, artifact := range chunk.Artifacts {
		if artifact.ImagePath == "" {
			continue
		}

		imgData, readErr := os.ReadFile(artifact.ImagePath)
		if readErr != nil {
			logger.Logger().Warnw("failed to read artifact image", "path", artifact.ImagePath, "error", readErr.Error())
			continue
		}

		mimeType := detectMimeType(artifact.ImagePath)
		b64 := base64.StdEncoding.EncodeToString(imgData)

		label := fmt.Sprintf("[%s at %.0fs", artifact.Type, artifact.TimestampSec)
		if artifact.OCRText != "" {
			label += fmt.Sprintf(" — OCR: %s", truncate(artifact.OCRText, 100))
		}
		label += "]"

		contents = append(contents,
			mcp.NewTextContent(label),
			mcp.NewImageContent(b64, mimeType),
		)
	}

	return &mcp.CallToolResult{Content: contents}, nil
}

func detectMimeType(path string) string {
	ext := strings.ToLower(filepath.Ext(path))
	if ext == ".png" {
		return "image/png"
	}
	return "image/jpeg"
}

func truncate(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen] + "..."
}
