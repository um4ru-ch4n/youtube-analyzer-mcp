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

// Frame represents an extracted video frame with its classification.
type Frame struct {
	TimestampSec float64   `json:"timestamp_sec"`
	FilePath     string    `json:"file_path"`
	Type         FrameType `json:"type"`
}

// FrameClassification holds the CLIP classification result for a frame.
type FrameClassification struct {
	Type       FrameType `json:"type"`
	Confidence float64   `json:"confidence"`
}

// FrameContent holds the textual content extracted from a processed frame.
type FrameContent struct {
	TimestampSec float64   `json:"timestamp_sec"`
	Type         FrameType `json:"frame_type"`
	Content      string    `json:"content"`
}
