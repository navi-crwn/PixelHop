package app

import (
	"context"
	"time"

	"github.com/navi-crwn/pixelhop/internal/db"
)

// janitor deletes expired images and stale sessions in the background, so
// there are no cron jobs to set up.
func (a *App) janitor(ctx context.Context) {
	t := time.NewTicker(5 * time.Minute)
	defer t.Stop()
	for {
		a.sweep(ctx)
		select {
		case <-ctx.Done():
			return
		case <-t.C:
		}
	}
}

// flushViews writes the counted image views to the database every few seconds.
func (a *App) flushViews(ctx context.Context) {
	t := time.NewTicker(10 * time.Second)
	defer t.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-t.C:
			a.db.FlushViews(ctx)
		}
	}
}

func (a *App) sweep(ctx context.Context) {
	a.purgeTrash(ctx)
	removed := a.removeBatches(ctx, "janitor", func() ([]*db.Image, error) {
		return a.db.ExpiredImages(ctx, a.cfg.InactiveDays, 100)
	})
	if removed > 0 {
		a.db.DeleteEmptyAlbums(ctx)
		a.log.Info("janitor: removed expired images", "count", removed)
	}
	a.db.PurgeSessions(ctx)
	a.db.SweepAIDay(ctx, aiDay, time.Now())
	a.db.PruneGuestUploads(ctx, time.Now().Add(-2*guestDay).Unix())
}

// removeBatches removes what next returns, batch after batch. A picture whose files cannot be deleted is logged
// once and left for the next run; it must not hold back the rest. It stops when a batch removes nothing.
func (a *App) removeBatches(ctx context.Context, what string, next func() ([]*db.Image, error)) (removed int) {
	failed := map[string]bool{}
	for {
		ims, err := next()
		if err != nil || len(ims) == 0 {
			return removed
		}
		before := removed
		for _, im := range ims {
			if err := a.removeImage(ctx, im); err != nil {
				if !failed[im.ID] {
					failed[im.ID] = true
					a.log.Error(what, "id", im.ID, "err", err)
				}
				continue
			}
			removed++
		}
		if removed == before {
			return removed
		}
	}
}
