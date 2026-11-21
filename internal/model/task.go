package model

import "time"

// TaskStatus represents the current state of a video analysis task.
type TaskStatus string

const (
	TaskStatusQueued           TaskStatus = "queued"
	TaskStatusDownloading      TaskStatus = "downloading"
	TaskStatusTranscribing     TaskStatus = "transcribing"
	TaskStatusExtractingFrames TaskStatus = "extracting_frames"
	TaskStatusProcessingFrames TaskStatus = "processing_frames"
	TaskStatusChunking         TaskStatus = "chunking"
	TaskStatusSummarizing      TaskStatus = "summarizing"
	TaskStatusCompleted        TaskStatus = "completed"
	TaskStatusFailed           TaskStatus = "failed"
)

// IsFinal returns true if the task is in a terminal state.
func (s TaskStatus) IsFinal() bool {
	return s == TaskStatusCompleted || s == TaskStatusFailed
}

// Task represents a video analysis task tracked by the system.
type Task struct {
	ID        string      `json:"id"`
	VideoURL  string      `json:"video_url"`
	Status    TaskStatus  `json:"status"`
	Progress  string      `json:"progress"`
	Error     string      `json:"error,omitempty"`
	Warnings  []Warning   `json:"warnings,omitempty"`
	Options   TaskOptions `json:"options"`
	CreatedAt time.Time   `json:"created_at"`
	UpdatedAt time.Time   `json:"updated_at"`
}

// TaskResult holds the final output of a completed video analysis pipeline.
type TaskResult struct {
	VideoMeta       VideoMeta           `json:"video_meta"`
	Chunks          []ChunkSummary      `json:"chunks"`
	FullTranscript  string              `json:"full_transcript"`
	Segments        []TranscriptSegment `json:"segments,omitempty"`
	Language        string              `json:"language,omitempty"`
	TotalFrames     int                 `json:"total_frames"`
	DurationSeconds float64             `json:"duration_seconds"`
}

// Warning captures a non-fatal issue encountered during pipeline processing.
type Warning struct {
	Step      string    `json:"step"`
	Message   string    `json:"message"`
	Timestamp time.Time `json:"timestamp"`
}

// VideoSummary is a lightweight view of a completed task for cross-session discovery.
type VideoSummary struct {
	TaskID            string    `json:"task_id"`
	VideoURL          string    `json:"video_url"`
	VideoTitle        string    `json:"video_title"`
	DurationSeconds   float64   `json:"duration_seconds"`
	ChunkCount        int       `json:"chunk_count"`
	ProcessingSeconds float64   `json:"processing_seconds"`
	AnalyzedAt        time.Time `json:"analyzed_at"`
}

// TaskOptions are per-task pipeline settings supplied at submission.
type TaskOptions struct {
	// SkipSummary disables LLM summarization; chunks keep empty summaries.
	SkipSummary bool `json:"skip_summary,omitempty"`
	// Language forces the Whisper language (ISO code); empty means auto-detect.
	Language string `json:"language,omitempty"`
	// Glossary is passed to Whisper as hotwords on every window.
	Glossary []string `json:"glossary,omitempty"`
	// InitialPrompt is the Whisper initial prompt for the first window.
	InitialPrompt string `json:"initial_prompt,omitempty"`
	// FrameDedupThreshold overrides the CLIP embedding dedup threshold; >= 1 disables it.
	FrameDedupThreshold float64 `json:"frame_dedup_threshold,omitempty"`
}
