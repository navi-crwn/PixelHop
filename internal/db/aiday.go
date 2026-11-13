package db

import (
	"context"
	"database/sql"
	"errors"
	"time"
)

// The AI tools' daily share lives here, not in memory, so a restart does not give a member the day back.
// A window starts at a member's first use of a tool and runs for per; the next use after it starts a new one.
// Every step below is a single statement, so two requests at the same moment cannot both take the last use.

// AITake counts one use of tool for the member in the current window. It reports the uses counted so far
// (this one included) and whether it was allowed; max <= 0 means no limit and counts nothing.
// A refused call is not counted, so AIGive makes room again.
func (d *DB) AITake(ctx context.Context, userID int64, tool string, max int, per time.Duration, at time.Time) (used int, ok bool, err error) {
	if max <= 0 {
		return 0, true, nil
	}
	// One statement opens the window: a new row, or the old one when its window has run out. No row comes
	// back while a window is still running. No half-made row is ever left for the sweep to delete in between.
	err = d.QueryRowContext(ctx, `INSERT INTO ai_day (user_id, tool, start_at, n) VALUES (?, ?, ?, 1)
		ON CONFLICT (user_id, tool) DO UPDATE SET start_at = excluded.start_at, n = 1 WHERE ai_day.start_at < ? RETURNING n`,
		userID, tool, at.Unix(), at.Add(-per).Unix()).Scan(&used)
	if err == nil {
		return used, true, nil
	}
	if !errors.Is(err, sql.ErrNoRows) {
		return 0, false, err
	}
	err = d.QueryRowContext(ctx, `UPDATE ai_day SET n = n + 1 WHERE user_id = ? AND tool = ? AND n < ? RETURNING n`, userID, tool, max).Scan(&used)
	if err == nil {
		return used, true, nil
	}
	if !errors.Is(err, sql.ErrNoRows) {
		return 0, false, err
	}
	used, _, err = d.AIUsed(ctx, userID, tool, per, at)
	return used, false, err
}

// AIGive takes back one use taken at or after since, for a call that gave no result after all (the AI
// service failed). It never goes below zero and leaves a window that began later alone.
func (d *DB) AIGive(ctx context.Context, userID int64, tool string, since time.Time) error {
	_, err := d.ExecContext(ctx, `UPDATE ai_day SET n = n - 1 WHERE user_id = ? AND tool = ? AND start_at <= ? AND n > 0`, userID, tool, since.Unix())
	return err
}

// AIUsed is how many results the member has had from tool in the current window, and when that window
// ends (the zero time when none is running).
func (d *DB) AIUsed(ctx context.Context, userID int64, tool string, per time.Duration, at time.Time) (used int, resets time.Time, err error) {
	var start int64
	err = d.QueryRowContext(ctx, `SELECT start_at, n FROM ai_day WHERE user_id = ? AND tool = ?`, userID, tool).Scan(&start, &used)
	if errors.Is(err, sql.ErrNoRows) {
		return 0, time.Time{}, nil
	}
	if err != nil {
		return 0, time.Time{}, err
	}
	if end := time.Unix(start, 0).Add(per); end.After(at) && start > 0 {
		return used, end, nil
	}
	return 0, time.Time{}, nil
}

// SweepAIDay drops the windows that have run out, so the table keeps only the day in progress.
func (d *DB) SweepAIDay(ctx context.Context, per time.Duration, at time.Time) error {
	_, err := d.ExecContext(ctx, `DELETE FROM ai_day WHERE start_at < ?`, at.Add(-per).Unix())
	return err
}
