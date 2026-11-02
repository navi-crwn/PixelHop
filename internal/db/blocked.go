package db

import "context"

// BlockHash remembers the SHA-256 of a file taken down for breaking the rules,
// so nobody can upload the same file again.
func (d *DB) BlockHash(ctx context.Context, sha, reason string) {
	if sha == "" {
		return
	}
	d.ExecContext(ctx, `INSERT OR IGNORE INTO blocked_hashes (sha256,reason,created_at) VALUES (?,?,?)`, sha, reason, now())
}

func (d *DB) HashBlocked(ctx context.Context, sha string) bool {
	var one int
	return d.QueryRowContext(ctx, `SELECT 1 FROM blocked_hashes WHERE sha256=?`, sha).Scan(&one) == nil
}
