package db

import (
	"context"
	"testing"
)

// Migration 18 (reminders) runs again on a database that already has the table, as it does when
// an older schema_version meets it, and hiding a card is kept per account.
func TestRemindersMigrationRunsTwice(t *testing.T) {
	dir := t.TempDir()
	d, err := Open(dir)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := d.Exec(`DELETE FROM schema_version WHERE v=18`); err != nil {
		t.Fatal(err)
	}
	d.Close()
	d, err = Open(dir) // the real migration path, against a table that is already there
	if err != nil {
		t.Fatalf("migration 18 a second time: %v", err)
	}
	defer d.Close()
	ctx := context.Background()
	u := &User{Email: "r@example.com", QuotaBytes: 1}
	if err := d.InsertUser(ctx, u); err != nil {
		t.Fatal(err)
	}
	if at, err := d.ReminderDismissedAt(ctx, u.ID, "k"); err != nil || at != 0 {
		t.Fatalf("never hidden: %d %v", at, err)
	}
	if err := d.DismissReminder(ctx, u.ID, "k"); err != nil {
		t.Fatal(err)
	}
	if at, _ := d.ReminderDismissedAt(ctx, u.ID, "k"); at == 0 {
		t.Fatal("hiding was not kept")
	}
}
