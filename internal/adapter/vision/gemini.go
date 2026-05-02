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
	"path/filepath"
	"strings"
	"time"
)

type GeminiAnalyzer struct {
	apiKey     string
	model      string
	httpClient *http.Client
}

func NewGemini(apiKey, model string, timeoutSec int) *GeminiAnalyzer {
	return &GeminiAnalyzer{
		apiKey: apiKey,
		model:  model,
		httpClient: &http.Client{
			Timeout: time.Duration(timeoutSec) * time.Second,
		},
	}
}

type geminiRequest struct {
	Contents []geminiContent `json:"contents"`
}

type geminiContent struct {
	Parts []geminiPart `json:"parts"`
}

type geminiPart struct {
	Text       string          `json:"text,omitempty"`
	InlineData *geminiInline   `json:"inline_data,omitempty"`
}

type geminiInline struct {
	MimeType string `json:"mime_type"`
	Data     string `json:"data"`
}

type geminiResponse struct {
	Candidates []geminiCandidate `json:"candidates"`
}

type geminiCandidate struct {
	Content geminiContent `json:"content"`
}

func (g *GeminiAnalyzer) AnalyzeImage(ctx context.Context, imagePath, prompt string) (string, error) {
	imageData, err := os.ReadFile(imagePath)
	if err != nil {
		return "", fmt.Errorf("read image: %w", err)
	}

	encoded := base64.StdEncoding.EncodeToString(imageData)
	mimeType := detectMimeType(imagePath)

	reqBody := geminiRequest{
		Contents: []geminiContent{
			{
				Parts: []geminiPart{
					{
						InlineData: &geminiInline{
							MimeType: mimeType,
							Data:     encoded,
						},
					},
					{
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

	url := fmt.Sprintf(
		"https://generativelanguage.googleapis.com/v1beta/models/%s:generateContent?key=%s",
		g.model, g.apiKey,
	)

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(body))
	if err != nil {
		return "", fmt.Errorf("create request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := g.httpClient.Do(req)
	if err != nil {
		return "", fmt.Errorf("gemini request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		respBody, _ := io.ReadAll(resp.Body)
		return "", fmt.Errorf("gemini returned status %d: %s", resp.StatusCode, string(respBody))
	}

	var result geminiResponse
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return "", fmt.Errorf("decode response: %w", err)
	}

	if len(result.Candidates) == 0 {
		return "", fmt.Errorf("gemini returned no candidates")
	}

	var textParts []string
	for _, part := range result.Candidates[0].Content.Parts {
		if part.Text != "" {
			textParts = append(textParts, part.Text)
		}
	}

	return strings.Join(textParts, "\n"), nil
}

func (g *GeminiAnalyzer) IsUsefulFrame(ctx context.Context, imagePath string) (bool, error) {
	prompt := "Is this video frame visually useful? It is useful if it contains a slide, code, diagram, chart, schema, or any informational visual content. It is NOT useful if it only shows a person talking (talking head) or is blank/blurry. Answer with exactly one word: yes or no."

	response, err := g.AnalyzeImage(ctx, imagePath, prompt)
	if err != nil {
		return false, fmt.Errorf("vision usefulness check: %w", err)
	}

	lower := strings.ToLower(strings.TrimSpace(response))
	return strings.HasPrefix(lower, "yes"), nil
}

func detectMimeType(path string) string {
	ext := strings.ToLower(filepath.Ext(path))
	switch ext {
	case ".png":
		return "image/png"
	case ".gif":
		return "image/gif"
	case ".webp":
		return "image/webp"
	default:
		return "image/jpeg"
	}
}
