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

func (s *OllamaSummarizer) SummarizeChunk(ctx context.Context, videoTitle string, transcriptText string, compressionRatio int) (string, error) {
	prompt := buildPrompt(videoTitle, transcriptText, compressionRatio)

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

func buildPrompt(videoTitle string, transcriptText string, compressionRatio int) string {
	wordCount := len(strings.Fields(transcriptText))
	targetWords := wordCount / compressionRatio
	if targetWords < 20 {
		targetWords = 20
	}

	var sb strings.Builder

	sb.WriteString(fmt.Sprintf("You are condensing a segment from the video \"%s\".\n", videoTitle))
	sb.WriteString("Your task is to compress the transcript while preserving the speaker's voice, tone, and style.\n\n")
	sb.WriteString("Rules:\n")
	sb.WriteString("- Write as if YOU are the speaker — same tone, same style, same energy\n")
	sb.WriteString("- This is NOT a third-person summary. Do NOT write 'The speaker discusses...' or 'The video explains...'\n")
	sb.WriteString("- Keep specific numbers, names, prices, and technical terms exactly as stated\n")
	sb.WriteString("- Preserve the logical flow — this should read like a shorter version of the same speech\n")
	sb.WriteString("- Remove filler words, repetitions, and tangents, but keep the substance\n")
	sb.WriteString(fmt.Sprintf("- Target length: approximately %d words (compress %dx from original)\n", targetWords, compressionRatio))
	sb.WriteString("- Output ONLY the compressed text, no headers or explanations\n")
	sb.WriteString("\n## Original transcript:\n")
	sb.WriteString(transcriptText)

	return sb.String()
}
