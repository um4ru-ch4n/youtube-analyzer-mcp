package ocr

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"

	"go.uber.org/zap"
)

// Reader is an HTTP client to the OCR sidecar (EasyOCR-based, GPU-capable).
type Reader struct {
	httpClient *http.Client
	baseURL    string
	logger     *zap.Logger
}

func New(url string, timeoutSec int, logger *zap.Logger) *Reader {
	if timeoutSec <= 0 {
		timeoutSec = 120
	}
	return &Reader{
		httpClient: &http.Client{Timeout: time.Duration(timeoutSec) * time.Second},
		baseURL:    url,
		logger:     logger,
	}
}

type readRequest struct {
	ImagePath string `json:"image_path"`
}

type readResponse struct {
	Text string `json:"text"`
}

func (r *Reader) ReadText(ctx context.Context, imagePath string) (string, error) {
	body, err := json.Marshal(readRequest{ImagePath: imagePath})
	if err != nil {
		return "", fmt.Errorf("marshal request: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, r.baseURL+"/read", bytes.NewReader(body))
	if err != nil {
		return "", fmt.Errorf("create request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")

	r.logger.Info("running OCR", zap.String("image", imagePath))

	resp, err := r.httpClient.Do(req)
	if err != nil {
		return "", fmt.Errorf("ocr request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		raw, _ := io.ReadAll(resp.Body)
		return "", fmt.Errorf("ocr returned status %d: %s", resp.StatusCode, string(raw))
	}

	var result readResponse
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return "", fmt.Errorf("decode ocr response: %w", err)
	}

	return result.Text, nil
}
