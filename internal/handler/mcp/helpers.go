package mcp

import (
	"encoding/json"
	"fmt"

	"github.com/mark3labs/mcp-go/mcp"
)

// requireString extracts a required string parameter from the MCP request.
// Returns an error if the parameter is missing or not a string.
func requireString(req mcp.CallToolRequest, key string) (string, error) {
	val, ok := req.GetArguments()[key]
	if !ok {
		return "", fmt.Errorf("missing required parameter: %s", key)
	}

	s, ok := val.(string)
	if !ok {
		return "", fmt.Errorf("parameter %s must be a string", key)
	}

	if s == "" {
		return "", fmt.Errorf("parameter %s must not be empty", key)
	}

	return s, nil
}

// toJSONResult marshals data into a successful MCP CallToolResult with
// pretty-printed JSON text content.
func toJSONResult(data any) *mcp.CallToolResult {
	formatted, err := json.MarshalIndent(data, "", "  ")
	if err != nil {
		return toErrorResult(fmt.Errorf("failed to marshal response: %w", err))
	}

	return mcp.NewToolResultText(string(formatted))
}

// toErrorResult creates an error MCP CallToolResult from the given error.
func toErrorResult(err error) *mcp.CallToolResult {
	return mcp.NewToolResultError(fmt.Sprintf("Error: %s", err.Error()))
}
