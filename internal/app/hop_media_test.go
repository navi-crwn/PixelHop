package app

import (
	"bufio"
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/navi-crwn/pixelhop/internal/hop"
)

// POST /hop/media: a store client (FileHop) saves a file in a member's library.

const storeSecret = "a-store-secret-for-filehop-only-1234567890"

// storeApp is a server where filehop may store files; it holds its own secret, nobody else's.
func storeApp(t *testing.T) (*App, http.Handler) {
	t.Helper()
	t.Setenv("PIXELHOP_STORE_CLIENTS", "filehop")
	t.Setenv("HOP_STORE_SECRET_FILEHOP", storeSecret)
	return ssoApp(t)
}

type storeAnswer struct {
	OK      bool   `json:"ok"`
	Error   string `json:"error"`
	Message string `json:"message"`
	Data    struct {
		ID        string `json:"id"`
		URL       string `json:"url"`
		PageURL   string `json:"page_url"`
		ThumbURL  string `json:"thumb_url"`
		Mime      string `json:"mime"`
		Bytes     int64  `json:"bytes"`
		Width     int    `json:"width"`
		Height    int    `json:"height"`
		Kind      string `json:"kind"`
		Duplicate bool   `json:"duplicate"`
	} `json:"data"`
}

// storeCall signs and sends the file as `app` with `secret` for `actor`.
func storeCall(t *testing.T, h http.Handler, app, secret, actor, name string, body []byte) (*httptest.ResponseRecorder, storeAnswer) {
	t.Helper()
	path := "/hop/media"
	if name != "" {
		path += "?name=" + name
	}
	req := httptest.NewRequest("POST", path, bytes.NewReader(body))
	for k, v := range hop.Headers(app, secret, actor, "POST", path, body) {
		req.Header[k] = v
	}
	req.Header.Set("Content-Type", "application/octet-stream")
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	var out storeAnswer
	json.Unmarshal(rec.Body.Bytes(), &out)
	return rec, out
}

func TestHopMediaStoresInTheActorsLibrary(t *testing.T) {
	a, h := storeApp(t)
	ctx := context.Background()
	me := linked(t, a, "201", "me@example.com")
	other := linked(t, a, "202", "other@example.com")

	rec, out := storeCall(t, h, "filehop", storeSecret, "201", "converted.png", testPNG(40, 30))
	if rec.Code != http.StatusCreated || !out.OK || out.Data.ID == "" {
		t.Fatalf("store: %d %s", rec.Code, rec.Body.String())
	}
	d := out.Data
	if d.Mime != "image/png" || d.Width != 40 || d.Height != 30 || d.Kind != "image" || d.Duplicate || d.Bytes == 0 {
		t.Errorf("answer: %+v", d)
	}
	if d.URL != "https://p.example.com/i/"+d.ID+".png" || d.PageURL != "https://p.example.com/"+d.ID || !strings.Contains(d.ThumbURL, "/t/"+d.ID+".") {
		t.Errorf("addresses: %s %s %s", d.URL, d.PageURL, d.ThumbURL)
	}
	// It is in the actor's library, counts against their quota, and nobody else's.
	im, err := a.db.Image(ctx, d.ID)
	if err != nil || !im.UserID.Valid || im.UserID.Int64 != me.ID || im.OwnerToken != "" || im.Title != "converted" {
		t.Fatalf("stored image: %+v %v", im, err)
	}
	if used, n := a.db.UserStorage(ctx, me.ID); n != 1 || used != d.Bytes {
		t.Errorf("library: %d files, %d bytes", n, used)
	}
	if _, n := a.db.UserStorage(ctx, other.ID); n != 0 {
		t.Errorf("another member's library got %d files", n)
	}
	// The public address works, and a browser on a cross-origin-isolated page may embed it.
	pub := fetch(h, "/i/"+d.ID+".png")
	if pub.Code != 200 || pub.Header().Get("Cross-Origin-Resource-Policy") != "cross-origin" {
		t.Errorf("public file: %d CORP %q", pub.Code, pub.Header().Get("Cross-Origin-Resource-Policy"))
	}
	// The same file again is not stored twice.
	rec, again := storeCall(t, h, "filehop", storeSecret, "201", "converted.png", testPNG(40, 30))
	if rec.Code != 200 || !again.Data.Duplicate || again.Data.ID != d.ID {
		t.Errorf("same file twice: %d %s", rec.Code, rec.Body.String())
	}
	// It is recorded in the audit trail.
	if n := a.db.QueryRow(`SELECT COUNT(*) FROM hop_audit WHERE action='media.store' AND app='filehop' AND actor='201'`); n == nil {
		t.Error("no audit row")
	} else {
		var c int
		n.Scan(&c)
		if c != 1 {
			t.Errorf("audit rows: %d", c)
		}
	}
}

func TestHopMediaRefusesUnsignedAndWrongCallers(t *testing.T) {
	a, h := storeApp(t)
	linked(t, a, "201", "me@example.com")
	png := testPNG(20, 20)

	// Unsigned: no headers at all, and headers without a signature.
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequest("POST", "/hop/media", bytes.NewReader(png)))
	if rec.Code != http.StatusUnauthorized {
		t.Errorf("unsigned: %d", rec.Code)
	}
	for name, c := range map[string]struct{ app, secret string }{
		"wrong secret":                {"filehop", "x-not-the-secret-but-long-enough-0123456789"},
		"the shared SSO secret":       {"filehop", testSecret},
		"an app that is not a client": {"biohop", storeSecret},
		"an admin client":             {"imperium", testSecret},
		"hub":                         {"helink", testSecret},
	} {
		rec, out := storeCall(t, h, c.app, c.secret, "201", "a.png", png)
		if rec.Code != http.StatusUnauthorized || out.Error != "bad_signature" {
			t.Errorf("%s: %d %s", name, rec.Code, rec.Body.String())
		}
	}
	// A call signed as filehop for one body cannot carry another, nor a different path.
	path := "/hop/media?name=a.png"
	req := httptest.NewRequest("POST", path, bytes.NewReader(testPNG(21, 21)))
	for k, v := range hop.Headers("filehop", storeSecret, "201", "POST", path, png) {
		req.Header[k] = v
	}
	rec = httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	if rec.Code != http.StatusUnauthorized {
		t.Errorf("swapped body: %d", rec.Code)
	}
	// Replaying a call fails.
	req = httptest.NewRequest("POST", path, bytes.NewReader(png))
	for k, v := range hop.Headers("filehop", storeSecret, "201", "POST", path, png) {
		req.Header[k] = v
	}
	first := httptest.NewRecorder()
	h.ServeHTTP(first, req)
	req2 := httptest.NewRequest("POST", path, bytes.NewReader(png))
	req2.Header = req.Header.Clone()
	second := httptest.NewRecorder()
	h.ServeHTTP(second, req2)
	var out storeAnswer
	json.Unmarshal(second.Body.Bytes(), &out)
	if first.Code != http.StatusCreated || second.Code != http.StatusUnauthorized || out.Error != "replay" {
		t.Errorf("replay: %d then %d %s", first.Code, second.Code, second.Body.String())
	}
	// Nothing was stored by the refused calls.
	if _, n := a.db.UserStorage(context.Background(), 1); n != 1 {
		t.Errorf("the library holds %d files, want only the one from the first replayed call", n)
	}
}

func TestHopMediaIsOffUntilAClientIsConfigured(t *testing.T) {
	a, h := ssoApp(t) // no PIXELHOP_STORE_CLIENTS
	linked(t, a, "201", "me@example.com")
	rec, out := storeCall(t, h, "filehop", storeSecret, "201", "a.png", testPNG(10, 10))
	if rec.Code != http.StatusNotFound || out.Error != "not_configured" {
		t.Fatalf("%d %s", rec.Code, rec.Body.String())
	}
	// A client that is listed but has no secret of its own is not a client.
	t.Setenv("PIXELHOP_STORE_CLIENTS", "filehop")
	_, h2 := ssoApp(t)
	if rec, _ := storeCall(t, h2, "filehop", testSecret, "201", "a.png", testPNG(10, 10)); rec.Code != http.StatusNotFound {
		t.Errorf("listed without a secret: %d", rec.Code)
	}
}

func TestHopMediaWrongUser(t *testing.T) {
	a, h := storeApp(t)
	ctx := context.Background()
	linked(t, a, "201", "me@example.com")
	png := testPNG(20, 20)

	if rec, out := storeCall(t, h, "filehop", storeSecret, "999", "a.png", png); rec.Code != http.StatusNotFound || out.Error != "no_account" {
		t.Errorf("unknown HOP ID: %d %s", rec.Code, rec.Body.String())
	}
	for _, actor := range []string{"", "system"} {
		if rec, out := storeCall(t, h, "filehop", storeSecret, actor, "a.png", png); rec.Code != http.StatusBadRequest || out.Error != "no_actor" {
			t.Errorf("actor %q: %d %s", actor, rec.Code, rec.Body.String())
		}
	}
	u, _ := a.db.UserByHelink(ctx, "201")
	u.Status = "suspended"
	a.db.UpdateUser(ctx, u)
	if rec, out := storeCall(t, h, "filehop", storeSecret, "201", "a.png", png); rec.Code != http.StatusForbidden || out.Error != "account_suspended" {
		t.Errorf("suspended: %d %s", rec.Code, rec.Body.String())
	}
	// A member who exists here only by e-mail, with no HOP ID, cannot be reached by guessing an id.
	member(t, a, "plain@example.com")
	if rec, _ := storeCall(t, h, "filehop", storeSecret, "plain@example.com", "a.png", png); rec.Code != http.StatusNotFound {
		t.Errorf("an e-mail is not an identity: %d", rec.Code)
	}
	if n := countImages(t, a); n != 0 {
		t.Errorf("%d images were stored for refused callers", n)
	}
}

func countImages(t *testing.T, a *App) int {
	t.Helper()
	var n int
	if err := a.db.QueryRow(`SELECT COUNT(*) FROM images`).Scan(&n); err != nil {
		t.Fatal(err)
	}
	return n
}

func TestHopMediaLimitsAndTypes(t *testing.T) {
	a, h := storeApp(t)
	ctx := context.Background()
	linked(t, a, "201", "me@example.com")

	// Over the size limit of a normal upload: refused before anything is stored.
	a.cfg.MaxUploadBytes = 1 << 10
	big := bytes.Repeat([]byte{0x89}, 2<<10)
	if rec, out := storeCall(t, h, "filehop", storeSecret, "201", "big.png", big); rec.Code != http.StatusRequestEntityTooLarge || out.Error != "too_large" {
		t.Errorf("too big: %d %s", rec.Code, rec.Body.String())
	}
	a.cfg.MaxUploadBytes = 20 << 20

	// Types: the ones a normal upload refuses are refused here, whatever the file is called.
	for name, body := range map[string][]byte{
		"notes.png":   []byte("this is plain text, not a picture"),
		"page.html":   []byte("<html><script>alert(1)</script></html>"),
		"vector.svg":  []byte(`<svg xmlns="http://www.w3.org/2000/svg"><script>alert(1)</script></svg>`),
		"broken.png":  testPNG(30, 30)[:40],
		"program.exe": append([]byte("MZ"), bytes.Repeat([]byte{0}, 64)...),
	} {
		rec, out := storeCall(t, h, "filehop", storeSecret, "201", name, body)
		if rec.Code != http.StatusUnsupportedMediaType || out.Error != "unsupported_type" {
			t.Errorf("%s: %d %s", name, rec.Code, rec.Body.String())
		}
	}
	if rec, out := storeCall(t, h, "filehop", storeSecret, "201", "empty.png", nil); rec.Code != http.StatusBadRequest || out.Error != "empty_body" {
		t.Errorf("empty: %d %s", rec.Code, rec.Body.String())
	}
	// A file taken down before is refused as on a normal upload.
	png := testPNG(33, 33)
	a.db.BlockHash(ctx, sha256hex(png), "test")
	if rec, out := storeCall(t, h, "filehop", storeSecret, "201", "gone.png", png); rec.Code != http.StatusForbidden || out.Error != "blocked_file" {
		t.Errorf("blocked file: %d %s", rec.Code, rec.Body.String())
	}
	if n := countImages(t, a); n != 0 {
		t.Errorf("%d images were stored from refused files", n)
	}
}

func TestHopMediaQuota(t *testing.T) {
	a, h := storeApp(t)
	ctx := context.Background()
	u := linked(t, a, "201", "me@example.com")
	png := testPNG(50, 50)
	u.QuotaBytes = int64(len(png)) + 10 // room for one
	a.db.UpdateUser(ctx, u)

	if rec, _ := storeCall(t, h, "filehop", storeSecret, "201", "one.png", png); rec.Code != http.StatusCreated {
		t.Fatalf("first: %d %s", rec.Code, rec.Body.String())
	}
	rec, out := storeCall(t, h, "filehop", storeSecret, "201", "two.png", testPNG(51, 51))
	if rec.Code != http.StatusRequestEntityTooLarge || out.Error != "quota_exceeded" {
		t.Fatalf("second: %d %s", rec.Code, rec.Body.String())
	}
	if _, n := a.db.UserStorage(ctx, u.ID); n != 1 {
		t.Errorf("library holds %d files after a refused one", n)
	}
}

func TestHopMediaIsRateLimited(t *testing.T) {
	a, h := storeApp(t)
	linked(t, a, "201", "me@example.com")
	a.cfg.UploadsPerMinute = 1 // 4 a minute for a member
	got := map[int]int{}
	for i := 0; i < 7; i++ {
		rec, _ := storeCall(t, h, "filehop", storeSecret, "201", "r"+strconv.Itoa(i)+".png", testPNG(20+i, 20))
		got[rec.Code]++
	}
	if got[http.StatusCreated] != 4 || got[http.StatusTooManyRequests] != 3 {
		t.Errorf("answers: %v", got)
	}
}

// The manifest names the capability only when the endpoint is on.
func TestHopManifestNamesMediaStore(t *testing.T) {
	for _, on := range []bool{false, true} {
		var a *App
		var h http.Handler
		if on {
			a, h = storeApp(t)
		} else {
			a, h = ssoApp(t)
		}
		_ = a
		path := "/hop/manifest"
		req := httptest.NewRequest("GET", path, nil)
		for k, v := range hop.Headers("imperium", testSecret, "system", "GET", path, nil) {
			req.Header[k] = v
		}
		rec := httptest.NewRecorder()
		h.ServeHTTP(rec, req)
		if got := strings.Contains(rec.Body.String(), "media_store"); got != on {
			t.Errorf("store configured=%v, manifest names media_store=%v: %s", on, got, rec.Body.String())
		}
	}
}

func sha256hex(b []byte) string {
	s := sha256.Sum256(b)
	return hex.EncodeToString(s[:])
}

// B1: what a page that is cross-origin isolated (FileHop, COEP require-corp) may load from here.
// A CORS fetch needs Access-Control-Allow-Origin on the answer, and gets no help from CORP; an <img>
// or <video> (no-cors) needs Cross-Origin-Resource-Policy: cross-origin. The file endpoints send both,
// the JSON API sends the CORS headers (also on an error answer, so a failed call can be read), and
// the signed server-to-server answers are never read by a browser.
func TestIsolatedPagesCanFetchAndEmbed(t *testing.T) {
	a, h := ssoApp(t)
	_, key := member(t, a, "iso@example.com")
	id := uploadFile(t, h, key, "pic.png", testPNG(40, 30))["id"].(string)

	for _, path := range []string{"/i/" + id + ".png", "/t/" + id + ".jpg"} {
		hd := fetch(h, path).Header()
		if hd.Get("Cross-Origin-Resource-Policy") != "cross-origin" || hd.Get("Access-Control-Allow-Origin") != "*" {
			t.Errorf("%s: CORP %q, ACAO %q", path, hd.Get("Cross-Origin-Resource-Policy"), hd.Get("Access-Control-Allow-Origin"))
		}
	}
	pre := httptest.NewRequest("OPTIONS", "/api/v1/upload", nil)
	pre.Header.Set("Origin", "https://file.example")
	pre.Header.Set("Access-Control-Request-Method", "POST")
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, pre)
	hd := rec.Header()
	if rec.Code != http.StatusNoContent || hd.Get("Access-Control-Allow-Origin") != "*" || !strings.Contains(hd.Get("Access-Control-Allow-Headers"), "Authorization") {
		t.Errorf("preflight: %d %v", rec.Code, hd)
	}
	for _, path := range []string{"/api/v1/images/" + id, "/api/v1/me"} { // answered, and refused
		if hd := fetch(h, path).Header(); hd.Get("Access-Control-Allow-Origin") != "*" {
			t.Errorf("%s: no CORS header for a page on another origin", path)
		}
	}
}

// trapBody is a request body that fails the test when anything reads it.
type trapBody struct {
	t    *testing.T
	read bool
}

func (b *trapBody) Read(p []byte) (int, error) {
	b.read = true
	b.t.Error("the body was read before the caller was known")
	return 0, io.EOF
}

// shaped is a call whose headers look right (a signature that is the right length) but is not signed by anyone.
func shaped(body io.Reader, size int64, mod func(http.Header)) *http.Request {
	req := httptest.NewRequest("POST", "/hop/media", body)
	req.ContentLength = size
	h := req.Header
	h.Set("X-HOP-App", "filehop")
	h.Set("X-HOP-Timestamp", strconv.FormatInt(time.Now().Unix(), 10))
	h.Set("X-HOP-Nonce", "abcdefghijklmnopqrstuv")
	h.Set("X-HOP-Actor", "201")
	h.Set("X-HOP-Signature", strings.Repeat("A", 43))
	if mod != nil {
		mod(h)
	}
	return req
}

// A caller without the secret is turned away before its body is read, and is never counted against the app.
func TestHopMediaRefusesCheaplyBeforeReadingTheBody(t *testing.T) {
	a, h := storeApp(t)
	linked(t, a, "201", "me@example.com")
	cases := map[string]struct {
		mod  func(http.Header)
		size int64
		want int
	}{
		"short signature":     {func(x http.Header) { x.Set("X-HOP-Signature", "abc") }, 100, 401},
		"bad signature chars": {func(x http.Header) { x.Set("X-HOP-Signature", strings.Repeat("!", 43)) }, 100, 401},
		"short nonce":         {func(x http.Header) { x.Set("X-HOP-Nonce", "short") }, 100, 401},
		"old timestamp":       {func(x http.Header) { x.Set("X-HOP-Timestamp", "1000") }, 100, 401},
		"no timestamp":        {func(x http.Header) { x.Del("X-HOP-Timestamp") }, 100, 401},
		"unknown app":         {func(x http.Header) { x.Set("X-HOP-App", "nobody") }, 100, 401},
		"announced too large": {nil, a.cfg.MaxUploadBytes + 1, 413},
	}
	for name, c := range cases {
		body := &trapBody{t: t}
		rec := httptest.NewRecorder()
		h.ServeHTTP(rec, shaped(body, c.size, c.mod))
		if rec.Code != c.want || body.read {
			t.Errorf("%s: %d (want %d), body read %v", name, rec.Code, c.want, body.read)
		}
	}
	// Only a fixed number of unverified bodies may be held at once: with every slot taken, the next is refused unread.
	for i := 0; i < hopMediaUnverified; i++ {
		a.hopMediaSlots.take("10.0.0." + strconv.Itoa(i))
	}
	body := &trapBody{t: t}
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, shaped(body, 100, nil))
	if rec.Code != http.StatusServiceUnavailable || body.read {
		t.Errorf("all slots taken: %d, body read %v", rec.Code, body.read)
	}
	for i := 0; i < hopMediaUnverified; i++ {
		a.hopMediaSlots.release("10.0.0." + strconv.Itoa(i))
	}
	if rec, _ := storeCall(t, h, "filehop", storeSecret, "201", "a.png", testPNG(20, 20)); rec.Code != http.StatusCreated {
		t.Errorf("a signed call after the slots freed: %d", rec.Code)
	}
}

// Bad-signature traffic from elsewhere does not use up what the real app may do.
func TestHopMediaFloodOfBadSignaturesDoesNotBlockTheRealApp(t *testing.T) {
	a, h := storeApp(t)
	linked(t, a, "201", "me@example.com")
	limited := 0
	for i := 0; i < hopMediaPerIP+hopMediaPerApp/2; i++ {
		req := shaped(strings.NewReader("junk"), 4, func(x http.Header) { x.Set("X-HOP-Nonce", "flood-nonce-number-"+strconv.Itoa(i)) })
		req.RemoteAddr = "203.0.113.7:4000"
		rec := httptest.NewRecorder()
		h.ServeHTTP(rec, req)
		switch rec.Code {
		case http.StatusUnauthorized:
		case http.StatusTooManyRequests:
			limited++
		default:
			t.Fatalf("flood call %d answered %d", i, rec.Code)
		}
	}
	if limited == 0 {
		t.Error("the flooding address was never limited")
	}
	// The real caller (another address) is not limited: neither the app's nor the member's allowance was spent.
	for i := 0; i < 4; i++ {
		if rec, _ := storeCall(t, h, "filehop", storeSecret, "201", "ok"+strconv.Itoa(i)+".png", testPNG(20+i, 20)); rec.Code != http.StatusCreated {
			t.Fatalf("signed call %d after the flood: %d %s", i, rec.Code, rec.Body.String())
		}
	}
}

// slowSender opens a connection as the address ip (through X-Forwarded-For), sends well-formed headers that announce
// a body, then only part of it, and holds the connection.
func slowSender(t *testing.T, addr, ip string, rest int) net.Conn {
	t.Helper()
	c, err := net.Dial("tcp", addr)
	if err != nil {
		t.Fatal(err)
	}
	fmt.Fprintf(c, "POST /hop/media HTTP/1.1\r\nHost: x\r\nX-Forwarded-For: %s\r\nContent-Length: 1000\r\n"+
		"X-HOP-App: filehop\r\nX-HOP-Timestamp: %d\r\nX-HOP-Nonce: abcdefghijklmnopqrstuv\r\nX-HOP-Actor: 201\r\nX-HOP-Signature: %s\r\n\r\nabc",
		ip, time.Now().Unix(), strings.Repeat("A", 43))
	c.Write(bytes.Repeat([]byte("x"), rest)) // the rest of the announced body, or nothing: then the sender stalls
	return c
}

// One address holds one slot; a body that stalls past the deadline frees it and gets an error;
// a real signed call from another address works all the while.
func TestHopMediaSlowBodiesCannotHoldTheSlots(t *testing.T) {
	t.Setenv("PIXELHOP_TRUSTED_PROXIES", "127.0.0.1")
	a, h := storeApp(t)
	linked(t, a, "201", "me@example.com")
	old := hopMediaReadWindow
	hopMediaReadWindow = 700 * time.Millisecond
	defer func() { hopMediaReadWindow = old }()
	srv := httptest.NewServer(h)
	defer srv.Close()
	addr := strings.TrimPrefix(srv.URL, "http://")

	first := slowSender(t, addr, "198.51.100.1", 0)
	defer first.Close()
	time.Sleep(200 * time.Millisecond) // it is now holding its slot
	// Same address again: refused at once, without waiting for the first.
	second := slowSender(t, addr, "198.51.100.1", 997) // a whole body, so the refusal is not held up by it
	defer second.Close()
	second.SetReadDeadline(time.Now().Add(400 * time.Millisecond))
	resp, err := http.ReadResponse(bufio.NewReader(second), nil)
	if err != nil || resp.StatusCode != http.StatusServiceUnavailable {
		t.Fatalf("a second slot for one address: %v %v", resp, err)
	}
	// Another address still gets through and is served while the first one stalls.
	body := testPNG(25, 25)
	path := "/hop/media?name=a.png"
	req, _ := http.NewRequest("POST", srv.URL+path, bytes.NewReader(body))
	for k, v := range hop.Headers("filehop", storeSecret, "201", "POST", path, body) {
		req.Header[k] = v
	}
	req.Header.Set("X-Forwarded-For", "198.51.100.2")
	ok, err := http.DefaultClient.Do(req)
	if err != nil || ok.StatusCode != http.StatusCreated {
		t.Fatalf("a signed call from another address while one is stalled: %v %v", ok, err)
	}
	ok.Body.Close()
	// The stalled body runs out of time: it gets an error answer and its slot is free again.
	first.SetReadDeadline(time.Now().Add(3 * time.Second))
	late, err := http.ReadResponse(bufio.NewReader(first), nil)
	if err != nil || late.StatusCode != http.StatusRequestTimeout {
		t.Fatalf("a stalled body: %v %v", late, err)
	}
	if !a.hopMediaSlots.take("198.51.100.1") {
		t.Fatal("the stalled sender still holds its slot")
	}
}
