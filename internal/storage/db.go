// Package storage persists screenshots in an embedded SQLite database (pure-Go driver, no CGO).
package storage

import (
	"context"
	"database/sql"
	"errors"
	"fmt"

	_ "modernc.org/sqlite" // registers the "sqlite" driver

	"github.com/sysm0n/Screenshotinator/internal/models"
)

// ErrNotFound is returned when a screenshot ID does not exist.
var ErrNotFound = errors.New("screenshot not found")

const schema = `
CREATE TABLE IF NOT EXISTS screenshots (
	id         INTEGER PRIMARY KEY AUTOINCREMENT,
	session_id TEXT NOT NULL,
	timestamp  TEXT NOT NULL,
	app_name   TEXT NOT NULL DEFAULT '',
	image_path TEXT NOT NULL,
	user_notes TEXT NOT NULL DEFAULT '',
	ai_summary TEXT NOT NULL DEFAULT ''
);
CREATE INDEX IF NOT EXISTS idx_screenshots_session ON screenshots(session_id, id);`

const columns = `id, session_id, timestamp, app_name, image_path, user_notes, ai_summary`

// DB is the screenshot repository. It is safe for concurrent use.
type DB struct{ db *sql.DB }

// Open opens (or creates) the database at path and applies the schema.
func Open(ctx context.Context, path string) (*DB, error) {
	// WAL + busy timeout: the capture pipeline and LLM workers write concurrently.
	db, err := sql.Open("sqlite", "file:"+path+"?_pragma=busy_timeout(5000)&_pragma=journal_mode(WAL)")
	if err != nil {
		return nil, fmt.Errorf("open %s: %w", path, err)
	}
	db.SetMaxOpenConns(1) // SQLite has a single writer; one connection avoids SQLITE_BUSY churn
	if _, err := db.ExecContext(ctx, schema); err != nil {
		db.Close()
		return nil, fmt.Errorf("migrate: %w", err)
	}
	return &DB{db: db}, nil
}

func (d *DB) Close() error { return d.db.Close() }

// Insert stores s and sets s.ID.
func (d *DB) Insert(ctx context.Context, s *models.Screenshot) error {
	res, err := d.db.ExecContext(ctx,
		`INSERT INTO screenshots (session_id, timestamp, app_name, image_path, user_notes, ai_summary)
		 VALUES (?, ?, ?, ?, ?, ?)`,
		s.SessionID, s.Timestamp, s.AppName, s.ImagePath, s.UserNotes, s.AISummary)
	if err != nil {
		return fmt.Errorf("insert screenshot: %w", err)
	}
	if s.ID, err = res.LastInsertId(); err != nil {
		return fmt.Errorf("insert screenshot: %w", err)
	}
	return nil
}

func (d *DB) Get(ctx context.Context, id int64) (*models.Screenshot, error) {
	s, err := scan(d.db.QueryRowContext(ctx, `SELECT `+columns+` FROM screenshots WHERE id = ?`, id))
	if errors.Is(err, sql.ErrNoRows) {
		return nil, fmt.Errorf("%w: id %d", ErrNotFound, id)
	}
	if err != nil {
		return nil, fmt.Errorf("get screenshot %d: %w", id, err)
	}
	return s, nil
}

// ListBySession returns the session's screenshots in capture order (never nil).
func (d *DB) ListBySession(ctx context.Context, sessionID string) ([]*models.Screenshot, error) {
	rows, err := d.db.QueryContext(ctx,
		`SELECT `+columns+` FROM screenshots WHERE session_id = ? ORDER BY id`, sessionID)
	if err != nil {
		return nil, fmt.Errorf("list session %s: %w", sessionID, err)
	}
	defer rows.Close()

	out := []*models.Screenshot{}
	for rows.Next() {
		s, err := scan(rows)
		if err != nil {
			return nil, fmt.Errorf("list session %s: %w", sessionID, err)
		}
		out = append(out, s)
	}
	return out, rows.Err()
}

func (d *DB) SetSummary(ctx context.Context, id int64, summary string) error {
	return d.update(ctx, `UPDATE screenshots SET ai_summary = ? WHERE id = ?`, summary, id)
}

func (d *DB) SetNotes(ctx context.Context, id int64, notes string) error {
	return d.update(ctx, `UPDATE screenshots SET user_notes = ? WHERE id = ?`, notes, id)
}

// Delete removes the row and returns its image path so the caller can remove the file.
func (d *DB) Delete(ctx context.Context, id int64) (imagePath string, err error) {
	err = d.db.QueryRowContext(ctx, `DELETE FROM screenshots WHERE id = ? RETURNING image_path`, id).Scan(&imagePath)
	if errors.Is(err, sql.ErrNoRows) {
		return "", fmt.Errorf("%w: id %d", ErrNotFound, id)
	}
	if err != nil {
		return "", fmt.Errorf("delete screenshot %d: %w", id, err)
	}
	return imagePath, nil
}

func (d *DB) update(ctx context.Context, query, value string, id int64) error {
	res, err := d.db.ExecContext(ctx, query, value, id)
	if err != nil {
		return fmt.Errorf("update screenshot %d: %w", id, err)
	}
	if n, err := res.RowsAffected(); err == nil && n == 0 {
		return fmt.Errorf("%w: id %d", ErrNotFound, id)
	}
	return nil
}

func scan(r interface{ Scan(...any) error }) (*models.Screenshot, error) {
	var s models.Screenshot
	if err := r.Scan(&s.ID, &s.SessionID, &s.Timestamp, &s.AppName, &s.ImagePath, &s.UserNotes, &s.AISummary); err != nil {
		return nil, err
	}
	return &s, nil
}
