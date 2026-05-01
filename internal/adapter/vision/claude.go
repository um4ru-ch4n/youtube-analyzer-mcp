package vision

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"
	"time"
)

type ClaudeAnalyzer struct {
	apiKey     string
	model      string
	httpClient *http.Client
}

func NewClaude(apiKey, model string, timeoutSec int) *ClaudeAnalyzer {
	return &ClaudeAnalyzer{
		apiKey: apiKey,
		model:  model,
		httpClient: &http.Client{
			Timeout: time.Duration(timeoutSec) * time.Second,
		},
	}
}

type claudeRequest struct {
	Model     string           `json:"model"`
	MaxTokens int              `json:"max_tokens"`
	Messages  []claudeMessage  `json:"messages"`
}

type claudeMessage struct {
	Role    string               `json:"role"`
	Content []claudeContentBlock `json:"content"`
}

type claudeContentBlock struct {
	Type   string       `json:"type"`
	Text   string       `json:"text,omitempty"`
	Source *claudeSource `json:"source,omitempty"`
}

type claudeSource struct {
	Type      string `json:"type"`
	MediaType string `json:"media_type"`
	Data      string `json:"data"`
}

type claudeResponse struct {
	Content []claudeResponseContent `json:"content"`
}

type claudeResponseContent struct {
	Type string `json:"type"`
	Text string `json:"text"`
}

func (c *ClaudeAnalyzer) AnalyzeImage(ctx context.Context, imagePath, prompt string) (string, error) {
	imageData, err := os.ReadFile(imagePath)
	if err != nil {
		return "", fmt.Errorf("read image: %w", err)
	}

	encoded := base64.StdEncoding.EncodeToString(imageData)
	mimeType := detectMimeType(imagePath)

	reqBody := claudeRequest{
		Model:     c.model,
		MaxTokens: 4096,
		Messages: []claudeMessage{
			{
				Role: "user",
				Content: []claudeContentBlock{
					{
						Type: "image",
						Source: &claudeSource{
							Type:      "base64",
							MediaType: mimeType,
							Data:      encoded,
						},
					},
					{
						Type: "text",
						Text: prompt,
					},
				},
			},
		},
	}

	body, err := json.Marshal(reqBody)
	if err != nil {
		return "", fmt.Errorf("marshal request: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, "https://api.anthropic.com/v1/messages", bytes.NewReader(body))
	if err != nil {
		return "", fmt.Errorf("create request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("x-api-key", c.apiKey)
	req.Header.Set("anthropic-version", "2023-06-01")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return "", fmt.Errorf("claude request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		respBody, _ := io.ReadAll(resp.Body)
		return "", fmt.Errorf("claude returned status %d: %s", resp.StatusCode, string(respBody))
	}

	var result claudeResponse
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return "", fmt.Errorf("decode response: %w", err)
	}

	var textParts []string
	for _, block := range result.Content {
		if block.Type == "text" {
			textParts = append(textParts, block.Text)
		}
	}

	return strings.Join(textParts, "\n"), nil
}
