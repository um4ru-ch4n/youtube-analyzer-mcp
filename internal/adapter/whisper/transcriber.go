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

// Decoding holds server-wide Whisper decoding defaults sent with every request.
type Decoding struct {
	VADFilter               bool
	VADMinSilenceMs         int
	ConditionOnPreviousText bool
}

type Transcriber struct {
	httpClient *http.Client
	baseURL    string
	decoding   Decoding
}

func New(url string, timeoutSec int, decoding Decoding) *Transcriber {
	return &Transcriber{
		httpClient: &http.Client{
			Timeout: time.Duration(timeoutSec) * time.Second,
		},
		baseURL:  url,
		decoding: decoding,
	}
}

type transcribeRequest struct {
	AudioPath     string `json:"audio_path"`
	Language      string `json:"language,omitempty"`
	InitialPrompt string `json:"initial_prompt,omitempty"`
	Hotwords      string `json:"hotwords,omitempty"`

	VADFilter               bool `json:"vad_filter"`
	VADMinSilenceMs         int  `json:"vad_min_silence_ms,omitempty"`
	ConditionOnPreviousText bool `json:"condition_on_previous_text"`
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

func (t *Transcriber) Transcribe(ctx context.Context, audioPath string, opts model.TranscribeOptions) (model.Transcript, error) {
	reqBody, err := json.Marshal(transcribeRequest{
		AudioPath:     audioPath,
		Language:      opts.Language,
		InitialPrompt: opts.InitialPrompt,
		Hotwords:      opts.Hotwords,

		VADFilter:               t.decoding.VADFilter,
		VADMinSilenceMs:         t.decoding.VADMinSilenceMs,
		ConditionOnPreviousText: t.decoding.ConditionOnPreviousText,
	})
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
