package pipeline

import (
	"context"

	"github.com/um4ru-ch4n/youtube-analyzer-mcp/internal/service/chunking"
	"github.com/um4ru-ch4n/youtube-analyzer-mcp/pkg/logger"
)

func (r *Runner) chunk(ctx context.Context, state *State) error {
	chunkDuration := r.cfg.ChunkDurationSec
	if chunkDuration <= 0 {
		chunkDuration = 45
	}

	state.Chunks = chunking.BuildChunks(state.Transcript.Segments, chunkDuration)

	logger.InfoKV(ctx, "chunking complete",
		"task_id", state.TaskID,
		"chunks", len(state.Chunks),
	)

	return nil
}
