package task

import (
	"database/sql"
	"fmt"
	"strings"
)

const createTasksTable = `
CREATE TABLE IF NOT EXISTS tasks (
    id TEXT PRIMARY KEY,
    video_url TEXT NOT NULL,
    status TEXT NOT NULL DEFAULT 'queued',
    progress TEXT NOT NULL DEFAULT '',
    error TEXT NOT NULL DEFAULT '',
    warnings_json TEXT NOT NULL DEFAULT '[]',
    result_json TEXT NOT NULL DEFAULT '',
    created_at DATETIME NOT NULL,
    updated_at DATETIME NOT NULL
);
`

// Migrate applies all necessary schema migrations to the database.
func Migrate(db *sql.DB) error {
	if _, err := db.Exec(createTasksTable); err != nil {
		return err
	}
	for _, stmt := range []string{
		`ALTER TABLE tasks ADD COLUMN video_title TEXT NOT NULL DEFAULT ''`,
		`ALTER TABLE tasks ADD COLUMN duration_seconds REAL NOT NULL DEFAULT 0`,
		`ALTER TABLE tasks ADD COLUMN chunk_count INTEGER NOT NULL DEFAULT 0`,
		`ALTER TABLE tasks ADD COLUMN processing_seconds REAL NOT NULL DEFAULT 0`,
	} {
		if err := execIdempotentAlter(db, stmt); err != nil {
			return fmt.Errorf("migration add_video_meta: %w", err)
		}
	}
	return nil
}

// execIdempotentAlter runs an ALTER TABLE ADD COLUMN statement and ignores
// "duplicate column name" errors so migrations are safe to re-run.
func execIdempotentAlter(db *sql.DB, stmt string) error {
	_, err := db.Exec(stmt)
	if err != nil && strings.Contains(err.Error(), "duplicate column name") {
		return nil
	}
	return err
}
