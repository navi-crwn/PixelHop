package app

import (
	"context"
	"sync"

	"github.com/navi-crwn/pixelhop/internal/db"
)

// quotaLocks hands out one lock per member. A member's uploads take turns
// from "how much is used" to "the row is stored", so requests that arrive
// together cannot all pass the same reading of the quota. Other members are
// not held up. The server is one process on one database, so an in-memory
// lock is enough.
type quotaLocks struct {
	mu sync.Mutex
	m  map[int64]*quotaLock
}

type quotaLock struct {
	turn chan struct{} // holds one token while somebody has the lock
	refs int           // callers holding or waiting for it
}

func (q *quotaLocks) lock(ctx context.Context, id int64) bool {
	q.mu.Lock()
	l := q.m[id]
	if l == nil {
		if q.m == nil {
			q.m = map[int64]*quotaLock{}
		}
		l = &quotaLock{turn: make(chan struct{}, 1)}
		q.m[id] = l
	}
	l.refs++
	q.mu.Unlock()
	select {
	case l.turn <- struct{}{}:
		return true
	case <-ctx.Done():
		q.forget(id, l)
		return false
	}
}

func (q *quotaLocks) unlock(id int64) {
	q.mu.Lock()
	l := q.m[id]
	q.mu.Unlock()
	<-l.turn
	q.forget(id, l)
}

func (q *quotaLocks) forget(id int64, l *quotaLock) {
	q.mu.Lock()
	if l.refs--; l.refs == 0 {
		delete(q.m, id)
	}
	q.mu.Unlock()
}

// reserveQuota checks that n more bytes fit in u's quota. When they do, the
// member's lock stays held and done must be called once the file is stored
// (or has failed): check, store and commit happen as one step. When they do
// not fit (or the request was cancelled while waiting), nothing is held and
// used says what the member is using now. Guests have no quota: the answer is
// always yes, with a done that does nothing.
func (a *App) reserveQuota(ctx context.Context, u *db.User, n int64) (done func(), used int64, ok bool) {
	if u == nil {
		return func() {}, 0, true
	}
	if !a.quota.lock(ctx, u.ID) {
		return nil, 0, false
	}
	used, _ = a.db.UserStorage(ctx, u.ID)
	if used+n > u.QuotaBytes {
		a.quota.unlock(u.ID)
		return nil, used, false
	}
	var once sync.Once
	return func() { once.Do(func() { a.quota.unlock(u.ID) }) }, used, true
}
