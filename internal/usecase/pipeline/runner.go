package pipeline

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"

	"github.com/um4ru-ch4n/youtube-analyzer-mcp/internal/config"
	"github.com/um4ru-ch4n/youtube-analyzer-mcp/internal/model"
	"github.com/um4ru-ch4n/youtube-analyzer-mcp/pkg/logger"
)

// StatusUpdater persists task status changes during pipeline execution.
type StatusUpdater interface {
	UpdateStatus(ctx context.Context, taskID string, status model.TaskStatus, progress string) error
	UpdateWarnings(ctx context.Context, taskID string, warnings []model.Warning) error
}

// Runner orchestrates the full video analysis pipeline.
type Runner struct {
	downloader      Downloader
	transcriber     Transcriber
	frameExtractor  FrameExtractor
	frameClassifier FrameClassifier
	ocrReader       OCRReader
	visionAnalyzer  VisionAnalyzer
	summarizer      Summarizer
	deduplicator    Deduplicator
	statusUpdater   StatusUpdater
	cfg             config.PipelineConfig
	dataDir         string
}

// New creates a new pipeline Runner.
func New(
	downloader Downloader,
	transcriber Transcriber,
	frameExtractor FrameExtractor,
	frameClassifier FrameClassifier,
	ocrReader OCRReader,
	visionAnalyzer VisionAnalyzer,
	summarizer Summarizer,
	deduplicator Deduplicator,
	statusUpdater StatusUpdater,
	cfg config.PipelineConfig,
	dataDir string,
) *Runner {
	return &Runner{
		downloader:      downloader,
		transcriber:     transcriber,
		frameExtractor:  frameExtractor,
		frameClassifier: frameClassifier,
		ocrReader:       ocrReader,
		visionAnalyzer:  visionAnalyzer,
		summarizer:      summarizer,
		deduplicator:    deduplicator,
		statusUpdater:   statusUpdater,
		cfg:             cfg,
		dataDir:         dataDir,
	}
}

// Run executes the full analysis pipeline for the given task
// and returns a TaskResult. It satisfies the PipelineRunner interface.
// Supports resume from checkpoint — if a previous run saved state, it
// skips already completed steps.
func (r *Runner) Run(ctx context.Context, task model.Task) (model.TaskResult, error) {
	taskDir := filepath.Join(r.dataDir, "tasks", task.ID)
	if err := os.MkdirAll(taskDir, 0o755); err != nil {
		return model.TaskResult{}, fmt.Errorf("create task dir: %w", err)
	}

	// Try to resume from checkpoint.
	state, err := LoadCheckpoint(taskDir)
	if err != nil {
		logger.WarnKV(ctx, "failed to load checkpoint, starting fresh", "error", err.Error())
	}

	if state != nil {
		logger.InfoKV(ctx, "resuming from checkpoint", "task_id", task.ID, "last_step", state.LastStep)
	}

	if state == nil {
		state = &State{
			TaskID:   task.ID,
			VideoURL: task.VideoURL,
			TempDir:  taskDir,
		}
	}

	// Step 1: Download (skip if already done)
	if state.LastStep < "01_download" {
		if err := r.updateStatus(ctx, state, model.TaskStatusDownloading); err != nil {
			return model.TaskResult{}, err
		}

		if err := r.download(ctx, state); err != nil {
			return model.TaskResult{}, model.PipelineError{Step: "download", Cause: err}
		}

		state.LastStep = "01_download"
		r.saveCheckpoint(ctx, state)
	}

	// Step 2: Parallel fan-out — transcribe + extract frames (skip if already done)
	if state.LastStep < "02_transcribe_frames" {
		var (
			transcribeErr    error
			extractFramesErr error
			parallelWg       sync.WaitGroup
		)

		parallelWg.Add(2)

		go func() {
			defer parallelWg.Done()
			r.updateStatus(ctx, state, model.TaskStatusTranscribing)
			transcribeErr = r.transcribe(ctx, state)
		}()

		go func() {
			defer parallelWg.Done()
			r.updateStatus(ctx, state, model.TaskStatusExtractingFrames)
			extractFramesErr = r.extractFrames(ctx, state)
		}()

		parallelWg.Wait()

		if transcribeErr != nil {
			return model.TaskResult{}, model.PipelineError{Step: "transcribe", Cause: transcribeErr}
		}
		if extractFramesErr != nil {
			return model.TaskResult{}, model.PipelineError{Step: "extract_frames", Cause: extractFramesErr}
		}

		state.LastStep = "02_transcribe_frames"
		r.saveCheckpoint(ctx, state)
		r.saveProcessingArtifact(ctx, state, "transcript", state.Transcript)
		r.saveProcessingArtifact(ctx, state, "frames_deduped", state.Frames)
	}

	// Step 3: Process frames (classify + OCR/Vision)
	if state.LastStep < "03_process_frames" {
		if err := r.updateStatus(ctx, state, model.TaskStatusProcessingFrames); err != nil {
			return model.TaskResult{}, err
		}

		if err := r.processFrames(ctx, state); err != nil {
			return model.TaskResult{}, model.PipelineError{Step: "process_frames", Cause: err}
		}

		state.LastStep = "03_process_frames"
		r.saveCheckpoint(ctx, state)
		r.saveProcessingArtifact(ctx, state, "frames_classified", state.ProcessedFrames)
	}

	// Step 4: Chunk (merge transcript + frames by time)
	if state.LastStep < "04_chunk" {
		if err := r.updateStatus(ctx, state, model.TaskStatusChunking); err != nil {
			return model.TaskResult{}, err
		}

		if err := r.chunk(ctx, state); err != nil {
			return model.TaskResult{}, model.PipelineError{Step: "chunk", Cause: err}
		}

		state.LastStep = "04_chunk"
		r.saveCheckpoint(ctx, state)
		r.saveProcessingArtifact(ctx, state, "chunks", state.Chunks)
	}

	// Step 5: Summarize
	if state.LastStep < "05_summarize" {
		if err := r.updateStatus(ctx, state, model.TaskStatusSummarizing); err != nil {
			return model.TaskResult{}, err
		}

		if err := r.summarize(ctx, state); err != nil {
			return model.TaskResult{}, model.PipelineError{Step: "summarize", Cause: err}
		}

		state.LastStep = "05_summarize"
		r.saveCheckpoint(ctx, state)
	}

	r.saveProcessingArtifact(ctx, state, "summaries", state.Summaries)

	// Step 6: Build result
	result := r.buildResult(state)
	r.saveProcessingArtifact(ctx, state, "result", result)

	// Cleanup temp files (keep frames + result)
	r.cleanup(ctx, state)

	// Persist warnings if any
	if len(state.Warnings) > 0 {
		if err := r.statusUpdater.UpdateWarnings(ctx, state.TaskID, state.Warnings); err != nil {
			logger.WarnKV(ctx, "failed to persist warnings", "error", err.Error())
		}
	}

	return result, nil
}

func (r *Runner) saveCheckpoint(ctx context.Context, state *State) {
	if err := state.Save(); err != nil {
		logger.WarnKV(ctx, "failed to save checkpoint", "task_id", state.TaskID, "error", err.Error())
	}
}

// saveProcessingArtifact saves a named intermediate result as JSON to processing/ dir.
func (r *Runner) saveProcessingArtifact(ctx context.Context, state *State, name string, data any) {
	dir := filepath.Join(state.TempDir, "processing")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		logger.WarnKV(ctx, "failed to create processing dir", "error", err.Error())
		return
	}

	raw, err := json.MarshalIndent(data, "", "  ")
	if err != nil {
		logger.WarnKV(ctx, "failed to marshal artifact", "name", name, "error", err.Error())
		return
	}

	path := filepath.Join(dir, name+".json")
	if err := os.WriteFile(path, raw, 0o644); err != nil {
		logger.WarnKV(ctx, "failed to write artifact", "name", name, "error", err.Error())
	}
}

func (r *Runner) updateStatus(ctx context.Context, state *State, status model.TaskStatus) error {
	logger.InfoKV(ctx, "pipeline step", "task_id", state.TaskID, "status", string(status))
	return r.statusUpdater.UpdateStatus(ctx, state.TaskID, status, string(status))
}

func (r *Runner) buildResult(state *State) model.TaskResult {
	var sb strings.Builder
	for i, seg := range state.Transcript.Segments {
		if i > 0 {
			sb.WriteString(" ")
		}
		sb.WriteString(seg.Text)
	}

	return model.TaskResult{
		VideoMeta:       state.VideoMeta,
		Chunks:          state.Summaries,
		FullTranscript:  sb.String(),
		TotalFrames:     len(state.Frames),
		DurationSeconds: state.VideoMeta.DurationSeconds,
	}
}

func (r *Runner) cleanup(ctx context.Context, state *State) {
	// Remove video and audio files, keep frames directory and result
	videoPath := state.DownloadResult.VideoPath
	if videoPath != "" {
		if err := os.Remove(videoPath); err != nil {
			logger.DebugKV(ctx, "failed to remove video file", "path", videoPath, "error", err.Error())
		}
	}

	audioPath := state.DownloadResult.AudioPath
	if audioPath != "" {
		if err := os.Remove(audioPath); err != nil {
			logger.DebugKV(ctx, "failed to remove audio file", "path", audioPath, "error", err.Error())
		}
	}

	if state.TempDir != "" {
		processingDir := filepath.Join(state.TempDir, "processing")
		if err := os.RemoveAll(processingDir); err != nil {
			logger.WarnKV(ctx, "failed to remove processing dir", "path", processingDir, "error", err.Error())
		}

		checkpointPath := filepath.Join(state.TempDir, "checkpoint.json")
		if err := os.Remove(checkpointPath); err != nil && !os.IsNotExist(err) {
			logger.WarnKV(ctx, "failed to remove checkpoint", "path", checkpointPath, "error", err.Error())
		}
	}
}
