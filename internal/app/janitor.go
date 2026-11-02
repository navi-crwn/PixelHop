package app

import (
	"context"
	"time"
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
	removed := 0
	for {
		ims, err := a.db.ExpiredImages(ctx, a.cfg.InactiveDays, 100)
		if err != nil || len(ims) == 0 {
			break
		}
		for _, im := range ims {
			if err := a.removeImage(ctx, im); err != nil {
				a.log.Error("janitor", "id", im.ID, "err", err)
				return
			}
			removed++
		}
	}
	if removed > 0 {
		a.db.DeleteEmptyAlbums(ctx)
		a.log.Info("janitor: removed expired images", "count", removed)
	}
	a.db.PurgeSessions(ctx)
}
