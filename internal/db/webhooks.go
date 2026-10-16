package db

import "context"

type Webhook struct {
	ID         int64
	UserID     int64
	URL        string
	Secret     string
	Events     string
	CreatedAt  int64
	LastStatus string
}

func (d *DB) InsertWebhook(ctx context.Context, h *Webhook) error {
	res, err := d.ExecContext(ctx, `INSERT INTO webhooks (user_id,url,secret,events,created_at) VALUES (?,?,?,?,?)`,
		h.UserID, h.URL, h.Secret, h.Events, now())
	if err == nil {
		h.ID, _ = res.LastInsertId()
	}
	return err
}

func (d *DB) Webhooks(ctx context.Context, userID int64) ([]Webhook, error) {
	rows, err := d.QueryContext(ctx, `SELECT id,user_id,url,secret,events,created_at,last_status FROM webhooks WHERE user_id=? ORDER BY id`, userID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []Webhook
	for rows.Next() {
		var h Webhook
		if err := rows.Scan(&h.ID, &h.UserID, &h.URL, &h.Secret, &h.Events, &h.CreatedAt, &h.LastStatus); err != nil {
			return nil, err
		}
		out = append(out, h)
	}
	return out, rows.Err()
}

func (d *DB) DeleteWebhook(ctx context.Context, userID, id int64) error {
	_, err := d.ExecContext(ctx, `DELETE FROM webhooks WHERE id=? AND user_id=?`, id, userID)
	return err
}

func (d *DB) SetWebhookStatus(ctx context.Context, id int64, status string) {
	d.ExecContext(ctx, `UPDATE webhooks SET last_status=? WHERE id=?`, status, id)
}
