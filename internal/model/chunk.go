package model

// Chunk groups transcript segments within a time range for summarization.
// Frames are matched separately as artifacts.
type Chunk struct {
	Index     int                 `json:"index"`
	TimeStart float64             `json:"time_start"`
	TimeEnd   float64             `json:"time_end"`
	Segments  []TranscriptSegment `json:"segments"`
}

// Artifact is a visual element extracted from the video, matched to a chunk by timestamp.
type Artifact struct {
	TimestampSec float64   `json:"timestamp_sec"`
	Type         FrameType `json:"frame_type"`
	ImagePath    string    `json:"image_path,omitempty"`
	OCRText      string    `json:"ocr_text,omitempty"`
}

// ChunkSummary is the final per-chunk output containing a text summary
// and associated visual artifacts.
type ChunkSummary struct {
	Index     int        `json:"index"`
	TimeStart float64    `json:"time_start"`
	TimeEnd   float64    `json:"time_end"`
	Summary   string     `json:"summary"`
	Artifacts []Artifact `json:"artifacts"`
}
