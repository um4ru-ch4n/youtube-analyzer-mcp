package summarizer

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

type OllamaSummarizer struct {
	httpClient *http.Client
	baseURL    string
	model      string
}

func New(url, model string, timeoutSec int) *OllamaSummarizer {
	return &OllamaSummarizer{
		httpClient: &http.Client{
			Timeout: time.Duration(timeoutSec) * time.Second,
		},
		baseURL: url,
		model:   model,
	}
}

type ollamaRequest struct {
	Model  string `json:"model"`
	Prompt string `json:"prompt"`
	Stream bool   `json:"stream"`
}

type ollamaResponse struct {
	Response string `json:"response"`
}

func (s *OllamaSummarizer) SummarizeChunk(ctx context.Context, transcriptText string) (string, error) {
	prompt := buildPrompt(transcriptText)

	reqBody := ollamaRequest{
		Model:  s.model,
		Prompt: prompt,
		Stream: false,
	}

	body, err := json.Marshal(reqBody)
	if err != nil {
		return "", fmt.Errorf("marshal request: %w", err)
	}

	endpoint := s.baseURL + "/api/generate"
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, bytes.NewReader(body))
	if err != nil {
		return "", fmt.Errorf("create request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := s.httpClient.Do(req)
	if err != nil {
		return "", fmt.Errorf("ollama request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		respBody, _ := io.ReadAll(resp.Body)
		return "", fmt.Errorf("ollama returned status %d: %s", resp.StatusCode, string(respBody))
	}

	var result ollamaResponse
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return "", fmt.Errorf("decode response: %w", err)
	}

	return strings.TrimSpace(result.Response), nil
}

func buildPrompt(transcriptText string) string {
	var sb strings.Builder

	sb.WriteString("Summarize the following video segment transcript.\n\n")
	sb.WriteString("## Transcript\n")
	sb.WriteString(transcriptText)
	sb.WriteString("\n\n")
	sb.WriteString("Provide a concise summary of the spoken content.")

	return sb.String()
}
