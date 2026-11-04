package app

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/navi-crwn/pixelhop/internal/storage"
)

// refuseKey is a storage backend that cannot delete one picture's files.
type refuseKey struct {
	storage.Backend
	prefix string
}

func (r *refuseKey) Delete(ctx context.Context, key string) error {
	if strings.HasPrefix(key, r.prefix) {
		return errors.New("storage unavailable")
	}
	return r.Backend.Delete(ctx, key)
}

// One picture whose files cannot be deleted must not hold back the rest of the sweep: the other expired and
// trashed pictures go, the stuck one stays for the next run, and old sessions are still cleared.
func TestSweepSkipsAPictureItCannotDelete(t *testing.T) {
	a, _ := ssoApp(t)
	ctx := context.Background()
	u := linked(t, a, "sweep-user", "sweep@example.com")
	stuck := addImage(t, a, u, "aa-stuck")
	var others []string
	for _, id := range []string{"bb-expired", "cc-expired"} {
		others = append(others, addImage(t, a, u, id).ID)
	}
	trashed := addImage(t, a, u, "dd-trashed")
	stuckTrash := addImage(t, a, u, "aa-stuck-trash")
	past := time.Now().Add(-time.Hour).Unix()
	for _, id := range append(others, stuck.ID) {
		if _, err := a.db.ExecContext(ctx, `UPDATE images SET expires_at=? WHERE id=?`, past, id); err != nil {
			t.Fatal(err)
		}
	}
	for _, id := range []string{trashed.ID, stuckTrash.ID} {
		if _, err := a.db.ExecContext(ctx, `UPDATE images SET deleted_at=?, hidden=1 WHERE id=?`, time.Now().Add(-30*24*time.Hour).Unix(), id); err != nil {
			t.Fatal(err)
		}
	}
	if err := a.db.InsertSession(ctx, "old-session", u.ID, -time.Hour, "", ""); err != nil {
		t.Fatal(err)
	}
	broken := &refuseKey{Backend: a.store, prefix: "aa-"}
	a.store, a.backends["local"] = broken, broken

	done := make(chan struct{})
	go func() { a.sweep(ctx); close(done) }()
	select {
	case <-done:
	case <-time.After(20 * time.Second):
		t.Fatal("sweep does not finish")
	}

	for _, id := range append(others, trashed.ID) {
		if _, err := a.db.Image(ctx, id); err == nil {
			t.Errorf("%s survived the sweep", id)
		}
	}
	for _, id := range []string{stuck.ID, stuckTrash.ID} {
		if _, err := a.db.Image(ctx, id); err != nil {
			t.Errorf("%s lost its record although its files are still stored", id)
		}
	}
	var n int
	a.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM sessions WHERE token_hash='old-session'`).Scan(&n)
	if n != 0 {
		t.Error("the expired session was not cleared")
	}
}
