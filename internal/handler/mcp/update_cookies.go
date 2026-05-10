package mcp

import (
	"context"
	"fmt"
	"os"
	"strings"

	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"

	"github.com/um4ru-ch4n/youtube-analyzer-mcp/pkg/logger"
)

func (h *Handler) registerUpdateCookies(s *server.MCPServer) {
	s.AddTool(
		mcp.NewTool("update_cookies",
			mcp.WithDescription(
				"Update the YouTube cookies file used by yt-dlp. "+
					"Call this whenever a task fails with 'Sign in to confirm you're not a bot' "+
					"or 'cookies are no longer valid'. After updating, call retry_task(task_id) "+
					"to resume the failed task from its last checkpoint.\n\n"+
					"Required format: Netscape cookie file (the one Chrome extension Cookie-Editor "+
					"exports as 'Netscape'). Fields are separated by either single tabs OR runs of "+
					"4+ spaces (server will normalize to tabs). Each cookie line has 7 fields: "+
					"domain | flag | path | secure | expiration | name | value. "+
					"Lines starting with '#' or '#HttpOnly_' are kept verbatim.\n\n"+
					"Cookies must include at minimum the SID-family cookies for .youtube.com: "+
					"__Secure-3PSID, __Secure-1PSID, SAPISID, SSID, __Secure-1PAPISID, "+
					"__Secure-3PAPISID, LOGIN_INFO. Without these yt-dlp will still hit the bot wall.",
			),
			mcp.WithString("content",
				mcp.Required(),
				mcp.Description("Full text of the Netscape cookie file (as exported from Cookie-Editor)"),
			),
		),
		h.handleUpdateCookies,
	)
}

func (h *Handler) handleUpdateCookies(_ context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	content, err := requireString(req, "content")
	if err != nil {
		return toErrorResult(err), nil
	}

	path := os.Getenv("YTDLP_COOKIES")
	if path == "" {
		return toErrorResult(fmt.Errorf("YTDLP_COOKIES env var is not set; server is not configured to manage cookies")), nil
	}

	normalized, lines, err := normalizeCookies(content)
	if err != nil {
		return toErrorResult(fmt.Errorf("validate cookies: %w", err)), nil
	}

	if err := os.WriteFile(path, []byte(normalized), 0o600); err != nil {
		return toErrorResult(fmt.Errorf("write cookies file: %w", err)), nil
	}

	logger.Logger().Infow("cookies updated", "path", path, "lines", lines)

	return toJSONResult(map[string]any{
		"ok":           true,
		"path":         path,
		"cookie_lines": lines,
		"hint":         "Call retry_task(task_id) to resume a previously failed task.",
	}), nil
}

// normalizeCookies converts Cookie-Editor's space-separated output to the tab-
// separated Netscape format yt-dlp requires. It also runs minimal sanity checks
// so we don't silently write garbage.
//
// Returns the normalized blob and the number of actual cookie lines (excluding
// comments and blanks).
func normalizeCookies(raw string) (string, int, error) {
	if strings.TrimSpace(raw) == "" {
		return "", 0, fmt.Errorf("cookies content is empty")
	}

	// Cookie-Editor exports with 4 spaces; yt-dlp wants real tabs.
	// Replace any run of 2+ spaces with a single tab so we cover variants.
	normalizedLines := make([]string, 0, 32)
	cookieCount := 0

	for _, line := range strings.Split(raw, "\n") {
		trimmed := strings.TrimRight(line, "\r\n")

		// Keep comments and blanks verbatim, but normalize trailing whitespace.
		if trimmed == "" || strings.HasPrefix(trimmed, "#") {
			// Special case: #HttpOnly_<rest> is technically a "comment" in
			// Netscape spec but yt-dlp parses it as a cookie row, so we still
			// need to tab-separate its fields.
			if strings.HasPrefix(trimmed, "#HttpOnly_") {
				normalizedLines = append(normalizedLines, normalizeFieldSeparators(trimmed))
				cookieCount++
				continue
			}
			normalizedLines = append(normalizedLines, trimmed)
			continue
		}

		normalized := normalizeFieldSeparators(trimmed)
		// Sanity check: tab-separated Netscape row has exactly 7 fields.
		fields := strings.Split(normalized, "\t")
		if len(fields) != 7 {
			return "", 0, fmt.Errorf(
				"line has %d fields, expected 7 (domain/flag/path/secure/expiration/name/value): %q",
				len(fields), trimmed,
			)
		}
		normalizedLines = append(normalizedLines, normalized)
		cookieCount++
	}

	if cookieCount == 0 {
		return "", 0, fmt.Errorf("no cookie lines found in input")
	}

	// Ensure standard header — yt-dlp doesn't strictly require it, but useful
	// for diffability.
	if !strings.HasPrefix(normalizedLines[0], "# Netscape HTTP Cookie File") {
		normalizedLines = append([]string{"# Netscape HTTP Cookie File"}, normalizedLines...)
	}

	return strings.Join(normalizedLines, "\n") + "\n", cookieCount, nil
}

// normalizeFieldSeparators converts runs of 2+ spaces to single tabs, leaving
// other whitespace alone. Used to repair Cookie-Editor's 4-space-separated rows.
func normalizeFieldSeparators(line string) string {
	// Fast path: already has tabs and no runs of spaces between fields.
	if strings.Contains(line, "\t") && !strings.Contains(line, "  ") {
		return line
	}

	// Replace any run of >=2 spaces with a single tab. We intentionally do not
	// touch single spaces — they may legitimately appear inside cookie values.
	var b strings.Builder
	b.Grow(len(line))
	i := 0
	for i < len(line) {
		if line[i] == ' ' {
			j := i
			for j < len(line) && line[j] == ' ' {
				j++
			}
			if j-i >= 2 {
				b.WriteByte('\t')
			} else {
				b.WriteByte(' ')
			}
			i = j
			continue
		}
		b.WriteByte(line[i])
		i++
	}
	return b.String()
}
