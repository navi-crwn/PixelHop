package app

import (
	"context"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"testing"

	"github.com/navi-crwn/pixelhop/internal/db"
)

// A purge that fails because storage is down must not use up the event id.
func TestHopEventProcessingFailureDoesNotConsumeID(t *testing.T) {
	a, h := ssoApp(t)
	u := linked(t, a, "retry-purge-user", "retry-purge@example.com")
	im := addImage(t, a, u, "retry-purge-image")
	flaky := &failingDelete{Backend: a.store, fail: true}
	a.store = flaky
	a.backends["local"] = flaky
	body := map[string]any{"id": "event-retry-purge", "type": "account.deleted", "data": map[string]any{"hop_id": u.HelinkID.String}}
	rec, _ := hopCall(t, h, "helink", "POST", "/hop/events", body)
	if rec.Code != 500 {
		t.Fatalf("failed purge status=%d body=%s", rec.Code, rec.Body.String())
	}
	flaky.fail = false
	hopCall(t, h, "helink", "POST", "/hop/events", body)
	if _, err := a.db.UserByID(context.Background(), u.ID); err == nil {
		t.Fatal("same-id retry did not purge account")
	}
	if _, err := a.db.Image(context.Background(), im.ID); err == nil {
		t.Fatal("same-id retry did not purge image")
	}
}

func TestTrashEmptyFailureStopsAndRetries(t *testing.T) {
	a, h := ssoApp(t)
	ctx := context.Background()
	u := linked(t, a, "trash-retry-user", "trash-retry@example.com")
	first := addImage(t, a, u, "trash-retry-first")
	second := addImage(t, a, u, "trash-retry-second")
	for _, im := range []*db.Image{first, second} {
		if err := a.db.TrashImage(ctx, im.ID); err != nil {
			t.Fatal(err)
		}
	}
	flaky := &failingDelete{Backend: a.store, fail: true}
	a.store, a.backends["local"] = flaky, flaky

	rec := post(h, "/dashboard/trash/empty", url.Values{}, sessionFor(t, a, u.ID))
	if rec.Code != http.StatusInternalServerError || rec.Header().Get("Location") != "" {
		t.Fatalf("failed trash empty: status=%d Location=%q", rec.Code, rec.Header().Get("Location"))
	}
	if flaky.deleteCall != 1 {
		t.Fatalf("trash empty did not stop at first failure: %d delete calls", flaky.deleteCall)
	}
	for _, im := range []*db.Image{first, second} {
		if _, err := a.db.Image(ctx, im.ID); err != nil {
			t.Fatalf("image metadata lost after failed trash empty: %v", err)
		}
	}
	flaky.fail = false
	if rec := post(h, "/dashboard/trash/empty", url.Values{}, sessionFor(t, a, u.ID)); rec.Code != http.StatusSeeOther {
		t.Fatalf("trash empty retry: %d %s", rec.Code, rec.Body.String())
	}
	for _, im := range []*db.Image{first, second} {
		if _, err := a.db.Image(ctx, im.ID); err == nil {
			t.Fatalf("image %s survived successful retry", im.ID)
		}
	}
}

func TestHopReportDeleteFailureRetainsOpenReportAndRetries(t *testing.T) {
	a, h := ssoApp(t)
	ctx := context.Background()
	u := linked(t, a, "report-retry-user", "report-retry@example.com")
	im := addImage(t, a, u, "report-retry-image")
	rp := &db.Report{ImageID: im.ID, Reason: "spam", IP: "203.0.113.10"}
	if err := a.db.InsertReport(ctx, rp); err != nil {
		t.Fatal(err)
	}
	if err := a.db.QueryRowContext(ctx, `SELECT id FROM reports WHERE image_id=?`, im.ID).Scan(&rp.ID); err != nil {
		t.Fatal(err)
	}
	flaky := &failingDelete{Backend: a.store, fail: true}
	a.store, a.backends["local"] = flaky, flaky
	path := "/hop/reports/" + strconv.FormatInt(rp.ID, 10) + "/actions"
	body := map[string]any{"action": "delete", "reason": "synthetic"}

	rec, out := hopCall(t, h, "imperium", "POST", path, body)
	if rec.Code != http.StatusInternalServerError || out.OK {
		t.Fatalf("failed report delete: status=%d body=%s", rec.Code, rec.Body.String())
	}
	got, err := a.db.Report(ctx, rp.ID)
	if err != nil || got.Status != "open" {
		t.Fatalf("report not retained open: got=%+v err=%v", got, err)
	}
	if _, err := a.db.Image(ctx, im.ID); err != nil {
		t.Fatalf("image metadata lost after failed report delete: %v", err)
	}
	audits, _, err := a.db.Audit(ctx, 0, 100, 0)
	if err != nil {
		t.Fatal(err)
	}
	for _, entry := range audits {
		if entry.Action == "report.delete" && entry.Target == strconv.FormatInt(rp.ID, 10) {
			t.Fatal("failed report delete recorded a success audit")
		}
	}
	flaky.fail = false
	hopPost(t, h, path, body)
	if _, err := a.db.Image(ctx, im.ID); err == nil {
		t.Fatal("report delete retry kept the image")
	}
	if got, err := a.db.Report(ctx, rp.ID); err == nil {
		t.Fatalf("successful deletion should cascade report rows, got=%+v", got)
	}
	audits, _, err = a.db.Audit(ctx, 0, 100, 0)
	if err != nil || len(audits) != 1 || audits[0].Action != "report.delete" {
		t.Fatalf("successful retry should add one audit: rows=%+v err=%v", audits, err)
	}
}

func TestPurgeUserStopsWhenImageStorageDeleteFails(t *testing.T) {
	a, _ := ssoApp(t)
	u := linked(t, a, "purge-storage-user", "purge-storage@example.com")
	im := addImage(t, a, u, "purge-image")
	if err := a.store.Put(context.Background(), im.KeyFull, strings.NewReader("synthetic"), 9, "image/jpeg"); err != nil {
		t.Fatal(err)
	}
	flaky := &failingDelete{Backend: a.store, fail: true}
	a.store = flaky
	a.backends["local"] = flaky
	for attempt := 0; attempt < 2; attempt++ {
		if _, err := a.purgeUser(context.Background(), u); err == nil {
			t.Fatal("image storage failure was swallowed")
		}
		if _, err := a.db.UserByID(context.Background(), u.ID); err != nil {
			t.Fatalf("account metadata was deleted despite failed image cleanup: %v", err)
		}
		if _, err := a.db.Image(context.Background(), im.ID); err != nil {
			t.Fatalf("image key metadata was deleted despite failed cleanup: %v", err)
		}
	}
	flaky.fail = false
	if _, err := a.purgeUser(context.Background(), u); err != nil {
		t.Fatalf("retry after storage recovery: %v", err)
	}
	if hasObject(a, im.KeyFull) {
		t.Fatal("image object remained after successful retry")
	}
}
