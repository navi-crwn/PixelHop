package db

import "context"

// AddGuestUpload records one file stored by the guest address ipHash.
func (d *DB) AddGuestUpload(ctx context.Context, ipHash string) error {
	_, err := d.ExecContext(ctx, `INSERT INTO guest_uploads (ip_hash, at) VALUES (?,?)`, ipHash, now())
	return err
}

// GuestUploads counts the files the guest address ipHash stored since the
// given time, and says when the oldest of them was stored (0 when none).
func (d *DB) GuestUploads(ctx context.Context, ipHash string, since int64) (n int, oldest int64) {
	d.QueryRowContext(ctx, `SELECT COUNT(*), COALESCE(MIN(at),0) FROM guest_uploads WHERE ip_hash=? AND at>?`, ipHash, since).Scan(&n, &oldest)
	return n, oldest
}

// PruneGuestUploads forgets guest upload rows stored before the given time.
func (d *DB) PruneGuestUploads(ctx context.Context, before int64) {
	d.ExecContext(ctx, `DELETE FROM guest_uploads WHERE at<?`, before)
}
