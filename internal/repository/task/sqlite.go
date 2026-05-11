package task

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"time"

	"github.com/um4ru-ch4n/youtube-analyzer-mcp/internal/model"

	_ "modernc.org/sqlite"
)

// Repository provides SQLite-backed storage for tasks.
type Repository struct {
	db *sql.DB
}

// New opens the SQLite database at dbPath, runs migrations, and returns a ready Repository.
func New(dbPath string) (*Repository, error) {
	db, err := sql.Open("sqlite", dbPath)
	if err != nil {
		return nil, fmt.Errorf("open sqlite: %w", err)
	}

	if err := db.Ping(); err != nil {
		db.Close()
		return nil, fmt.Errorf("ping sqlite: %w", err)
	}

	// Enable WAL mode for better concurrent read performance.
	if _, err := db.Exec("PRAGMA journal_mode=WAL"); err != nil {
		db.Close()
		return nil, fmt.Errorf("set WAL mode: %w", err)
	}

	// Set busy timeout to avoid SQLITE_BUSY on concurrent writes.
	if _, err := db.Exec("PRAGMA busy_timeout=5000"); err != nil {
		db.Close()
		return nil, fmt.Errorf("set busy_timeout: %w", err)
	}

	// Limit to 1 connection — SQLite handles one writer at a time.
	db.SetMaxOpenConns(1)

	if err := Migrate(db); err != nil {
		db.Close()
		return nil, fmt.Errorf("migrate: %w", err)
	}

	return &Repository{db: db}, nil
}

// Create inserts a new task record.
func (r *Repository) Create(ctx context.Context, task model.Task) error {
	warningsJSON, err := json.Marshal(task.Warnings)
	if err != nil {
		return fmt.Errorf("marshal warnings: %w", err)
	}

	_, err = r.db.ExecContext(ctx,
		`INSERT INTO tasks (id, video_url, status, progress, error, warnings_json, result_json, created_at, updated_at)
		 VALUES (?, ?, ?, ?, ?, ?, '', ?, ?)`,
		task.ID, task.VideoURL, string(task.Status), task.Progress, task.Error,
		string(warningsJSON), task.CreatedAt.UTC(), task.UpdatedAt.UTC(),
	)
	if err != nil {
		return fmt.Errorf("insert task: %w", err)
	}

	return nil
}

// Get retrieves a task by ID. Returns model.ErrTaskNotFound if the task does not exist.
func (r *Repository) Get(ctx context.Context, taskID string) (model.Task, error) {
	row := r.db.QueryRowContext(ctx,
		`SELECT id, video_url, status, progress, error, warnings_json, created_at, updated_at
		 FROM tasks WHERE id = ?`, taskID,
	)

	var t model.Task
	var status string
	var warningsJSON string
	var createdAt, updatedAt string

	err := row.Scan(&t.ID, &t.VideoURL, &status, &t.Progress, &t.Error, &warningsJSON, &createdAt, &updatedAt)
	if err == sql.ErrNoRows {
		return model.Task{}, model.ErrTaskNotFound
	}
	if err != nil {
		return model.Task{}, fmt.Errorf("scan task: %w", err)
	}

	t.Status = model.TaskStatus(status)

	if err := json.Unmarshal([]byte(warningsJSON), &t.Warnings); err != nil {
		return model.Task{}, fmt.Errorf("unmarshal warnings: %w", err)
	}

	t.CreatedAt, err = time.Parse("2006-01-02 15:04:05+00:00", createdAt)
	if err != nil {
		t.CreatedAt, err = time.Parse("2006-01-02T15:04:05Z", createdAt)
		if err != nil {
			t.CreatedAt, _ = time.Parse(time.RFC3339Nano, createdAt)
		}
	}

	t.UpdatedAt, err = time.Parse("2006-01-02 15:04:05+00:00", updatedAt)
	if err != nil {
		t.UpdatedAt, err = time.Parse("2006-01-02T15:04:05Z", updatedAt)
		if err != nil {
			t.UpdatedAt, _ = time.Parse(time.RFC3339Nano, updatedAt)
		}
	}

	return t, nil
}

// UpdateStatus updates the status and progress of a task.
func (r *Repository) UpdateStatus(ctx context.Context, taskID string, status model.TaskStatus, progress string) error {
	res, err := r.db.ExecContext(ctx,
		`UPDATE tasks SET status = ?, progress = ?, updated_at = ? WHERE id = ?`,
		string(status), progress, time.Now().UTC(), taskID,
	)
	if err != nil {
		return fmt.Errorf("update status: %w", err)
	}

	return checkRowsAffected(res, taskID)
}

// UpdateWarnings stores the given warnings as JSON on the task.
func (r *Repository) UpdateWarnings(ctx context.Context, taskID string, warnings []model.Warning) error {
	warningsJSON, err := json.Marshal(warnings)
	if err != nil {
		return fmt.Errorf("marshal warnings: %w", err)
	}

	res, err := r.db.ExecContext(ctx,
		`UPDATE tasks SET warnings_json = ?, updated_at = ? WHERE id = ?`,
		string(warningsJSON), time.Now().UTC(), taskID,
	)
	if err != nil {
		return fmt.Errorf("update warnings: %w", err)
	}

	return checkRowsAffected(res, taskID)
}

// SaveResult stores the task result as JSON and marks the task as completed.
func (r *Repository) SaveResult(ctx context.Context, taskID string, result model.TaskResult, processingDuration time.Duration) error {
	resultJSON, err := json.Marshal(result)
	if err != nil {
		return fmt.Errorf("marshal result: %w", err)
	}

	res, err := r.db.ExecContext(ctx,
		`UPDATE tasks SET result_json = ?, status = ?, updated_at = ?,
		     video_title = ?, duration_seconds = ?, chunk_count = ?, processing_seconds = ?
		 WHERE id = ?`,
		string(resultJSON), string(model.TaskStatusCompleted), time.Now().UTC(),
		result.VideoMeta.Title, result.DurationSeconds, len(result.Chunks),
		processingDuration.Seconds(),
		taskID,
	)
	if err != nil {
		return fmt.Errorf("save result: %w", err)
	}

	return checkRowsAffected(res, taskID)
}

// GetResult retrieves the analysis result for a completed task.
func (r *Repository) GetResult(ctx context.Context, taskID string) (model.TaskResult, error) {
	row := r.db.QueryRowContext(ctx,
		`SELECT result_json FROM tasks WHERE id = ?`, taskID,
	)

	var resultJSON string
	err := row.Scan(&resultJSON)
	if err == sql.ErrNoRows {
		return model.TaskResult{}, model.ErrTaskNotFound
	}
	if err != nil {
		return model.TaskResult{}, fmt.Errorf("scan result: %w", err)
	}

	if resultJSON == "" {
		return model.TaskResult{}, model.ErrTaskNotCompleted
	}

	var result model.TaskResult
	if err := json.Unmarshal([]byte(resultJSON), &result); err != nil {
		return model.TaskResult{}, fmt.Errorf("unmarshal result: %w", err)
	}

	return result, nil
}

// Delete removes a task by ID.
func (r *Repository) Delete(ctx context.Context, taskID string) error {
	res, err := r.db.ExecContext(ctx, `DELETE FROM tasks WHERE id = ?`, taskID)
	if err != nil {
		return fmt.Errorf("delete task: %w", err)
	}

	return checkRowsAffected(res, taskID)
}

// List returns tasks ordered by creation time descending, with limit and offset.
func (r *Repository) List(ctx context.Context, limit, offset int) ([]model.Task, error) {
	rows, err := r.db.QueryContext(ctx,
		`SELECT id, video_url, status, progress, error, warnings_json, created_at, updated_at
		 FROM tasks ORDER BY created_at DESC LIMIT ? OFFSET ?`, limit, offset,
	)
	if err != nil {
		return nil, fmt.Errorf("query tasks: %w", err)
	}
	defer rows.Close()

	var tasks []model.Task
	for rows.Next() {
		var t model.Task
		var status string
		var warningsJSON string
		var createdAt, updatedAt string

		if err := rows.Scan(&t.ID, &t.VideoURL, &status, &t.Progress, &t.Error, &warningsJSON, &createdAt, &updatedAt); err != nil {
			return nil, fmt.Errorf("scan task row: %w", err)
		}

		t.Status = model.TaskStatus(status)

		if err := json.Unmarshal([]byte(warningsJSON), &t.Warnings); err != nil {
			return nil, fmt.Errorf("unmarshal warnings: %w", err)
		}

		t.CreatedAt, _ = parseTime(createdAt)
		t.UpdatedAt, _ = parseTime(updatedAt)

		tasks = append(tasks, t)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("rows iteration: %w", err)
	}

	return tasks, nil
}

// ListCompleted returns all completed tasks with denormalized video metadata,
// ordered by creation time descending. Used by the list_videos MCP tool for cross-session discovery.
func (r *Repository) ListCompleted(ctx context.Context, limit, offset int) ([]model.VideoSummary, error) {
	rows, err := r.db.QueryContext(ctx,
		`SELECT id, video_url, video_title, duration_seconds, chunk_count, processing_seconds, created_at
		 FROM tasks WHERE status = 'completed'
		 ORDER BY created_at DESC LIMIT ? OFFSET ?`,
		limit, offset,
	)
	if err != nil {
		return nil, fmt.Errorf("query completed tasks: %w", err)
	}
	defer rows.Close()

	var result []model.VideoSummary
	for rows.Next() {
		var v model.VideoSummary
		var createdAt string
		if err := rows.Scan(&v.TaskID, &v.VideoURL, &v.VideoTitle, &v.DurationSeconds, &v.ChunkCount, &v.ProcessingSeconds, &createdAt); err != nil {
			return nil, fmt.Errorf("scan video summary: %w", err)
		}
		v.AnalyzedAt, _ = parseTime(createdAt)
		result = append(result, v)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("rows iteration: %w", err)
	}
	return result, nil
}

// Close closes the underlying database connection.
func (r *Repository) Close() error {
	return r.db.Close()
}

func checkRowsAffected(res sql.Result, taskID string) error {
	n, err := res.RowsAffected()
	if err != nil {
		return fmt.Errorf("rows affected: %w", err)
	}
	if n == 0 {
		return model.ErrTaskNotFound
	}
	return nil
}

func parseTime(s string) (time.Time, error) {
	t, err := time.Parse("2006-01-02 15:04:05+00:00", s)
	if err == nil {
		return t, nil
	}

	t, err = time.Parse("2006-01-02T15:04:05Z", s)
	if err == nil {
		return t, nil
	}

	return time.Parse(time.RFC3339Nano, s)
}
