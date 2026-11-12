package db

import (
	"context"
	"database/sql"
	"errors"
)

// DismissReminder records that a member hid a reminder card just now; hiding
// it again later moves the time forward.
func (d *DB) DismissReminder(ctx context.Context, userID int64, key string) error {
	_, err := d.ExecContext(ctx, `INSERT INTO reminders (user_id, key, dismissed_at) VALUES (?, ?, ?)
		ON CONFLICT(user_id, key) DO UPDATE SET dismissed_at = excluded.dismissed_at`, userID, key, now())
	return err
}

// ReminderDismissedAt is when the member last hid that card, 0 when never.
func (d *DB) ReminderDismissedAt(ctx context.Context, userID int64, key string) (int64, error) {
	var at int64
	err := d.QueryRowContext(ctx, `SELECT dismissed_at FROM reminders WHERE user_id = ? AND key = ?`, userID, key).Scan(&at)
	if errors.Is(err, sql.ErrNoRows) {
		return 0, nil
	}
	return at, err
}
