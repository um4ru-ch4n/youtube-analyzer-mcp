package mcp

import (
	"reflect"
	"testing"

	"github.com/mark3labs/mcp-go/mcp"

	"github.com/um4ru-ch4n/youtube-analyzer-mcp/internal/model"
)

func TestParseTaskOptions(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		args    map[string]any
		want    model.TaskOptions
		wantErr bool
	}{
		{
			name: "empty",
			args: map[string]any{},
			want: model.TaskOptions{},
		},
		{
			name: "all set",
			args: map[string]any{
				"skip_summary":          true,
				"language":              " ru ",
				"glossary":              "Uniswap, USDC,, AMM ",
				"initial_prompt":        "Урок.",
				"frame_dedup_threshold": 0.98,
			},
			want: model.TaskOptions{
				SkipSummary:         true,
				Language:            "ru",
				Glossary:            []string{"Uniswap", "USDC", "AMM"},
				InitialPrompt:       "Урок.",
				FrameDedupThreshold: 0.98,
			},
		},
		{
			name:    "skip_summary not bool",
			args:    map[string]any{"skip_summary": "yes"},
			wantErr: true,
		},
		{
			name:    "negative threshold",
			args:    map[string]any{"frame_dedup_threshold": -1.0},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			req := mcp.CallToolRequest{}
			req.Params.Arguments = tt.args

			got, err := parseTaskOptions(req)
			if (err != nil) != tt.wantErr {
				t.Fatalf("err = %v, wantErr %v", err, tt.wantErr)
			}
			if !tt.wantErr && !reflect.DeepEqual(got, tt.want) {
				t.Errorf("got %+v, want %+v", got, tt.want)
			}
		})
	}
}

func TestTranscriptText(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name   string
		result model.TaskResult
		want   string
	}{
		{
			name: "segments",
			result: model.TaskResult{Segments: []model.TranscriptSegment{
				{StartSec: 5.7, Text: " Привет. "},
				{StartSec: 3725, Text: "Пока."},
			}},
			want: "[00:00:05] Привет.\n[01:02:05] Пока.\n",
		},
		{
			name:   "plain fallback",
			result: model.TaskResult{FullTranscript: "старый текст"},
			want:   "старый текст",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			got := transcriptText(buildTranscriptResponse("id", tt.result))
			if got != tt.want {
				t.Errorf("got %q, want %q", got, tt.want)
			}
		})
	}
}
