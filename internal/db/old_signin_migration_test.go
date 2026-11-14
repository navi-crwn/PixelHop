package db

import (
	"database/sql"
	"testing"
)

// The password and Google link an account kept from before it was linked to HOP ID are emptied once;
// accounts without a HOP ID, and everything else on the linked rows, stay as they are. Running it again
// changes nothing.
func TestMigrationEmptiesOldSignInOfLinkedAccounts(t *testing.T) {
	ctx := t.Context()
	d, err := Open(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	defer d.Close()
	mk := func(email string, helink bool, pass, google string) *User {
		u := &User{Email: email, Name: "N", QuotaBytes: 7, PassHash: pass}
		if google != "" {
			u.GoogleID = sql.NullString{String: google, Valid: true}
		}
		if err := d.InsertUser(ctx, u); err != nil {
			t.Fatal(err)
		}
		if helink {
			u.HelinkID = sql.NullString{String: email, Valid: true}
		}
		u.Role, u.Bio = "admin", "keep me"
		if err := d.UpdateUser(ctx, u); err != nil {
			t.Fatal(err)
		}
		return u
	}
	both := mk("both@example.com", true, "hash", "g-1")
	passOnly := mk("pass@example.com", true, "hash", "")
	plain := mk("plain@example.com", false, "hash", "g-2")
	if err := d.InsertSession(ctx, "tok", both.ID, 3600e9, "", ""); err != nil {
		t.Fatal(err)
	}

	run := func() {
		if _, err := d.Exec(`DELETE FROM schema_version WHERE v=?`, len(migrations)); err != nil {
			t.Fatal(err)
		}
		if err := d.migrate(); err != nil {
			t.Fatal(err)
		}
	}
	run()
	run() // a second run is harmless
	for _, u := range []*User{both, passOnly} {
		got, _ := d.UserByID(ctx, u.ID)
		if got.PassHash != "" || got.GoogleID.Valid {
			t.Errorf("%s kept an old way in: pass=%q google=%v", u.Email, got.PassHash, got.GoogleID)
		}
		if got.Role != "admin" || got.Bio != "keep me" || got.QuotaBytes != 7 || !got.HelinkID.Valid {
			t.Errorf("%s lost something else: %+v", u.Email, got)
		}
	}
	got, _ := d.UserByID(ctx, plain.ID)
	if got.PassHash != "hash" || got.GoogleID.String != "g-2" {
		t.Errorf("an account without HOP ID was touched: %+v", got)
	}
	if _, err := d.SessionUser(ctx, "tok"); err != nil {
		t.Errorf("a valid session was removed: %v", err)
	}
}
