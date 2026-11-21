package mcp

import (
	"fmt"
	"strings"

	"github.com/mark3labs/mcp-go/mcp"

	"github.com/um4ru-ch4n/youtube-analyzer-mcp/internal/model"
)

// withTaskOptions appends the optional per-task pipeline parameters shared by add_video and add_local_video.
func withTaskOptions(base ...mcp.ToolOption) []mcp.ToolOption {
	return append(base,
		mcp.WithBoolean("skip_summary",
			mcp.Description("Skip LLM summarization (the slowest step). Result keeps transcript, chunks and frames; summaries are empty."),
		),
		mcp.WithString("language",
			mcp.Description("Force Whisper language, ISO code (e.g. ru, en). Default: auto-detect."),
		),
		mcp.WithString("glossary",
			mcp.Description("Comma-separated domain terms passed to Whisper as hotwords on every window. "+
				"Use with care: on long Russian lectures hotwords caused repetition loops and invented text; prefer initial_prompt."),
		),
		mcp.WithString("initial_prompt",
			mcp.Description("Whisper initial prompt: a short punctuated sample in the target language; helps punctuation and terms at the start."),
		),
		mcp.WithNumber("frame_dedup_threshold",
			mcp.Description("CLIP embedding similarity above which a frame is dropped as a duplicate (server default 0.95). Higher keeps more frames; 1 disables embedding dedup."),
		),
	)
}

func parseTaskOptions(req mcp.CallToolRequest) (model.TaskOptions, error) {
	args := req.GetArguments()

	var opts model.TaskOptions

	if v, ok := args["skip_summary"]; ok {
		b, isBool := v.(bool)
		if !isBool {
			return opts, fmt.Errorf("skip_summary must be a boolean")
		}
		opts.SkipSummary = b
	}

	if v, ok := args["language"].(string); ok {
		opts.Language = strings.TrimSpace(v)
	}

	if v, ok := args["glossary"].(string); ok {
		for _, term := range strings.Split(v, ",") {
			term = strings.TrimSpace(term)
			if term != "" {
				opts.Glossary = append(opts.Glossary, term)
			}
		}
	}

	if v, ok := args["initial_prompt"].(string); ok {
		opts.InitialPrompt = strings.TrimSpace(v)
	}

	if v, ok := args["frame_dedup_threshold"]; ok {
		f, isNum := v.(float64)
		if !isNum || f <= 0 {
			return opts, fmt.Errorf("frame_dedup_threshold must be a positive number")
		}
		opts.FrameDedupThreshold = f
	}

	return opts, nil
}
