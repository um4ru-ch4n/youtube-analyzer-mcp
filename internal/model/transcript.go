package model

// TranscriptSegment represents a single timestamped segment of speech.
type TranscriptSegment struct {
	StartSec float64 `json:"start_sec"`
	EndSec   float64 `json:"end_sec"`
	Text     string  `json:"text"`
}

// Transcript holds the full transcription result with language info.
type Transcript struct {
	Segments []TranscriptSegment `json:"segments"`
	Language string              `json:"language"`
}
