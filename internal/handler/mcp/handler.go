package mcp

import (
	"github.com/mark3labs/mcp-go/server"
)

// Handler is the MCP transport layer. It registers tool handlers on a provided
// MCP server and delegates all business logic to TaskService.
type Handler struct {
	taskService TaskService
}

// New creates a new MCP handler with the given task service dependency.
func New(taskService TaskService) *Handler {
	return &Handler{
		taskService: taskService,
	}
}

// RegisterTools registers all YouTube analyzer MCP tools on the provided server.
func (h *Handler) RegisterTools(s *server.MCPServer) {
	h.registerAddVideo(s)
	h.registerGetStatus(s)
	h.registerGetResult(s)
	h.registerGetArtifacts(s)
	h.registerDeleteTask(s)
	h.registerRetryTask(s)
}
