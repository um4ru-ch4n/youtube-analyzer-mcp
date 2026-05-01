package pipeline

import (
	"context"
	"fmt"

	"github.com/um4ru-ch4n/youtube-analyzer-mcp/pkg/logger"
)

func (r *Runner) transcribe(ctx context.Context, state *State) error {
	logger.InfoKV(ctx, "transcribing audio", "task_id", state.TaskID, "audio_path", state.DownloadResult.AudioPath)

	transcript, err := r.transcriber.Transcribe(ctx, state.DownloadResult.AudioPath)
	if err != nil {
		return fmt.Errorf("transcribe audio: %w", err)
	}

	state.Transcript = transcript

	logger.InfoKV(ctx, "transcription complete",
		"task_id", state.TaskID,
		"segments", len(transcript.Segments),
		"language", transcript.Language,
	)

	return nil
}
