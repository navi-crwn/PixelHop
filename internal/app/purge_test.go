package app

import (
	"bytes"
	"context"
	"errors"
	"testing"

	"github.com/navi-crwn/pixelhop/internal/storage"
)

// failingDelete is a storage backend whose deletes fail on demand.
type failingDelete struct {
	storage.Backend
	fail bool
}

func (f *failingDelete) Delete(ctx context.Context, key string) error {
	if f.fail {
		return errors.New("storage unavailable")
	}
	return f.Backend.Delete(ctx, key)
}

func storedAvatar(t *testing.T, a *App, id int64, key string) {
	t.Helper()
	ctx := context.Background()
	if err := a.store.Put(ctx, key, bytes.NewReader([]byte("jpeg bytes")), 10, "image/jpeg"); err != nil {
		t.Fatal(err)
	}
	u, err := a.db.UserByID(ctx, id)
	if err != nil {
		t.Fatal(err)
	}
	u.AvatarKey = key
	if err := a.db.UpdateUser(ctx, u); err != nil {
		t.Fatal(err)
	}
}

func hasObject(a *App, key string) bool {
	obj, err := a.store.Get(context.Background(), key, "")
	if err != nil {
		return false
	}
	obj.Body.Close()
	return true
}

func TestPurgeUserRemovesProfilePhoto(t *testing.T) {
	a, _ := ssoApp(t)
	ctx := context.Background()
	u, _ := member(t, a, "gone@example.com")
	storedAvatar(t, a, u.ID, "avatars/7-abcdef.jpg")
	u, _ = a.db.UserByID(ctx, u.ID)

	if _, err := a.purgeUser(ctx, u); err != nil {
		t.Fatal(err)
	}
	if hasObject(a, "avatars/7-abcdef.jpg") {
		t.Error("the profile photo is still in storage")
	}
	if _, err := a.db.UserByID(ctx, u.ID); err == nil {
		t.Error("the account is still there")
	}
}

// A photo that is already gone (removed by hand, or a purge that ran twice)
// must not keep the account alive.
func TestPurgeUserPhotoAlreadyGone(t *testing.T) {
	a, _ := ssoApp(t)
	ctx := context.Background()
	u, _ := member(t, a, "twice@example.com")
	u.AvatarKey = "avatars/9-missing.jpg"
	if err := a.db.UpdateUser(ctx, u); err != nil {
		t.Fatal(err)
	}
	if _, err := a.purgeUser(ctx, u); err != nil {
		t.Fatalf("missing photo: %v", err)
	}
	if _, err := a.db.UserByID(ctx, u.ID); err == nil {
		t.Error("the account is still there")
	}
}

// When the photo cannot be deleted the account row (the only record of the
// photo's key) stays, so the purge can be run again.
func TestPurgeUserKeepsAccountWhenPhotoDeleteFails(t *testing.T) {
	a, _ := ssoApp(t)
	ctx := context.Background()
	u, _ := member(t, a, "retry@example.com")
	storedAvatar(t, a, u.ID, "avatars/8-retry.jpg")
	u, _ = a.db.UserByID(ctx, u.ID)

	real := a.store
	flaky := &failingDelete{Backend: real, fail: true}
	a.store = flaky
	if _, err := a.purgeUser(ctx, u); err == nil {
		t.Fatal("a failed photo delete was not reported")
	}
	if got, err := a.db.UserByID(ctx, u.ID); err != nil || got.AvatarKey != "avatars/8-retry.jpg" {
		t.Fatalf("account should be untouched after the failure: %v %+v", err, got)
	}
	if !hasObject(a, "avatars/8-retry.jpg") {
		t.Fatal("photo vanished although the delete failed")
	}

	flaky.fail = false
	if _, err := a.purgeUser(ctx, u); err != nil {
		t.Fatalf("retry: %v", err)
	}
	if hasObject(a, "avatars/8-retry.jpg") {
		t.Error("the profile photo is still in storage after the retry")
	}
	if _, err := a.db.UserByID(ctx, u.ID); err == nil {
		t.Error("the account is still there after the retry")
	}
}
