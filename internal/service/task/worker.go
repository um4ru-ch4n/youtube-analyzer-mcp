package task

import (
	"context"

	"github.com/um4ru-ch4n/youtube-analyzer-mcp/internal/model"
	"github.com/um4ru-ch4n/youtube-analyzer-mcp/pkg/logger"
)

// runWorker processes tasks from the queue until ctx is cancelled.
func (m *Manager) runWorker(ctx context.Context, workerID int) {
	ctx = logger.WithKV(ctx, "worker_id", workerID)
	logger.InfoKV(ctx, "worker started")

	for {
		select {
		case <-ctx.Done():
			logger.InfoKV(ctx, "worker stopped")
			return
		case taskID := <-m.queue:
			m.processTask(ctx, taskID)
		}
	}
}

func (m *Manager) processTask(ctx context.Context, taskID string) {
	ctx = logger.WithTaskID(ctx, taskID)
	logger.InfoKV(ctx, "processing task")

	task, err := m.repo.Get(ctx, taskID)
	if err != nil {
		logger.ErrorKV(ctx, "failed to get task from repo", "error", err.Error())
		return
	}

	err = m.repo.UpdateStatus(ctx, taskID, model.TaskStatusDownloading, string(model.TaskStatusDownloading))
	if err != nil {
		logger.ErrorKV(ctx, "failed to update task status", "error", err.Error())
		return
	}

	result, err := m.pipeline.Run(ctx, task)
	if err != nil {
		logger.ErrorKV(ctx, "pipeline failed", "error", err.Error())

		updateErr := m.repo.UpdateStatus(ctx, taskID, model.TaskStatusFailed, err.Error())
		if updateErr != nil {
			logger.ErrorKV(ctx, "failed to update task status to failed", "error", updateErr.Error())
		}

		return
	}

	err = m.repo.SaveResult(ctx, taskID, result)
	if err != nil {
		logger.ErrorKV(ctx, "failed to save task result", "error", err.Error())

		updateErr := m.repo.UpdateStatus(ctx, taskID, model.TaskStatusFailed, err.Error())
		if updateErr != nil {
			logger.ErrorKV(ctx, "failed to update task status to failed", "error", updateErr.Error())
		}

		return
	}

	err = m.repo.UpdateStatus(ctx, taskID, model.TaskStatusCompleted, string(model.TaskStatusCompleted))
	if err != nil {
		logger.ErrorKV(ctx, "failed to update task status to completed", "error", err.Error())
		return
	}

	logger.InfoKV(ctx, "task completed successfully")
}
