package app

import (
	"bytes"
	"context"
	"database/sql"
	"encoding/json"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/navi-crwn/pixelhop/internal/db"
)

// uploadAs posts one picture (key "" = a guest) and returns the answer's image JSON and the guest delete link.
func uploadAs(t *testing.T, h http.Handler, key string, png []byte) (id string, duplicate bool, deleteURL string) {
	t.Helper()
	var body bytes.Buffer
	mw := multipart.NewWriter(&body)
	fw, _ := mw.CreateFormFile("file", "same.png")
	fw.Write(png)
	mw.Close()
	req := httptest.NewRequest("POST", "/api/v1/upload", &body)
	req.Header.Set("Content-Type", mw.FormDataContentType())
	if key != "" {
		req.Header.Set("Authorization", "Bearer "+key)
	}
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	var r struct {
		Success bool
		Error   string
		Data    struct {
			Images []struct {
				ID        string
				Duplicate bool
				DeleteURL string `json:"delete_url"`
			}
		}
	}
	json.Unmarshal(rec.Body.Bytes(), &r)
	if !r.Success || len(r.Data.Images) != 1 {
		t.Fatalf("upload: %d %s", rec.Code, rec.Body.String())
	}
	im := r.Data.Images[0]
	return im.ID, im.Duplicate, im.DeleteURL
}

func keyOf(t *testing.T, a *App, id string) string {
	t.Helper()
	im, err := a.db.Image(t.Context(), id)
	if err != nil {
		t.Fatalf("image %s: %v", id, err)
	}
	return im.KeyFull
}

// Two members and a guest upload the same picture: three links, three owners, one stored file. Deleting
// them one by one keeps the file for whoever is left and removes it with the last.
func TestSameFileFromDifferentUploadersIsStoredOnce(t *testing.T) {
	a, h := ssoApp(t)
	ctx := t.Context()
	ua, ka := member(t, a, "share-a@example.com")
	ub, kb := member(t, a, "share-b@example.com")
	png := testPNG(64, 48)

	ida, dupA, _ := uploadAs(t, h, ka, png)
	idb, dupB, _ := uploadAs(t, h, kb, png)
	idg, dupG, delG := uploadAs(t, h, "", png)
	if dupA || dupB || dupG {
		t.Fatalf("an uploader was told the file was already here: duplicate=%v %v %v", dupA, dupB, dupG)
	}
	if ida == idb || idb == idg || ida == idg {
		t.Fatal("two uploaders got the same link")
	}
	key := keyOf(t, a, ida)
	if keyOf(t, a, idb) != key || keyOf(t, a, idg) != key {
		t.Fatalf("the same bytes were stored more than once: %q %q %q", key, keyOf(t, a, idb), keyOf(t, a, idg))
	}
	if n, err := a.db.CountKeyRefs(ctx, key); err != nil || n != 3 {
		t.Fatalf("rows pointing at the file: %d, want 3", n)
	}
	// Each row is its own: owner, delete token (guest), thumbnail key, and the quota counts the whole file for each member.
	imA, _ := a.db.Image(ctx, ida)
	imB, _ := a.db.Image(ctx, idb)
	imG, _ := a.db.Image(ctx, idg)
	if imA.UserID.Int64 != ua.ID || imB.UserID.Int64 != ub.ID || imG.UserID.Valid || imG.OwnerToken == "" || imA.OwnerToken != "" {
		t.Fatalf("owners: %+v %+v %+v", imA.UserID, imB.UserID, imG.UserID)
	}
	for _, uid := range []int64{ua.ID, ub.ID} {
		if used, n := a.db.UserStorage(ctx, uid); n != 1 || used != imA.Size {
			t.Fatalf("member %d storage: %d bytes in %d files, want the whole file once", uid, used, n)
		}
	}
	for _, id := range []string{ida, idb, idg} {
		if rec := fetch(h, "/i/"+id+".png"); rec.Code != 200 || !bytes.Equal(rec.Body.Bytes(), mustObject(t, a, key)) {
			t.Fatalf("/i/%s: %d", id, rec.Code)
		}
	}
	// The same member uploading it again still gets their own link back, as before.
	if again, dup, _ := uploadAs(t, h, ka, png); !dup || again != ida {
		t.Fatalf("same owner again: %q duplicate=%v, want %q true", again, dup, ida)
	}

	// Delete A's: the file stays for B and the guest.
	if rec, _ := call(t, h, "DELETE", "/api/v1/images/"+ida+"?permanent=1", ka, nil); rec.Code != 200 {
		t.Fatalf("delete A: %d", rec.Code)
	}
	if !hasObject(a, key) || fetch(h, "/i/"+idb+".png").Code != 200 || fetch(h, "/i/"+idg+".png").Code != 200 {
		t.Fatal("deleting one uploader's link took the file from the others")
	}
	// Delete B's: still there for the guest.
	if rec, _ := call(t, h, "DELETE", "/api/v1/images/"+idb+"?permanent=1", kb, nil); rec.Code != 200 {
		t.Fatalf("delete B: %d", rec.Code)
	}
	if !hasObject(a, key) || fetch(h, "/i/"+idg+".png").Code != 200 {
		t.Fatal("the guest's link broke when the second member deleted theirs")
	}
	// The guest's delete link removes the last row and then the file.
	path := delG[strings.Index(delG, "/delete/"):]
	if rec := fetch(h, path); rec.Code >= 500 {
		t.Fatalf("guest delete page: %d", rec.Code)
	}
	rec := httptest.NewRecorder()
	req := httptest.NewRequest("POST", path, strings.NewReader("confirm=1"))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Set("Sec-Fetch-Site", "same-origin")
	h.ServeHTTP(rec, req)
	if _, err := a.db.Image(ctx, idg); err == nil {
		t.Fatalf("the guest's row survived its delete link (%d)", rec.Code)
	}
	if hasObject(a, key) {
		t.Fatal("the file outlived its last row")
	}
}

func mustObject(t *testing.T, a *App, key string) []byte {
	t.Helper()
	obj, err := a.store.Get(t.Context(), key, "")
	if err != nil {
		t.Fatal(err)
	}
	defer obj.Body.Close()
	var b bytes.Buffer
	b.ReadFrom(obj.Body)
	return b.Bytes()
}

// With the setting off every upload keeps its own copy, as before; turning it off changes nothing already shared.
func TestShareStorageCanBeSwitchedOff(t *testing.T) {
	a, h := ssoApp(t)
	ctx := t.Context()
	_, ka := member(t, a, "off-a@example.com")
	_, kb := member(t, a, "off-b@example.com")

	// The control center sees it, on by default, and switches it off.
	rec, out := hopCall(t, h, "imperium", "GET", "/hop/settings", nil)
	if rec.Code != 200 || !strings.Contains(string(out.Data), `"key":"share_storage"`) || !strings.Contains(string(out.Data), `"default":true`) {
		t.Fatalf("setting not offered: %d", rec.Code)
	}
	if !a.shareStorageOn(ctx) {
		t.Fatal("sharing must be on by default")
	}
	shared1, _, _ := uploadAs(t, h, ka, testPNG(70, 50))
	shared2, _, _ := uploadAs(t, h, kb, testPNG(70, 50))
	if rec, o := hopCall(t, h, "imperium", "PUT", "/hop/settings", map[string]any{"share_storage": false}); rec.Code != 200 || !o.OK {
		t.Fatalf("put: %d %s", rec.Code, rec.Body.String())
	}
	if a.shareStorageOn(ctx) {
		t.Fatal("setting not stored")
	}
	off1, _, _ := uploadAs(t, h, ka, testPNG(71, 50))
	off2, _, _ := uploadAs(t, h, kb, testPNG(71, 50))
	if keyOf(t, a, off1) == keyOf(t, a, off2) {
		t.Fatal("sharing is off but two uploads share a file")
	}
	if keyOf(t, a, shared1) != keyOf(t, a, shared2) {
		t.Fatal("switching the setting off split files that were already shared")
	}
	if fetch(h, "/i/"+shared1+".png").Code != 200 || fetch(h, "/i/"+shared2+".png").Code != 200 {
		t.Fatal("files shared before still open")
	}
}

// Neither a trashed picture nor one in another storage backend is a source to share.
func TestOnlyVisiblePicturesInTheSameStorageAreShared(t *testing.T) {
	a, h := ssoApp(t)
	ctx := t.Context()
	_, ka := member(t, a, "only-a@example.com")
	_, kb := member(t, a, "only-b@example.com")
	_, kc := member(t, a, "only-c@example.com")
	png := testPNG(80, 60)
	first, _, _ := uploadAs(t, h, ka, png)

	if err := a.db.TrashImage(ctx, first); err != nil {
		t.Fatal(err)
	}
	second, _, _ := uploadAs(t, h, kb, png)
	if keyOf(t, a, first) == keyOf(t, a, second) {
		t.Fatal("a trashed picture was shared")
	}
	if err := a.db.RestoreImage(ctx, first); err != nil {
		t.Fatal(err)
	}
	if _, err := a.db.Exec(`UPDATE images SET backend='remote' WHERE id IN (?, ?)`, first, second); err != nil {
		t.Fatal(err)
	}
	third, _, _ := uploadAs(t, h, kc, png)
	if k := keyOf(t, a, third); k == keyOf(t, a, first) || k == keyOf(t, a, second) {
		t.Fatal("a file in another storage backend was shared")
	}
}

// The race between an upload that shares a file and the removal of the row that stores it.
// Both take shareMu, so one finishes before the other starts.
func TestUploadAndRemovalOfASharedFileAreOrdered(t *testing.T) {
	a, h := ssoApp(t)
	_, ka := member(t, a, "race-a@example.com")
	_, kb := member(t, a, "race-b@example.com")
	png := testPNG(90, 70)
	ida, _, _ := uploadAs(t, h, ka, png)

	// Holding the lock, neither an upload of the same bytes nor a removal gets anywhere.
	a.shareMu.Lock()
	done := make(chan string, 2)
	go func() { uploadAs(t, h, kb, png); done <- "upload" }()
	im, _ := a.db.Image(t.Context(), ida)
	go func() { a.removeImage(context.Background(), im); done <- "removal" }()
	select {
	case who := <-done:
		a.shareMu.Unlock()
		t.Fatalf("%s went through while shareMu was held", who)
	case <-time.After(150 * time.Millisecond):
	}
	a.shareMu.Unlock()
	<-done
	<-done
}

// Many uploads of the same bytes racing the removal of the picture that stores them: whatever row exists
// afterwards must have its file. Run with -race.
func TestSharedFileSurvivesRacingUploadsAndRemovals(t *testing.T) {
	a, h := ssoApp(t)
	ctx := t.Context()
	_, ka := member(t, a, "stress-a@example.com")
	users := make([]string, 6)
	for i := range users {
		_, users[i] = member(t, a, "stress-"+string(rune('b'+i))+"@example.com")
	}
	for round := 0; round < 12; round++ {
		png := testPNG(100+round, 60)
		ida, _, _ := uploadAs(t, h, ka, png)
		im, _ := a.db.Image(ctx, ida)
		var wg sync.WaitGroup
		ids := make([]string, len(users))
		for i := range users {
			wg.Add(1)
			go func() { defer wg.Done(); ids[i], _, _ = uploadAs(t, h, users[i], png) }()
		}
		wg.Add(1)
		go func() { defer wg.Done(); a.removeImage(ctx, im) }()
		wg.Wait()
		for _, id := range ids {
			row, err := a.db.Image(ctx, id)
			if err != nil {
				t.Fatalf("round %d: upload %s has no row", round, id)
			}
			if !hasObject(a, row.KeyFull) || !hasObject(a, row.KeyThumb) {
				t.Fatalf("round %d: %s points at a file that was deleted under it", round, id)
			}
		}
	}
}

// ---- shared files in the delete paths ----

// sharedObject stores one file's bytes and thumbnail and returns two rows (two owners) that point at it.
func sharedObject(t *testing.T, a *App, ownerA, ownerB *db.User, idA, idB string) (*db.Image, *db.Image) {
	t.Helper()
	ctx := context.Background()
	first := addImage(t, a, ownerA, idA)
	for _, k := range []string{first.KeyFull, first.KeyThumb} {
		if err := a.store.Put(ctx, k, bytes.NewReader([]byte("bytes of "+idA)), 10, "image/jpeg"); err != nil {
			t.Fatal(err)
		}
	}
	second := &db.Image{ID: idB, Title: "Pic " + idB, Mime: first.Mime, Ext: first.Ext, Size: first.Size, Width: 10, Height: 10,
		SHA256: first.SHA256, Backend: "local", KeyFull: first.KeyFull, KeyThumb: first.KeyThumb, IP: "203.0.113.9"}
	if ownerB != nil {
		second.UserID = sql.NullInt64{Int64: ownerB.ID, Valid: true}
	}
	if err := a.db.InsertImage(ctx, second); err != nil {
		t.Fatal(err)
	}
	return first, second
}

func TestPurgeKeepsAFileAnotherOwnerShares(t *testing.T) {
	a, _ := ssoApp(t)
	ctx := context.Background()
	ua, _ := member(t, a, "purge-share-a@example.com")
	ub, _ := member(t, a, "purge-share-b@example.com")
	first, second := sharedObject(t, a, ua, ub, "shared-1", "shared-2")
	if _, err := a.purgeUser(ctx, ua); err != nil {
		t.Fatal(err)
	}
	if !hasObject(a, first.KeyFull) || !hasObject(a, first.KeyThumb) {
		t.Fatal("deleting one account took the file the other account's link still opens")
	}
	if _, err := a.db.Image(ctx, second.ID); err != nil {
		t.Fatal("the other account's row went with it")
	}
	if _, err := a.purgeUser(ctx, ub); err != nil {
		t.Fatal(err)
	}
	if hasObject(a, first.KeyFull) || hasObject(a, first.KeyThumb) {
		t.Fatal("the file outlived the last account that used it")
	}
}

func TestDeleteFailureOfASharedFileLeavesTheRowToRetry(t *testing.T) {
	a, _ := ssoApp(t)
	ctx := context.Background()
	ua, _ := member(t, a, "fail-share-a@example.com")
	ub, _ := member(t, a, "fail-share-b@example.com")
	first, second := sharedObject(t, a, ua, ub, "fs-1", "fs-2")
	flaky := &failingDelete{Backend: a.store, fail: true}
	a.store, a.backends["local"] = flaky, flaky

	// Not the last reference: nothing in storage is touched, so storage being down does not matter.
	if err := a.removeImage(ctx, first); err != nil {
		t.Fatalf("removing one of two rows failed with storage down: %v", err)
	}
	if flaky.deleteCall != 0 {
		t.Fatalf("the shared file was touched (%d delete calls)", flaky.deleteCall)
	}
	// The last reference needs storage: it fails, and the row stays for the next try.
	if err := a.removeImage(ctx, second); err == nil {
		t.Fatal("last row removed although the file could not be deleted")
	}
	if _, err := a.db.Image(ctx, second.ID); err != nil {
		t.Fatal("the row was dropped but its file is still stored")
	}
	flaky.fail = false
	if err := a.removeImage(ctx, second); err != nil || hasObject(a, second.KeyFull) {
		t.Fatalf("retry: %v, object still there=%v", err, hasObject(a, second.KeyFull))
	}
}

func TestSweepKeepsAFileTheOtherOwnerStillHas(t *testing.T) {
	a, _ := ssoApp(t)
	ctx := context.Background()
	u, _ := member(t, a, "sweep-share@example.com")
	guestRow, memberRow := sharedObject(t, a, nil, u, "sw-guest", "sw-member")
	if _, err := a.db.ExecContext(ctx, `UPDATE images SET expires_at=1 WHERE id=?`, guestRow.ID); err != nil {
		t.Fatal(err)
	}
	a.sweep(ctx)
	if _, err := a.db.Image(ctx, guestRow.ID); err == nil {
		t.Fatal("the expired row survived the sweep")
	}
	if _, err := a.db.Image(ctx, memberRow.ID); err != nil || !hasObject(a, memberRow.KeyFull) {
		t.Fatal("the sweep took the file from the member's still-valid link")
	}
}

func TestEmptyingTheTrashKeepsASharedFile(t *testing.T) {
	a, h := ssoApp(t)
	ctx := context.Background()
	ua, _ := member(t, a, "trash-share-a@example.com")
	ub, _ := member(t, a, "trash-share-b@example.com")
	first, second := sharedObject(t, a, ua, ub, "ts-1", "ts-2")
	if err := a.db.TrashImage(ctx, first.ID); err != nil {
		t.Fatal(err)
	}
	if rec := post(h, "/dashboard/trash/empty", map[string][]string{}, sessionFor(t, a, ua.ID)); rec.Code != http.StatusSeeOther {
		t.Fatalf("empty trash: %d", rec.Code)
	}
	if _, err := a.db.Image(ctx, first.ID); err == nil {
		t.Fatal("trashed row survived")
	}
	if !hasObject(a, second.KeyFull) {
		t.Fatal("emptying one member's trash deleted the file another member still shares")
	}
}
