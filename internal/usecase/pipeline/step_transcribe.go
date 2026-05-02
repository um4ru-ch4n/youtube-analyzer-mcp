package pipeline

import (
	"context"
	"fmt"

	"github.com/um4ru-ch4n/youtube-analyzer-mcp/internal/adapter/subtitle"
	"github.com/um4ru-ch4n/youtube-analyzer-mcp/pkg/logger"
)

func (r *Runner) transcribe(ctx context.Context, state *State) error {
	// Fast path: use downloaded subtitles if available.
	if state.DownloadResult.SubtitlesPath != "" {
		logger.InfoKV(ctx, "parsing subtitles from file (skipping Whisper)",
			"task_id", state.TaskID,
			"path", state.DownloadResult.SubtitlesPath,
		)

		transcript, err := subtitle.ParseVTT(state.DownloadResult.SubtitlesPath)
		if err != nil {
			logger.WarnKV(ctx, "subtitle parsing failed, falling back to Whisper",
				"task_id", state.TaskID,
				"error", err.Error(),
			)
			return r.transcribeWithWhisper(ctx, state)
		}

		if len(transcript.Segments) == 0 {
			logger.WarnKV(ctx, "subtitles empty, falling back to Whisper",
				"task_id", state.TaskID,
			)
			return r.transcribeWithWhisper(ctx, state)
		}

		state.Transcript = transcript
		logger.InfoKV(ctx, "transcription from subtitles complete",
			"task_id", state.TaskID,
			"segments", len(transcript.Segments),
			"language", transcript.Language,
		)
		return nil
	}

	// Slow path: Whisper transcription.
	return r.transcribeWithWhisper(ctx, state)
}

func (r *Runner) transcribeWithWhisper(ctx context.Context, state *State) error {
	logger.InfoKV(ctx, "transcribing audio via Whisper",
		"task_id", state.TaskID,
		"audio_path", state.DownloadResult.AudioPath,
	)

	transcript, err := r.transcriber.Transcribe(ctx, state.DownloadResult.AudioPath)
	if err != nil {
		return fmt.Errorf("transcribe audio: %w", err)
	}

	state.Transcript = transcript

	logger.InfoKV(ctx, "Whisper transcription complete",
		"task_id", state.TaskID,
		"segments", len(transcript.Segments),
		"language", transcript.Language,
	)

	return nil
}
