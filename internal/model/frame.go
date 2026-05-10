package model

// FrameType classifies the visual content of an extracted frame.
type FrameType string

const (
	FrameTypeTalkingHead FrameType = "talking_head"
	FrameTypeSlide       FrameType = "slide"
	FrameTypeCode        FrameType = "code"
	FrameTypeDiagram     FrameType = "diagram"
	FrameTypeOther       FrameType = "other"
)

// Frame represents an extracted video frame before classification.
type Frame struct {
	TimestampSec float64   `json:"timestamp_sec"`
	FilePath     string    `json:"file_path"`
	Type         FrameType `json:"type"`
}

// FrameClassification holds the CLIP classification result for a frame.
type FrameClassification struct {
	Type       FrameType `json:"type"`
	Confidence float64   `json:"confidence"`
	// Embedding is L2-normalized image embedding from CLIP (typically 768-dim).
	// Empty when classification was done via /classify (without embedding).
	Embedding []float32 `json:"embedding,omitempty"`
}

// ProcessedFrame is a frame after CLIP classification + optional OCR.
// It carries enough info to become an Artifact in the final result.
type ProcessedFrame struct {
	TimestampSec float64   `json:"timestamp_sec"`
	Type         FrameType `json:"frame_type"`
	ImagePath    string    `json:"image_path"`
	OCRText      string    `json:"ocr_text,omitempty"`
	Useful       bool      `json:"useful"`
}
