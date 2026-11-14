package db

import (
	"database/sql"
	"testing"
	"time"
)

// Sessions a HOP ID member opened without the hub (verified_at -1) start being checked with it;
// the sessions of members without a HOP ID stay as they are.
func TestMigrationChecksOldSessionsOfHopIDMembers(t *testing.T) {
	ctx := t.Context()
	d, err := Open(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	defer d.Close()
	linked := &User{Email: "linked@example.com", QuotaBytes: 1}
	plain := &User{Email: "plain@example.com", QuotaBytes: 1}
	for _, u := range []*User{linked, plain} {
		if err := d.InsertUser(ctx, u); err != nil {
			t.Fatal(err)
		}
	}
	linked.HelinkID = sql.NullString{String: "7", Valid: true}
	if err := d.UpdateUser(ctx, linked); err != nil {
		t.Fatal(err)
	}
	must := func(err error) {
		t.Helper()
		if err != nil {
			t.Fatal(err)
		}
	}
	must(d.InsertSession(ctx, "linked-detached", linked.ID, time.Hour, "", ""))
	must(d.InsertSession(ctx, "plain-detached", plain.ID, time.Hour, "", ""))
	must(d.InsertHubSession(ctx, "linked-hub", linked.ID, time.Hour, "", ""))

	// Run the last migration again as an upgrade would.
	if _, err := d.Exec(`DELETE FROM schema_version WHERE v=?`, len(migrations)); err != nil {
		t.Fatal(err)
	}
	must(d.migrate())

	at := func(tok string) int64 {
		t.Helper()
		v, err := d.SessionVerifiedAt(ctx, tok)
		if err != nil {
			t.Fatal(err)
		}
		return v
	}
	if got := at("linked-detached"); got != 0 {
		t.Errorf("HOP ID member's detached session verified_at = %d, want 0 (checked with the hub)", got)
	}
	if got := at("plain-detached"); got != DetachedSession {
		t.Errorf("session of a member without HOP ID verified_at = %d, want %d (untouched)", got, DetachedSession)
	}
	if got := at("linked-hub"); got <= 0 {
		t.Errorf("a hub-vouched session lost its stamp: %d", got)
	}
}
