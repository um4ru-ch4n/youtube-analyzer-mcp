package model

import (
	"errors"
	"fmt"
)

// Sentinel errors for task lifecycle and validation.
var (
	ErrTaskNotFound    = errors.New("task not found")
	ErrTaskNotCompleted = errors.New("task not completed")
	ErrTaskNotDeletable = errors.New("task can only be deleted when completed or failed")
	ErrVideoTooLong    = errors.New("video exceeds maximum allowed duration")
	ErrDownloadFailed  = errors.New("video download failed")
	ErrInvalidURL      = errors.New("invalid YouTube URL")
)

// PipelineError wraps an error with the pipeline step where it occurred.
type PipelineError struct {
	Step  string
	Cause error
}

func (e PipelineError) Error() string {
	return fmt.Sprintf("pipeline step %q failed: %v", e.Step, e.Cause)
}

func (e PipelineError) Unwrap() error {
	return e.Cause
}
