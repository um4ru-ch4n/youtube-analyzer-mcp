package ocr

import "context"

// NoopReader implements OCRReader without doing any work. Used when OCR is
// disabled in config — we still need an OCRReader to satisfy the pipeline
// interface, but slide/code frames just pass through without text.
type NoopReader struct{}

func NewNoop() *NoopReader {
	return &NoopReader{}
}

func (NoopReader) ReadText(_ context.Context, _ string) (string, error) {
	return "", nil
}

func (NoopReader) ReleaseVRAM(_ context.Context) error {
	return nil
}
