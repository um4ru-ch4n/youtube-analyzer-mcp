package pipeline

import (
	"context"

	"github.com/um4ru-ch4n/youtube-analyzer-mcp/internal/model"
)

type Downloader interface {
	Download(ctx context.Context, url, outputDir string) (model.DownloadResult, error)
}

type Transcriber interface {
	Transcribe(ctx context.Context, audioPath string) (model.Transcript, error)
}

type FrameExtractor interface {
	ExtractFrames(ctx context.Context, videoPath, outputDir string) ([]model.Frame, error)
}

type FrameClassifier interface {
	Classify(ctx context.Context, imagePath string) (model.FrameClassification, error)
	ClassifyBatch(ctx context.Context, paths []string) ([]model.FrameClassification, error)
}

type OCRReader interface {
	ReadText(ctx context.Context, imagePath string) (string, error)
}

type VisionAnalyzer interface {
	AnalyzeImage(ctx context.Context, imagePath, prompt string) (string, error)
	IsUsefulFrame(ctx context.Context, imagePath string) (bool, error)
}

type Summarizer interface {
	SummarizeChunk(ctx context.Context, videoTitle string, transcriptText string, compressionRatio int) (string, error)
}

type Deduplicator interface {
	FilterDuplicates(ctx context.Context, frames []model.Frame) ([]model.Frame, error)
}
