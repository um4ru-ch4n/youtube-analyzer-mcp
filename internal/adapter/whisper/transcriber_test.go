package whisper

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/um4ru-ch4n/youtube-analyzer-mcp/internal/model"
)

func TestTranscribeSendsDecodingSettings(t *testing.T) {
	t.Parallel()

	var got map[string]any
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		err := json.NewDecoder(r.Body).Decode(&got)
		if err != nil {
			t.Errorf("decode request: %v", err)
		}
		_, _ = w.Write([]byte(`{"segments":[{"start":1,"end":2,"text":"привет"}],"language":"ru"}`))
	}))
	defer srv.Close()

	tr := New(srv.URL, 5, Decoding{VADFilter: true, VADMinSilenceMs: 1000, ConditionOnPreviousText: false})

	res, err := tr.Transcribe(context.Background(), "/data/a.wav", model.TranscribeOptions{Language: "ru", InitialPrompt: "Урок."})
	if err != nil {
		t.Fatalf("Transcribe: %v", err)
	}

	want := map[string]any{
		"audio_path":                 "/data/a.wav",
		"language":                   "ru",
		"initial_prompt":             "Урок.",
		"vad_filter":                 true,
		"vad_min_silence_ms":         float64(1000),
		"condition_on_previous_text": false,
	}
	for k, v := range want {
		if got[k] != v {
			t.Errorf("request[%q] = %v, want %v", k, got[k], v)
		}
	}
	if _, ok := got["hotwords"]; ok {
		t.Errorf("empty hotwords must be omitted, got %v", got["hotwords"])
	}
	if len(res.Segments) != 1 || res.Language != "ru" {
		t.Errorf("unexpected result %+v", res)
	}
}
