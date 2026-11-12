package app

import (
	"encoding/json"
	"net/http"
	"time"

	"github.com/navi-crwn/pixelhop/internal/db"
)

// Reminder cards are the dashboard's nudges towards a task a member has not
// done yet (today: the four "getting started" steps). The HOP family's
// hop-ui.js gives every [data-hop-remind] card a "Hide" button that posts its
// key here. A hidden card stays away for seven days and comes back if the task
// is still open; once the task is done the card is gone for good, whatever was
// hidden. The same pattern as NoteHop and the Laravel apps.

const reminderQuiet = 7 * 24 * time.Hour

// remindStart: the getting-started steps are not all done.
const remindStart = "getting-started"

var reminderKeys = map[string]bool{remindStart: true}

// reminderDue reports whether a card is shown now: its task is still open and
// the member has not hidden it in the last seven days.
func (a *App) reminderDue(r *http.Request, u *db.User, key string, taskOpen bool, now time.Time) bool {
	if !taskOpen {
		return false
	}
	at, err := a.db.ReminderDismissedAt(r.Context(), u.ID, key)
	if err != nil {
		a.log.Warn("reminder", "key", key, "err", err)
		return true
	}
	return at == 0 || now.Unix()-at >= int64(reminderQuiet/time.Second)
}

// apiDismissReminder: POST /api/me/reminders/dismiss {"key": "getting-started"}.
func (a *App) apiDismissReminder(w http.ResponseWriter, r *http.Request) {
	u := a.apiUser(w, r)
	if u == nil {
		return
	}
	var q struct {
		Key string `json:"key"`
	}
	if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, 256)).Decode(&q); err != nil || !reminderKeys[q.Key] {
		apiError(w, http.StatusBadRequest, "unknown reminder")
		return
	}
	if err := a.db.DismissReminder(r.Context(), u.ID, q.Key); err != nil {
		a.log.Error("dismiss reminder", "err", err)
		apiError(w, http.StatusInternalServerError, "could not save that; please try again")
		return
	}
	apiOK(w, map[string]any{"again": time.Now().Add(reminderQuiet).Unix()})
}
