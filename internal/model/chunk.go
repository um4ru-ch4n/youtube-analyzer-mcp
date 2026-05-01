package model

// Chunk groups transcript segments and frame content within a time range
// for summarization.
type Chunk struct {
	Index     int                 `json:"index"`
	TimeStart float64             `json:"time_start"`
	TimeEnd   float64             `json:"time_end"`
	Segments  []TranscriptSegment `json:"segments"`
	Frames    []FrameContent      `json:"frames"`
}

// ChunkSummary is the final per-chunk output containing a text summary
// and associated frame content.
type ChunkSummary struct {
	Index     int            `json:"index"`
	TimeStart float64        `json:"time_start"`
	TimeEnd   float64        `json:"time_end"`
	Summary   string         `json:"summary"`
	Frames    []FrameContent `json:"frames"`
}
