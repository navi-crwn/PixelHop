package db

import (
	"context"
	"sync"
	"time"
)

// Views are counted in memory and written in one transaction every few
// seconds, so a viral image costs one write per flush instead of three per
// visitor. A crash loses at most the last few seconds of counts.
type viewDelta struct {
	n    int
	last int64
	days map[string]int
	refs map[string]int
}

type viewBuffer struct {
	mu  sync.Mutex
	img map[string]*viewDelta
}

// CountView records one view of an image for today, and where it came from.
func (d *DB) CountView(ctx context.Context, id, refHost string) {
	if d.views == nil {
		return
	}
	day := time.Now().UTC().Format("2006-01-02")
	d.views.mu.Lock()
	defer d.views.mu.Unlock()
	if d.views.img == nil {
		d.views.img = map[string]*viewDelta{}
	}
	v := d.views.img[id]
	if v == nil {
		v = &viewDelta{days: map[string]int{}, refs: map[string]int{}}
		d.views.img[id] = v
	}
	v.n++
	v.last = now()
	v.days[day]++
	if refHost != "" {
		v.refs[refHost]++
	}
}

// FlushViews writes the counted views to the database.
func (d *DB) FlushViews(ctx context.Context) {
	if d.views == nil {
		return
	}
	d.views.mu.Lock()
	pending := d.views.img
	d.views.img = nil
	d.views.mu.Unlock()
	if len(pending) == 0 {
		return
	}
	tx, err := d.BeginTx(ctx, nil)
	if err != nil {
		d.requeueViews(pending)
		return
	}
	for id, v := range pending {
		// An image deleted since it was viewed simply matches nothing.
		tx.ExecContext(ctx, `UPDATE images SET views=views+?, last_viewed=? WHERE id=?`, v.n, v.last, id)
		for day, n := range v.days {
			tx.ExecContext(ctx, `INSERT INTO image_views (image_id,day,views) SELECT ?,?,? WHERE EXISTS (SELECT 1 FROM images WHERE id=?)
				ON CONFLICT(image_id,day) DO UPDATE SET views=views+excluded.views`, id, day, n, id)
		}
		for host, n := range v.refs {
			tx.ExecContext(ctx, `INSERT INTO image_referrers (image_id,host,views) SELECT ?,?,? WHERE EXISTS (SELECT 1 FROM images WHERE id=?)
				ON CONFLICT(image_id,host) DO UPDATE SET views=views+excluded.views`, id, host, n, id)
		}
	}
	if err := tx.Commit(); err != nil {
		d.requeueViews(pending)
	}
}

// requeueViews puts counts back after a failed write.
func (d *DB) requeueViews(pending map[string]*viewDelta) {
	d.views.mu.Lock()
	defer d.views.mu.Unlock()
	if d.views.img == nil {
		d.views.img = pending
		return
	}
	for id, v := range pending {
		cur := d.views.img[id]
		if cur == nil {
			d.views.img[id] = v
			continue
		}
		cur.n += v.n
		for k, n := range v.days {
			cur.days[k] += n
		}
		for k, n := range v.refs {
			cur.refs[k] += n
		}
	}
}
