package whisper

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

type Transcriber struct {
	httpClient *http.Client
	baseURL    string
}

func New(url string, timeoutSec int) *Transcriber {
	return &Transcriber{
		httpClient: &http.Client{
			Timeout: time.Duration(timeoutSec) * time.Second,
		},
		baseURL: url,
	}
}

type transcribeRequest struct {
	AudioPath string `json:"audio_path"`
}

type transcribeResponse struct {
	Segments []segmentResponse `json:"segments"`
	Language string            `json:"language"`
}

type segmentResponse struct {
	Start float64 `json:"start"`
	End   float64 `json:"end"`
	Text  string  `json:"text"`
}

func (t *Transcriber) Transcribe(ctx context.Context, audioPath string) (model.Transcript, error) {
	reqBody, err := json.Marshal(transcribeRequest{AudioPath: audioPath})
	if err != nil {
		return model.Transcript{}, fmt.Errorf("marshal request: %w", err)
	}

	endpoint := t.baseURL + "/transcribe"
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, bytes.NewReader(reqBody))
	if err != nil {
		return model.Transcript{}, fmt.Errorf("create request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := t.httpClient.Do(req)
	if err != nil {
		return model.Transcript{}, fmt.Errorf("whisper request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return model.Transcript{}, fmt.Errorf("whisper returned status %d: %s", resp.StatusCode, string(body))
	}

	var result transcribeResponse
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return model.Transcript{}, fmt.Errorf("decode response: %w", err)
	}

	segments := make([]model.TranscriptSegment, 0, len(result.Segments))
	for _, seg := range result.Segments {
		segments = append(segments, model.TranscriptSegment{
			StartSec: seg.Start,
			EndSec:   seg.End,
			Text:     seg.Text,
		})
	}

	return model.Transcript{
		Segments: segments,
		Language: result.Language,
	}, nil
}
