package app

import (
	"io"
	"net/http"
	"strings"
	"sync/atomic"
	"testing"
	"time"
)

// hubStub answers the announcements address for the test: body is the JSON, delay how long the "hub" takes.
func hubStub(a *App, body string, delay time.Duration, calls *atomic.Int32) {
	a.annClient = &http.Client{Transport: roundTripFunc(func(r *http.Request) (*http.Response, error) {
		calls.Add(1)
		if r.URL.Path != "/announcements.json" || r.URL.Query().Get("app") != "pixelhop" {
			return &http.Response{StatusCode: 404, Body: io.NopCloser(strings.NewReader("")), Header: http.Header{}}, nil
		}
		select {
		case <-time.After(delay):
		case <-r.Context().Done():
			return nil, r.Context().Err()
		}
		return &http.Response{StatusCode: 200, Body: io.NopCloser(strings.NewReader(body)), Header: http.Header{"Content-Type": {"application/json"}}}, nil
	})}
}

type roundTripFunc func(*http.Request) (*http.Response, error)

func (f roundTripFunc) RoundTrip(r *http.Request) (*http.Response, error) { return f(r) }

const famJSON = `{"version":1,"items":[
 {"id":"12","level":"info","title":"Scheduled <b>maintenance</b>","body":"Plain <script>alert(1)</script> text.","url":"https://hel.example.com/announcements/12","apps":["*"]},
 {"id":"11","level":"info","title":"Older","body":"older one","apps":["*"]}]}`

func dash(t *testing.T, a *App, h http.Handler) string {
	t.Helper()
	u, _ := member(t, a, "fam"+randomToken(4)+"@example.com")
	return fetch(h, "/dashboard", sessionFor(t, a, u.ID)).Body.String()
}

// (a) a running announcement shows as one card on the dashboard, under the app's own announcement; (c) its
// text is escaped; the link is kept only for an https address on the hub's domain.
func TestFamilyAnnouncementCard(t *testing.T) {
	a, h := ssoApp(t)
	var calls atomic.Int32
	hubStub(a, famJSON, 0, &calls)
	a.cfg.HelinkURL = "https://hel.example.com"
	a.db.SetSetting(t.Context(), "announcement", "App level notice.")
	a.refreshFamilyAnnouncements() // what the background trip does
	body := dash(t, a, h)
	if strings.Count(body, `id="fam-announce"`) != 1 {
		t.Fatalf("want exactly one family card:\n%s", body[:min(len(body), 400)])
	}
	if app, fam := strings.Index(body, `id="announce"`), strings.Index(body, `id="fam-announce"`); app < 0 || app > fam {
		t.Error("the app's own announcement must be above the family card")
	}
	if strings.Contains(body, "<script>alert(1)") || strings.Contains(body, "<b>maintenance</b>") {
		t.Error("markup in an announcement was not escaped")
	}
	for _, want := range []string{"&lt;b&gt;maintenance&lt;/b&gt;", `data-id="12"`, `href="https://hel.example.com/announcements/12"`, `id="fam-announce-close"`} {
		if !strings.Contains(body, want) {
			t.Errorf("card lacks %s", want)
		}
	}
	if strings.Contains(body, "older one") {
		t.Error("more than one family card")
	}
}

// (e) a critical one is picked first and cannot be closed; bad ids and foreign links are dropped.
func TestFamilyAnnouncementCriticalBadIDAndLinks(t *testing.T) {
	a, h := ssoApp(t)
	var calls atomic.Int32
	hubStub(a, `{"items":[
 {"id":"a/../b","level":"critical","title":"Bad id","body":"x"},
 {"id":"13","level":"info","title":"Brand new notice","body":"n","url":"https://evil.example.org/x"},
 {"id":"9","level":"critical","title":"Keep out of the dark","body":"c","url":"http://hel.example.com/insecure"}]}`, 0, &calls)
	a.cfg.HelinkURL = "https://hel.example.com"
	a.refreshFamilyAnnouncements()
	body := dash(t, a, h)
	if !strings.Contains(body, "Keep out of the dark") || strings.Contains(body, "Brand new notice") || strings.Contains(body, "Bad id") {
		t.Fatalf("wrong card picked")
	}
	if strings.Contains(body, `id="fam-announce-close"`) {
		t.Error("a critical announcement has a close button")
	}
	if strings.Contains(body, "http://hel.example.com/insecure") || strings.Contains(body, "evil.example.org") {
		t.Error("a link that is not https on the hub's domain was kept")
	}
	a.famAnn.items = nil
	hubStub(a, `{"items":[{"id":"14","level":"info","title":"Zebra notice","body":"b","url":"https://evil.example.org/x"}]}`, 0, &calls)
	a.refreshFamilyAnnouncements()
	if body := dash(t, a, h); strings.Contains(body, "evil.example.org") || !strings.Contains(body, "Zebra notice") {
		t.Error("a foreign link was kept, or the card is missing")
	}
}

// (b) a hub that is slow or down never holds the dashboard: the page answers at once without the card, the
// trip happens in the background, and a failed trip is not retried for a minute.
func TestFamilyAnnouncementNeverSlowsTheDashboard(t *testing.T) {
	a, h := ssoApp(t)
	var calls atomic.Int32
	hubStub(a, famJSON, 5*time.Second, &calls) // answers after 5 s: the trip gives up at 2 s
	a.cfg.HelinkURL = "https://hel.example.com"
	u, _ := member(t, a, "slow@example.com")
	c := sessionFor(t, a, u.ID)
	start := time.Now()
	body := fetch(h, "/dashboard", c).Body.String()
	if d := time.Since(start); d > time.Second {
		t.Fatalf("the dashboard waited %v for the hub", d)
	}
	if strings.Contains(body, `id="fam-announce"`) {
		t.Error("a card from a hub that has not answered")
	}
	// Let the background trip end (it gives up at 2 s) before the stub is swapped.
	for i := 0; i < 60; i++ {
		a.famAnn.mu.Lock()
		busy := a.famAnn.busy
		a.famAnn.mu.Unlock()
		if !busy {
			break
		}
		time.Sleep(100 * time.Millisecond)
	}
	// Down hub, direct: error, old copy (none) kept, no panic.
	a.annClient = &http.Client{Transport: roundTripFunc(func(*http.Request) (*http.Response, error) { return nil, io.ErrUnexpectedEOF })}
	a.refreshFamilyAnnouncements()
	if a.familyAnnouncement() != nil {
		t.Error("a card appeared from a failed trip")
	}
	// A good copy survives a later failure.
	hubStub(a, famJSON, 0, &calls)
	a.refreshFamilyAnnouncements()
	a.annClient = &http.Client{Transport: roundTripFunc(func(*http.Request) (*http.Response, error) { return nil, io.ErrUnexpectedEOF })}
	a.famAnn.mu.Lock()
	a.famAnn.freshUntil = time.Now().Add(-time.Second)
	a.famAnn.mu.Unlock()
	if a.familyAnnouncement() == nil {
		t.Error("the last good copy was dropped when the hub went away")
	}
}

// (f) without a hub nothing is asked of anyone.
func TestFamilyAnnouncementNeedsAHub(t *testing.T) {
	a, h := standaloneApp(t)
	var calls atomic.Int32
	hubStub(a, famJSON, 0, &calls)
	u, _ := member(t, a, "solo-ann@example.com")
	body := fetch(h, "/dashboard", sessionFor(t, a, u.ID)).Body.String()
	time.Sleep(50 * time.Millisecond)
	if calls.Load() != 0 || strings.Contains(body, "fam-announce") {
		t.Errorf("a stand-alone install asked a hub (%d calls) or showed a card", calls.Load())
	}
}
