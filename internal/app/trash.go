package app

import (
	"context"
	"net/http"
	"time"

	"github.com/navi-crwn/pixelhop/internal/db"
)

// trashDays is how long a deleted file can still be restored.
const trashDays = 7

type trashItem struct {
	*db.Image
	Left int64 // when it will be removed for good
}

func (a *App) trashPage(w http.ResponseWriter, r *http.Request) {
	u := currentUser(r)
	ims, total, _ := a.db.Images(r.Context(), db.ImageQuery{UserID: u.ID, Trash: true, Limit: 200})
	var items []trashItem
	for _, im := range ims {
		items = append(items, trashItem{Image: im, Left: a.db.DeletedAt(r.Context(), im.ID) + trashDays*86400})
	}
	a.render(w, r, "trash", "Trash", map[string]any{"Items": items, "Total": total, "Days": trashDays})
}

func (a *App) ownTrashed(w http.ResponseWriter, r *http.Request) *db.Image {
	u := currentUser(r)
	im, err := a.db.Image(r.Context(), r.PathValue("id"))
	if err != nil || im.UserID.Int64 != u.ID || !a.db.Trashed(r.Context(), im.ID) {
		a.notFound(w, r)
		return nil
	}
	return im
}

func (a *App) trashRestore(w http.ResponseWriter, r *http.Request) {
	if im := a.ownTrashed(w, r); im != nil {
		a.db.RestoreImage(r.Context(), im.ID)
		http.Redirect(w, r, "/dashboard/trash", http.StatusSeeOther)
	}
}

func (a *App) trashDelete(w http.ResponseWriter, r *http.Request) {
	if im := a.ownTrashed(w, r); im != nil {
		if err := a.removeImage(r.Context(), im); err != nil {
			a.serverError(w, r, err)
			return
		}
		a.db.DeleteEmptyAlbums(r.Context())
		http.Redirect(w, r, "/dashboard/trash", http.StatusSeeOther)
	}
}

func (a *App) trashEmpty(w http.ResponseWriter, r *http.Request) {
	u := currentUser(r)
	ims, _, _ := a.db.Images(r.Context(), db.ImageQuery{UserID: u.ID, Trash: true, Limit: 200})
	for _, im := range ims {
		a.removeImage(r.Context(), im)
	}
	a.db.DeleteEmptyAlbums(r.Context())
	http.Redirect(w, r, "/dashboard/trash", http.StatusSeeOther)
}

// purgeTrash removes files that have waited in the trash for a week.
func (a *App) purgeTrash(ctx context.Context) {
	for {
		ims, err := a.db.PurgeableTrash(ctx, time.Now().Unix()-trashDays*86400, 100)
		if err != nil || len(ims) == 0 {
			return
		}
		for _, im := range ims {
			if err := a.removeImage(ctx, im); err != nil {
				a.log.Error("janitor: trash", "id", im.ID, "err", err)
				return
			}
		}
		a.db.DeleteEmptyAlbums(ctx)
	}
}
