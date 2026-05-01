package ocr

import (
	"bytes"
	"context"
	"fmt"
	"os/exec"
	"strings"

	"go.uber.org/zap"
)

type Reader struct {
	languages []string
	logger    *zap.Logger
}

func New(languages []string, logger *zap.Logger) *Reader {
	return &Reader{languages: languages, logger: logger}
}

func (r *Reader) ReadText(ctx context.Context, imagePath string) (string, error) {
	lang := strings.Join(r.languages, "+")

	args := []string{imagePath, "stdout", "-l", lang}

	r.logger.Info("running tesseract", zap.Strings("args", args))

	cmd := exec.CommandContext(ctx, "tesseract", args...)
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	if err := cmd.Run(); err != nil {
		r.logger.Error("tesseract failed", zap.String("stderr", stderr.String()), zap.Error(err))
		return "", fmt.Errorf("tesseract: %w", err)
	}

	return strings.TrimSpace(stdout.String()), nil
}
