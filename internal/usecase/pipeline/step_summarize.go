package pipeline

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/um4ru-ch4n/youtube-analyzer-mcp/internal/model"
	"github.com/um4ru-ch4n/youtube-analyzer-mcp/pkg/logger"
)

func (r *Runner) summarize(ctx context.Context, state *State) error {
	logger.InfoKV(ctx, "summarizing chunks", "task_id", state.TaskID, "chunk_count", len(state.Chunks))

	summaries := make([]model.ChunkSummary, 0, len(state.Chunks))

	for _, ch := range state.Chunks {
		var sb strings.Builder
		for _, seg := range ch.Segments {
			sb.WriteString(seg.Text)
			sb.WriteString(" ")
		}

		summary, err := r.summarizer.SummarizeChunk(ctx, sb.String(), ch.Frames)
		if err != nil {
			logger.WarnKV(ctx, "chunk summarization failed, using empty summary",
				"task_id", state.TaskID,
				"chunk_index", ch.Index,
				"error", err.Error(),
			)

			state.Warnings = append(state.Warnings, model.Warning{
				Step:      "summarize",
				Message:   fmt.Sprintf("chunk %d summarization failed: %v", ch.Index, err),
				Timestamp: time.Now().UTC(),
			})

			summary = ""
		}

		summaries = append(summaries, model.ChunkSummary{
			Index:     ch.Index,
			TimeStart: ch.TimeStart,
			TimeEnd:   ch.TimeEnd,
			Summary:   summary,
			Frames:    ch.Frames,
		})
	}

	state.Summaries = summaries

	logger.InfoKV(ctx, "summarization complete", "task_id", state.TaskID, "summaries", len(summaries))

	return nil
}
