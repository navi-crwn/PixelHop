package db

import (
	"context"
	"crypto/subtle"
	"database/sql"
	"errors"
)

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

// ClaimItem is one guest upload a browser wants to claim: the image id and
// the sha256 of the delete token that browser kept.
type ClaimItem struct{ ID, TokenHash string }

// ClaimSkip says why one item was not claimed: "invalid" (no such image, or the
// token does not match, or it belongs to someone else: deliberately one answer),
// "unavailable" (the token is right but the image is removed, reported, in the
// trash or expired) or "quota" (it would not fit the member's storage).
type ClaimSkip struct{ ID, Reason string }

// ClaimResult is the outcome of ClaimImages.
type ClaimResult struct {
	Claimed []string
	Already []string // claimed earlier by this same member
	Skipped []ClaimSkip
}

// ClaimImages moves guest uploads into a member's library, all in one
// transaction. An image is moved only while it has no owner, the token matches
// and it is neither removed, in the trash, expired nor reported. The token is
// forgotten (so the old delete link stops working) and an expiry that only came
// from the guest limit is cleared; one the guest chose themself stays. room is
// how many more bytes the member may store; guestTTL is the guest expiry in seconds (0 = none).
func (d *DB) ClaimImages(ctx context.Context, userID int64, items []ClaimItem, room, guestTTL int64) (ClaimResult, error) {
	var res ClaimResult
	tx, err := d.BeginTx(ctx, nil)
	if err != nil {
		return res, err
	}
	defer tx.Rollback()
	t := now()
	for _, it := range items {
		var (
			uid                                          sql.NullInt64
			tok                                          string
			hidden, deleted, claimedAt, size, expires, n int64
		)
		err := tx.QueryRowContext(ctx, `SELECT user_id, owner_token, hidden, deleted_at, claimed_at, size, expires_at,
			(SELECT COUNT(*) FROM reports WHERE image_id=images.id AND status<>'dismissed') FROM images WHERE id=?`, it.ID).
			Scan(&uid, &tok, &hidden, &deleted, &claimedAt, &size, &expires, &n)
		switch {
		case errors.Is(err, sql.ErrNoRows):
			res.Skipped = append(res.Skipped, ClaimSkip{it.ID, "invalid"})
			continue
		case err != nil:
			return res, err
		}
		if uid.Valid && uid.Int64 == userID && claimedAt > 0 {
			res.Already = append(res.Already, it.ID)
			continue
		}
		if uid.Valid || tok == "" || subtle.ConstantTimeCompare([]byte(tok), []byte(it.TokenHash)) != 1 {
			res.Skipped = append(res.Skipped, ClaimSkip{it.ID, "invalid"})
			continue
		}
		if hidden != 0 || deleted > 0 || n > 0 || (expires > 0 && expires <= t) {
			res.Skipped = append(res.Skipped, ClaimSkip{it.ID, "unavailable"})
			continue
		}
		if size > room {
			res.Skipped = append(res.Skipped, ClaimSkip{it.ID, "quota"})
			continue
		}
		r, err := tx.ExecContext(ctx, `UPDATE images SET user_id=?, owner_token='', claimed_at=?,
			expires_at=CASE WHEN ?>0 AND expires_at>0 AND expires_at-created_at>=?-60 THEN 0 ELSE expires_at END
			WHERE id=? AND user_id IS NULL AND owner_token=?`, userID, t, guestTTL, guestTTL, it.ID, tok)
		if err != nil {
			return res, err
		}
		if c, _ := r.RowsAffected(); c != 1 {
			res.Skipped = append(res.Skipped, ClaimSkip{it.ID, "invalid"})
			continue
		}
		room -= size
		res.Claimed = append(res.Claimed, it.ID)
	}
	return res, tx.Commit()
}
