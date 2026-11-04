package app

import (
	"bytes"
	"context"
	"errors"
	"testing"

	"github.com/navi-crwn/pixelhop/internal/db"
	"github.com/navi-crwn/pixelhop/internal/storage"
)

// failingDelete is a storage backend whose deletes fail on demand.
type failingDelete struct {
	storage.Backend
	fail       bool
	deleteCall int
}

func (f *failingDelete) Delete(ctx context.Context, key string) error {
	f.deleteCall++
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

// "Deletes every image" includes the ones waiting in the trash: without this the row and the files outlive the account.
func TestPurgeUserRemovesTrashedImages(t *testing.T) {
	a, _ := ssoApp(t)
	ctx := context.Background()
	u, _ := member(t, a, "gone@example.com")
	live, trashed := addImage(t, a, u, "live-pic"), addImage(t, a, u, "trashed-pic")
	for _, im := range []*db.Image{live, trashed} {
		if err := a.store.Put(ctx, im.KeyFull, bytes.NewReader([]byte("jpeg bytes")), 10, "image/jpeg"); err != nil {
			t.Fatal(err)
		}
	}
	if err := a.db.TrashImage(ctx, trashed.ID); err != nil {
		t.Fatal(err)
	}

	n, err := a.purgeUser(ctx, u)
	if err != nil {
		t.Fatal(err)
	}
	if n != 2 {
		t.Errorf("purge counted %d images, want 2", n)
	}
	if _, err := a.db.Image(ctx, trashed.ID); err == nil {
		t.Error("the row of the trashed image is still there")
	}
	if hasObject(a, trashed.KeyFull) {
		t.Error("the trashed image is still in storage")
	}
}

// A trashed image that cannot be deleted keeps the account, like a live one: the purge can run again.
func TestPurgeUserKeepsAccountWhenTrashedImageStays(t *testing.T) {
	a, _ := ssoApp(t)
	ctx := context.Background()
	u, _ := member(t, a, "gone@example.com")
	im := addImage(t, a, u, "trashed-pic")
	if err := a.db.TrashImage(ctx, im.ID); err != nil {
		t.Fatal(err)
	}
	broken := &failingDelete{Backend: a.backends["local"], fail: true}
	a.backends["local"] = broken

	if _, err := a.purgeUser(ctx, u); err == nil {
		t.Error("purge reported success with an image left")
	}
	if _, err := a.db.UserByID(ctx, u.ID); err != nil {
		t.Errorf("the account went first: %v", err)
	}
}

// When the images cannot be listed nothing says they are gone, so the account stays and the error comes back.
func TestPurgeUserKeepsAccountWhenImagesCannotBeListed(t *testing.T) {
	for name, trash := range map[string]bool{"live": false, "trash": true} {
		t.Run(name, func(t *testing.T) {
			a, _ := ssoApp(t)
			ctx := context.Background()
			u, _ := member(t, a, "gone@example.com")
			im := addImage(t, a, u, "unreadable-pic")
			if trash {
				if err := a.db.TrashImage(ctx, im.ID); err != nil {
					t.Fatal(err)
				}
			}
			// A row that does not scan makes the listing fail while every other query still works.
			if _, err := a.db.ExecContext(ctx, `UPDATE images SET size='not a number' WHERE id=?`, im.ID); err != nil {
				t.Fatal(err)
			}
			if _, err := a.purgeUser(ctx, u); err == nil {
				t.Error("purge reported success although the images could not be listed")
			}
			if _, err := a.db.UserByID(ctx, u.ID); err != nil {
				t.Errorf("the account was deleted: %v", err)
			}
		})
	}
}
