package clip

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"

	"github.com/um4ru-ch4n/youtube-analyzer-mcp/internal/model"
)

type Classifier struct {
	httpClient *http.Client
	baseURL    string
}

func New(url string, timeoutSec int) *Classifier {
	return &Classifier{
		httpClient: &http.Client{
			Timeout: time.Duration(timeoutSec) * time.Second,
		},
		baseURL: url,
	}
}

type classifyRequest struct {
	ImagePath string `json:"image_path"`
}

type classifyBatchRequest struct {
	ImagePaths []string `json:"image_paths"`
}

type classifyResponse struct {
	Type       string  `json:"type"`
	Confidence float64 `json:"confidence"`
}

type classifyWithEmbeddingResponse struct {
	Type       string    `json:"type"`
	Confidence float64   `json:"confidence"`
	Embedding  []float32 `json:"embedding"`
}

func (c *Classifier) ClassifyWithEmbedding(ctx context.Context, imagePath string) (model.FrameClassification, error) {
	reqBody, err := json.Marshal(classifyRequest{ImagePath: imagePath})
	if err != nil {
		return model.FrameClassification{}, fmt.Errorf("marshal request: %w", err)
	}

	endpoint := c.baseURL + "/classify_with_embedding"
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, bytes.NewReader(reqBody))
	if err != nil {
		return model.FrameClassification{}, fmt.Errorf("create request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return model.FrameClassification{}, fmt.Errorf("clip request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return model.FrameClassification{}, fmt.Errorf("clip returned status %d: %s", resp.StatusCode, string(body))
	}

	var result classifyWithEmbeddingResponse
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return model.FrameClassification{}, fmt.Errorf("decode response: %w", err)
	}

	return model.FrameClassification{
		Type:       model.FrameType(result.Type),
		Confidence: result.Confidence,
		Embedding:  result.Embedding,
	}, nil
}

func (c *Classifier) Classify(ctx context.Context, imagePath string) (model.FrameClassification, error) {
	reqBody, err := json.Marshal(classifyRequest{ImagePath: imagePath})
	if err != nil {
		return model.FrameClassification{}, fmt.Errorf("marshal request: %w", err)
	}

	endpoint := c.baseURL + "/classify"
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, bytes.NewReader(reqBody))
	if err != nil {
		return model.FrameClassification{}, fmt.Errorf("create request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return model.FrameClassification{}, fmt.Errorf("clip request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return model.FrameClassification{}, fmt.Errorf("clip returned status %d: %s", resp.StatusCode, string(body))
	}

	var result classifyResponse
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return model.FrameClassification{}, fmt.Errorf("decode response: %w", err)
	}

	return model.FrameClassification{
		Type:       model.FrameType(result.Type),
		Confidence: result.Confidence,
	}, nil
}

func (c *Classifier) ClassifyBatch(ctx context.Context, paths []string) ([]model.FrameClassification, error) {
	reqBody, err := json.Marshal(classifyBatchRequest{ImagePaths: paths})
	if err != nil {
		return nil, fmt.Errorf("marshal request: %w", err)
	}

	endpoint := c.baseURL + "/classify_batch"
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, bytes.NewReader(reqBody))
	if err != nil {
		return nil, fmt.Errorf("create request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("clip batch request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("clip batch returned status %d: %s", resp.StatusCode, string(body))
	}

	var results []classifyResponse
	if err := json.NewDecoder(resp.Body).Decode(&results); err != nil {
		return nil, fmt.Errorf("decode response: %w", err)
	}

	classifications := make([]model.FrameClassification, 0, len(results))
	for _, r := range results {
		classifications = append(classifications, model.FrameClassification{
			Type:       model.FrameType(r.Type),
			Confidence: r.Confidence,
		})
	}

	return classifications, nil
}

// ReleaseVRAM tells the sidecar to drop its CUDA cache. Use after a batch of
// classification work to free memory for other GPU tenants (e.g. Ollama).
func (c *Classifier) ReleaseVRAM(ctx context.Context) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.baseURL+"/release_vram", nil)
	if err != nil {
		return fmt.Errorf("create request: %w", err)
	}
	resp, err := c.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("clip release_vram: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("clip release_vram returned %d", resp.StatusCode)
	}
	return nil
}
