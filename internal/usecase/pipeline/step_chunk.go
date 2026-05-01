package pipeline

import (
	"context"

	"github.com/um4ru-ch4n/youtube-analyzer-mcp/internal/model"
	"github.com/um4ru-ch4n/youtube-analyzer-mcp/internal/service/chunking"
	"github.com/um4ru-ch4n/youtube-analyzer-mcp/pkg/logger"
)

func (r *Runner) chunk(ctx context.Context, state *State) error {
	chunkDuration := r.cfg.ChunkDurationSec
	if chunkDuration <= 0 {
		chunkDuration = 45
	}

	// Filter out talking_head frames from chunk content — they carry no information
	informativeFrames := make([]model.FrameContent, 0, len(state.FrameContents))
	for _, fc := range state.FrameContents {
		if fc.Type == model.FrameTypeTalkingHead {
			continue
		}
		informativeFrames = append(informativeFrames, fc)
	}

	state.Chunks = chunking.BuildChunks(state.Transcript.Segments, informativeFrames, chunkDuration)

	logger.InfoKV(ctx, "chunking complete",
		"task_id", state.TaskID,
		"chunks", len(state.Chunks),
	)

	return nil
}
