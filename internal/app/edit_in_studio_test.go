package app

import (
	"bytes"
	"database/sql"
	"io/fs"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/navi-crwn/pixelhop/internal/db"
	"github.com/navi-crwn/pixelhop/web"
)

const editHref = `href="/tools?src=%s"`

func editLink(id string) string { return strings.Replace(editHref, "%s", id, 1) }

// Pictures get an "Edit in studio" button on the result page, for guests and members alike; videos and
// PDFs, which the studio cannot open, do not.
func TestResultPageOffersEditInStudioForPicturesOnly(t *testing.T) {
	a, h := ssoApp(t)
	u, key := member(t, a, "res-edit@example.com")
	mine := upload(t, h, key, 1)[0]

	// A guest's upload: the same page, no account.
	var body bytes.Buffer
	mw := multipart.NewWriter(&body)
	fw, _ := mw.CreateFormFile("file", "guest.png")
	fw.Write(testPNG(50, 40))
	mw.Close()
	req := httptest.NewRequest("POST", "/api/v1/upload", &body)
	req.Header.Set("Content-Type", mw.FormDataContentType())
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK && rec.Code != http.StatusCreated {
		t.Skipf("guest uploads are off here: %d", rec.Code)
	}

	pdf := addImage(t, a, u, "doc1")
	pdf.Mime, pdf.Ext = "application/pdf", "pdf"
	if _, err := a.db.Exec(`UPDATE images SET mime=?, ext=? WHERE id=?`, pdf.Mime, pdf.Ext, pdf.ID); err != nil {
		t.Fatal(err)
	}
	vid := addImage(t, a, u, "clip1")
	if _, err := a.db.Exec(`UPDATE images SET mime='video/mp4', ext='mp4' WHERE id=?`, vid.ID); err != nil {
		t.Fatal(err)
	}

	page := fetch(h, "/uploaded?id="+strings.Join([]string{mine, "doc1", "clip1"}, ",")).Body.String()
	if !strings.Contains(page, editLink(mine)) {
		t.Errorf("a picture has no Edit in studio button")
	}
	for _, id := range []string{"doc1", "clip1"} {
		if strings.Contains(page, editLink(id)) {
			t.Errorf("%s (not a picture) offers Edit in studio", id)
		}
	}
	if !strings.Contains(page, "Edit in studio") {
		t.Error("the button has no label")
	}

	// The page a share sheet or a form post answers with is the same template.
	var b2 bytes.Buffer
	m2 := multipart.NewWriter(&b2)
	f2, _ := m2.CreateFormFile("file", "shared.png")
	f2.Write(testPNG(44, 33))
	m2.Close()
	rq := httptest.NewRequest("POST", "/share", &b2)
	rq.Header.Set("Content-Type", m2.FormDataContentType())
	rq.Header.Set("Sec-Fetch-Site", "same-origin")
	rq.Header.Set("Authorization", "Bearer "+key)
	rc := httptest.NewRecorder()
	h.ServeHTTP(rc, rq)
	if rc.Code != http.StatusOK || !strings.Contains(rc.Body.String(), `href="/tools?src=`) {
		t.Errorf("/share result: %d, button present=%v", rc.Code, strings.Contains(rc.Body.String(), `href="/tools?src=`))
	}
}

// The member's own file tile carries the same button, for pictures only.
func TestDashboardTileOffersEditInStudioForPicturesOnly(t *testing.T) {
	a, h := ssoApp(t)
	u := linked(t, a, "71", "tiles@example.com")
	addImage(t, a, u, "pic1")
	addImage(t, a, u, "doc2")
	a.db.Exec(`UPDATE images SET mime='application/pdf', ext='pdf' WHERE id='doc2'`)
	c := sessionFor(t, a, u.ID)
	page := fetch(h, "/dashboard", c).Body.String()
	if !strings.Contains(page, `class="tile-edit" `+editLink("pic1")) {
		t.Error("the picture tile has no edit button")
	}
	if strings.Contains(page, editLink("doc2")) {
		t.Error("the PDF tile offers the studio")
	}
}

// The studio asks for the file with ?download: on a server that keeps files in S3 with a public address a
// plain request would be answered with a redirect to that other host, which the studio page's content
// security policy does not let it read.
func TestStudioLoadsTheFileWithoutARedirect(t *testing.T) {
	a := s3App(t)
	h := a.Handler()
	u := linked(t, a, "72", "studio-s3@example.com")
	im := &db.Image{ID: "s3pic", UserID: sql.NullInt64{Int64: u.ID, Valid: true}, Mime: "image/jpeg", Ext: "jpg", Backend: "s3", KeyFull: "s3pic.jpg", KeyThumb: "s3pic_t.jpg"}
	if err := a.db.InsertImage(t.Context(), im); err != nil {
		t.Fatal(err)
	}
	if rec := fetch(h, "/i/s3pic.img"); rec.Code != http.StatusFound {
		t.Fatalf("premise: a public S3 file is redirected, got %d", rec.Code)
	}
	if rec := fetch(h, "/i/s3pic.img?download"); rec.Code != http.StatusOK || rec.Body.String() != "synthetic" {
		t.Fatalf("?download: %d %q, want the bytes from the app", rec.Code, rec.Body.String())
	}
	js, err := fs.ReadFile(web.Static, "static/js/studio.js")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(js), "'/i/' + src + '.img?download'") {
		t.Error("studio.js does not ask for ?download")
	}
}
