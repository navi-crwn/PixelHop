package app

import (
	"bytes"
	"context"
	"database/sql"
	"encoding/json"
	"image"
	"image/png"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strconv"
	"strings"
	"testing"

	"github.com/navi-crwn/pixelhop/internal/db"
)

func TestUploadClassificationAndDuplicates(t *testing.T) {
	for _, kind := range []string{"image", "pdf"} {
		t.Run(kind, func(t *testing.T) {
			a, h := ssoApp(t)
			u := linked(t, a, "upload-owner", "upload-owner@example.com")
			var data bytes.Buffer
			name := "sample.png"
			if kind == "image" {
				if err := png.Encode(&data, image.NewRGBA(image.Rect(0, 0, 24, 24))); err != nil {
					t.Fatal(err)
				}
			} else {
				name = "sample.pdf"
				data.WriteString("%PDF-1.4\n1 0 obj<</Type/Catalog>>endobj\n%%EOF")
			}
			ids := map[string]string{}
			for _, warning := range []string{"none", "sensitive", "adult", "none"} {
				var body bytes.Buffer
				form := multipart.NewWriter(&body)
				part, err := form.CreateFormFile("file", name)
				if err != nil {
					t.Fatal(err)
				}
				part.Write(data.Bytes())
				form.WriteField("content_warning", warning)
				form.WriteField("warning_reason", "Upload classification")
				form.Close()
				req := httptest.NewRequest("POST", a.cfg.BaseURL+"/api/v1/upload", &body)
				req.Header.Set("Content-Type", form.FormDataContentType())
				req.AddCookie(sessionFor(t, a, u.ID))
				res := httptest.NewRecorder()
				h.ServeHTTP(res, req)
				if res.Code != 200 {
					t.Fatalf("upload: %d %s", res.Code, res.Body.String())
				}
				var response struct {
					Data struct {
						ID string `json:"id"`
					} `json:"data"`
				}
				if err := json.Unmarshal(res.Body.Bytes(), &response); err != nil {
					t.Fatal(err)
				}
				im, err := a.db.Image(context.Background(), response.Data.ID)
				if err != nil || im.ContentWarning != warning || im.WarningReason != "Upload classification" {
					t.Fatalf("stored classification: %+v %v; %s", im, err, res.Body.String())
				}
				if previous := ids[warning]; previous != "" && previous != im.ID {
					t.Fatal("same classification did not deduplicate")
				}
				ids[warning] = im.ID
			}
			if ids["none"] == ids["adult"] || ids["none"] == ids["sensitive"] {
				t.Fatal("classification changed an existing shared upload")
			}
		})
	}
}

func TestChunkUploadPreservesClassification(t *testing.T) {
	a, h := ssoApp(t)
	var data bytes.Buffer
	if err := png.Encode(&data, image.NewRGBA(image.Rect(0, 0, 24, 24))); err != nil {
		t.Fatal(err)
	}
	call := func(method, path, contentType string, body *bytes.Buffer) *httptest.ResponseRecorder {
		req := httptest.NewRequest(method, a.cfg.BaseURL+path, body)
		req.Header.Set("Content-Type", contentType)
		res := httptest.NewRecorder()
		h.ServeHTTP(res, req)
		if res.Code != 200 {
			t.Fatalf("%s: %d %s", path, res.Code, res.Body.String())
		}
		return res
	}
	res := call("POST", "/api/v1/uploads", "application/json", bytes.NewBufferString(`{"name":"chunk.png","size":`+strconv.Itoa(data.Len())+`}`))
	var response struct {
		Data struct {
			ID string `json:"id"`
		} `json:"data"`
	}
	if err := json.Unmarshal(res.Body.Bytes(), &response); err != nil {
		t.Fatal(err)
	}
	base := "/api/v1/uploads/" + response.Data.ID
	call("PUT", base+"?offset=0", "application/octet-stream", &data)
	res = call("POST", base+"/finish", "application/x-www-form-urlencoded", bytes.NewBufferString("content_warning=adult&warning_reason=Chunk+warning"))
	if err := json.Unmarshal(res.Body.Bytes(), &response); err != nil {
		t.Fatal(err)
	}
	im, err := a.db.Image(context.Background(), response.Data.ID)
	if err != nil || im.ContentWarning != "adult" || im.WarningReason != "Chunk warning" {
		t.Fatalf("chunk classification: %+v %v", im, err)
	}
	req := httptest.NewRequest("GET", a.cfg.BaseURL+"/t/"+im.ID+".png", nil)
	blocked := httptest.NewRecorder()
	h.ServeHTTP(blocked, req)
	if blocked.Code != 303 {
		t.Fatalf("chunk thumbnail accessible before gate: %d", blocked.Code)
	}
}

func TestWarningMediaRequiresConfirmation(t *testing.T) {
	for _, albumWarning := range []bool{false, true} {
		t.Run(map[bool]string{false: "image", true: "album"}[albumWarning], func(t *testing.T) {
			a, h := ssoApp(t)
			ctx := context.Background()
			u := linked(t, a, "gate-owner", "gate-owner@example.com")
			im := addImage(t, a, u, "gate-media")
			gatePath, cookieName := "/gate/"+im.ID, gateCookieName(im.ID)
			if albumWarning {
				al := &db.Album{ID: "gate-album", Title: "Sensitive album", UserID: im.UserID, ContentWarning: "sensitive"}
				if err := a.db.InsertAlbum(ctx, al); err != nil {
					t.Fatal(err)
				}
				im.AlbumID = sql.NullString{String: al.ID, Valid: true}
				gatePath, cookieName = "/gate/a/"+al.ID, albumGateCookieName(al.ID)
			} else {
				im.ContentWarning = "adult"
			}
			if err := a.db.UpdateImageMeta(ctx, im); err != nil {
				t.Fatal(err)
			}
			var pngData bytes.Buffer
			if err := png.Encode(&pngData, image.NewRGBA(image.Rect(0, 0, 8, 8))); err != nil {
				t.Fatal(err)
			}
			for _, key := range []string{im.KeyFull, im.KeyThumb} {
				if err := a.backends[im.Backend].Put(ctx, key, bytes.NewReader(pngData.Bytes()), int64(pngData.Len()), "image/png"); err != nil {
					t.Fatal(err)
				}
			}
			paths := []string{"/i/gate-media.jpg", "/t/gate-media.jpg", "/i/gate-media.jpg?w=16", "/i/2026/10/31/gate-media_original.jpg", "/i/2026/10/31/gate-media_thumb.jpg"}
			for _, path := range paths {
				for _, referer := range []string{"", a.cfg.BaseURL + "/gate-media", "https://outside.example"} {
					req := httptest.NewRequest("GET", a.cfg.BaseURL+path, nil)
					req.Header.Set("Referer", referer)
					w := httptest.NewRecorder()
					h.ServeHTTP(w, req)
					if w.Code != 303 || !strings.Contains(w.Header().Get("Cache-Control"), "no-store") {
						t.Fatalf("unguarded %s (%s): %d", path, referer, w.Code)
					}
				}
			}
			for _, remember := range []bool{false, true} {
				form := url.Values{}
				if remember {
					form.Set("remember", "1")
				}
				req := httptest.NewRequest("POST", a.cfg.BaseURL+gatePath, strings.NewReader(form.Encode()))
				req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
				w := httptest.NewRecorder()
				h.ServeHTTP(w, req)
				var cookie *http.Cookie
				for _, c := range w.Result().Cookies() {
					if c.Name == cookieName {
						cookie = c
					}
				}
				age := 0
				if remember {
					age = 86400
				}
				if cookie == nil || cookie.MaxAge != age || cookie.Domain != "" || !cookie.HttpOnly || !cookie.Secure || cookie.SameSite != http.SameSiteLaxMode {
					t.Fatalf("cookie: %+v", cookie)
				}
				for _, path := range paths {
					r := httptest.NewRequest("GET", a.cfg.BaseURL+path, nil)
					r.AddCookie(cookie)
					res := httptest.NewRecorder()
					h.ServeHTTP(res, r)
					if res.Code != 200 || res.Body.Len() == 0 || !strings.Contains(res.Header().Get("Cache-Control"), "no-store") {
						t.Errorf("confirmed media %s: %d %s", path, res.Code, res.Body.String())
					}
				}
			}
			ownerRequest := httptest.NewRequest("GET", a.cfg.BaseURL+paths[0], nil)
			ownerRequest.AddCookie(sessionFor(t, a, u.ID))
			ownerResponse := httptest.NewRecorder()
			h.ServeHTTP(ownerResponse, ownerRequest)
			if ownerResponse.Code != 200 {
				t.Fatalf("owner media: %d", ownerResponse.Code)
			}
		})
	}
}

func TestContentWarningGateImage(t *testing.T) {
	a, h := ssoApp(t)

	im := &db.Image{
		ID:             "nsfw123",
		Title:          "Adult Media",
		Mime:           "image/jpeg",
		Ext:            "jpg",
		Size:           1024,
		Width:          800,
		Height:         600,
		SHA256:         "nsfw123hash",
		Backend:        "local",
		KeyFull:        "nsfw123.jpg",
		KeyThumb:       "nsfw123_t.jpg",
		IP:             "192.0.2.1",
		ContentWarning: "adult",
		WarningReason:  "Contains adult themes",
		Listed:         true,
	}
	if err := a.db.InsertImage(context.Background(), im); err != nil {
		t.Fatal(err)
	}

	// 1. Visit view page without cookie -> gate is present
	req := httptest.NewRequest("GET", "/nsfw123", nil)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rec.Code)
	}
	body := rec.Body.String()
	if !strings.Contains(body, "Age Verification Required") {
		t.Fatalf("expected gate badge in body")
	}
	if !strings.Contains(body, "Contains adult themes") {
		t.Fatalf("expected creator warning note in body")
	}

	// 2. Direct hotlink with external referer redirects to view page
	reqImg := httptest.NewRequest("GET", "/i/nsfw123.jpg", nil)
	reqImg.Header.Set("Referer", "https://external.example.com/blog")
	recImg := httptest.NewRecorder()
	h.ServeHTTP(recImg, reqImg)
	if recImg.Code != http.StatusSeeOther {
		t.Fatalf("expected 303 redirect for external referer, got %d", recImg.Code)
	}
	if loc := recImg.Header().Get("Location"); !strings.Contains(loc, "/nsfw123") {
		t.Fatalf("expected redirect to /nsfw123, got %q", loc)
	}

	// 3. Confirm gate via POST /gate/nsfw123
	for _, path := range []string{"/i/nsfw123.jpg", "/t/nsfw123.jpg", "/i/nsfw123.jpg?w=200"} {
		for _, ref := range []string{"", a.cfg.BaseURL + "/nsfw123", "https://external.example.com/"} {
			r := httptest.NewRequest("GET", path, nil)
			r.Header.Set("Referer", ref)
			w := httptest.NewRecorder()
			h.ServeHTTP(w, r)
			if w.Code != http.StatusSeeOther {
				t.Errorf("unguarded media %s referer=%q: status %d", path, ref, w.Code)
			}
		}
	}
	form := url.Values{"remember": {"1"}}
	reqGate := httptest.NewRequest("POST", "/gate/nsfw123", strings.NewReader(form.Encode()))
	reqGate.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	recGate := httptest.NewRecorder()
	h.ServeHTTP(recGate, reqGate)
	if recGate.Code != http.StatusSeeOther {
		t.Fatalf("expected 303 redirect after gate confirmation, got %d", recGate.Code)
	}
	cookie := recGate.Result().Cookies()
	var gateCookie *http.Cookie
	for _, c := range cookie {
		if c.Name == "hop_gate_ph_nsfw123" {
			gateCookie = c
			break
		}
	}
	if gateCookie == nil || gateCookie.Value != "1" {
		t.Fatalf("expected hop_gate_ph_nsfw123 cookie to be set")
	}

	// 4. Visit view page with cookie -> gate overlay not shown
	reqPassed := httptest.NewRequest("GET", "/nsfw123", nil)
	reqPassed.AddCookie(gateCookie)
	recPassed := httptest.NewRecorder()
	h.ServeHTTP(recPassed, reqPassed)
	if recPassed.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", recPassed.Code)
	}
	bodyPassed := recPassed.Body.String()
	if strings.Contains(bodyPassed, "Age Verification Required") {
		t.Fatalf("expected gate overlay to not appear after cookie verification")
	}

	// 5. Explore page excludes adult content
	reqExplore := httptest.NewRequest("GET", "/explore", nil)
	recExplore := httptest.NewRecorder()
	h.ServeHTTP(recExplore, reqExplore)
	if strings.Contains(recExplore.Body.String(), "/nsfw123") {
		t.Fatalf("adult content should not be listed on explore")
	}
}

func TestContentWarningModerationActions(t *testing.T) {
	a, h := ssoApp(t)

	im := &db.Image{
		ID:             "modpic",
		Title:          "Mod Pic",
		Mime:           "image/jpeg",
		Ext:            "jpg",
		Size:           100,
		Width:          10,
		Height:         10,
		SHA256:         "modpicsha",
		Backend:        "local",
		KeyFull:        "modpic.jpg",
		KeyThumb:       "modpic_t.jpg",
		IP:             "127.0.0.1",
		ContentWarning: "none",
	}
	if err := a.db.InsertImage(context.Background(), im); err != nil {
		t.Fatal(err)
	}

	// Mark NSFW
	res := hopPost(t, h, "/hop/content/image/modpic/actions", map[string]any{
		"action": "mark_nsfw",
		"reason": "Explicit nudity",
	})
	if !res.OK {
		t.Fatalf("mark_nsfw failed: %s", res.Message)
	}

	updated, err := a.db.Image(context.Background(), "modpic")
	if err != nil {
		t.Fatal(err)
	}
	if updated.ContentWarning != "adult" || updated.WarningReason != "Explicit nudity" {
		t.Fatalf("expected adult warning, got %q / %q", updated.ContentWarning, updated.WarningReason)
	}

	// Mark Sensitive
	res = hopPost(t, h, "/hop/content/image/modpic/actions", map[string]any{
		"action": "mark_sensitive",
		"reason": "Contains spoilers",
	})
	if !res.OK {
		t.Fatalf("mark_sensitive failed: %s", res.Message)
	}

	updated, err = a.db.Image(context.Background(), "modpic")
	if err != nil {
		t.Fatal(err)
	}
	if updated.ContentWarning != "sensitive" || updated.WarningReason != "Contains spoilers" {
		t.Fatalf("expected sensitive warning, got %q / %q", updated.ContentWarning, updated.WarningReason)
	}

	// Unmark NSFW
	res = hopPost(t, h, "/hop/content/image/modpic/actions", map[string]any{
		"action": "unmark_nsfw",
	})
	if !res.OK {
		t.Fatalf("unmark_nsfw failed: %s", res.Message)
	}

	updated, err = a.db.Image(context.Background(), "modpic")
	if err != nil {
		t.Fatal(err)
	}
	if updated.ContentWarning != "none" {
		t.Fatalf("expected none warning, got %q", updated.ContentWarning)
	}
}
