package db

import (
	"context"
	"strconv"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

func aiUser(t *testing.T, d *DB, email string) int64 {
	t.Helper()
	u := &User{Email: email, Name: "T", Role: "user", Status: "active"}
	if err := d.InsertUser(context.Background(), u); err != nil {
		t.Fatal(err)
	}
	return u.ID
}

// The window starts at the first use, counts to the limit, refuses without counting, gives back only inside its own
// window, starts again after a day, and the migration can be applied twice.
func TestAIDayWindow(t *testing.T) {
	ctx := context.Background()
	d, err := Open(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	defer d.Close()
	uid := aiUser(t, d, "w@example.com")
	t0 := time.Now().Truncate(time.Second)
	day := 24 * time.Hour

	for i := 1; i <= 3; i++ {
		if used, ok, err := d.AITake(ctx, uid, "upscale", 3, day, t0.Add(time.Duration(i)*time.Hour)); err != nil || !ok || used != i {
			t.Fatalf("take %d: used=%d ok=%v err=%v", i, used, ok, err)
		}
	}
	if used, ok, _ := d.AITake(ctx, uid, "upscale", 3, day, t0.Add(4*time.Hour)); ok || used != 3 {
		t.Fatalf("4th take: used=%d ok=%v", used, ok)
	}
	if used, resets, _ := d.AIUsed(ctx, uid, "upscale", day, t0.Add(5*time.Hour)); used != 3 || !resets.Equal(t0.Add(time.Hour).Add(day)) {
		t.Fatalf("AIUsed: %d, resets %v", used, resets)
	}
	// Another tool and another member have their own share.
	if _, ok, _ := d.AITake(ctx, uid, "faces", 3, day, t0); !ok {
		t.Fatal("a different tool shared the count")
	}
	if _, ok, _ := d.AITake(ctx, aiUser(t, d, "w2@example.com"), "upscale", 3, day, t0); !ok {
		t.Fatal("a different member shared the count")
	}
	// Giving back makes room; a use taken before this window began gives nothing back.
	d.AIGive(ctx, uid, "upscale", t0.Add(2*time.Hour))
	if used, _, _ := d.AIUsed(ctx, uid, "upscale", day, t0.Add(5*time.Hour)); used != 2 {
		t.Fatalf("after give back: %d", used)
	}
	d.AIGive(ctx, uid, "upscale", t0) // taken before the window began: not this window's
	if used, _, _ := d.AIUsed(ctx, uid, "upscale", day, t0.Add(5*time.Hour)); used != 2 {
		t.Fatalf("give back of an older window changed the count: %d", used)
	}
	// A day after the first use a new window begins with this use.
	if used, ok, _ := d.AITake(ctx, uid, "upscale", 3, day, t0.Add(time.Hour+day+time.Second)); !ok || used != 1 {
		t.Fatalf("next day: used=%d ok=%v", used, ok)
	}
	// No limit counts nothing.
	if _, ok, _ := d.AITake(ctx, uid, "none", 0, day, t0); !ok {
		t.Fatal("max 0 refused")
	}
	// The sweep drops windows that have run out.
	if err := d.SweepAIDay(ctx, day, t0.Add(3*day)); err != nil {
		t.Fatal(err)
	}
	var rows int
	d.QueryRow(`SELECT COUNT(*) FROM ai_day`).Scan(&rows)
	if rows != 0 {
		t.Fatalf("rows left after the windows ran out: %d", rows)
	}
	// The migration is safe to run again on a database that has the table.
	if _, err := d.Exec(migrations[18]); err != nil {
		t.Fatalf("second run of the migration: %v", err)
	}
}

// Closing and reopening the database (a restart) keeps the count; and at one moment only max takes get through.
func TestAIDayTakesAreAtomicAndDurable(t *testing.T) {
	ctx := context.Background()
	dir := t.TempDir()
	d, err := Open(dir)
	if err != nil {
		t.Fatal(err)
	}
	uid := aiUser(t, d, "a@example.com")
	now := time.Now()
	var ok atomic.Int32
	var wg sync.WaitGroup
	for i := 0; i < 60; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			if _, allowed, err := d.AITake(ctx, uid, "erase", 7, 24*time.Hour, now); err != nil {
				t.Error(err)
			} else if allowed {
				ok.Add(1)
			}
		}()
	}
	wg.Wait()
	if ok.Load() != 7 {
		t.Fatalf("%d takes got through, want 7", ok.Load())
	}
	d.Close()
	d, err = Open(dir)
	if err != nil {
		t.Fatal(err)
	}
	defer d.Close()
	if used, _, _ := d.AIUsed(ctx, uid, "erase", 24*time.Hour, now); used != 7 {
		t.Fatalf("after reopening: %d, want 7", used)
	}
}

// The janitor's sweep must never turn a member's first use of the day into a refusal. The moment that
// mattered was between "make the row" and "count on it": a sweep there deleted the half-made row and the
// member was refused before ever using the tool. A trigger plays that sweep exactly at that moment, every
// time, so the test does not depend on timing: it deletes any row whose window has not begun (start_at 0)
// as soon as it is inserted. AITake must not leave such a row, so the first use of every tool goes through.
func TestAIDaySweepBetweenStepsNeverRefuses(t *testing.T) {
	ctx := context.Background()
	d, err := Open(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	defer d.Close()
	uid := aiUser(t, d, "sweep@example.com")
	if _, err := d.Exec(`CREATE TRIGGER sweep_half_made AFTER INSERT ON ai_day WHEN NEW.start_at = 0
		BEGIN DELETE FROM ai_day WHERE user_id = NEW.user_id AND tool = NEW.tool; END`); err != nil {
		t.Fatal(err)
	}
	now := time.Now()
	for i := 0; i < 5; i++ {
		tool := "t" + strconv.Itoa(i)
		if used, ok, err := d.AITake(ctx, uid, tool, 5, 24*time.Hour, now); err != nil || !ok || used != 1 {
			t.Fatalf("first use of %s: used=%d ok=%v err=%v", tool, used, ok, err)
		}
		if used, ok, err := d.AITake(ctx, uid, tool, 5, 24*time.Hour, now); err != nil || !ok || used != 2 {
			t.Fatalf("second use of %s: used=%d ok=%v err=%v", tool, used, ok, err)
		}
	}
}

// The real sweep runs every five minutes, not flat out: one sweeper that yields between runs and a fixed,
// small number of takes, all of which must go through and count correctly.
func TestAIDayTakesWhileTheSweepRuns(t *testing.T) {
	ctx := context.Background()
	d, err := Open(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	defer d.Close()
	uid := aiUser(t, d, "sweep2@example.com")
	stop := make(chan struct{})
	done := make(chan struct{})
	go func() {
		defer close(done)
		for {
			select {
			case <-stop:
				return
			default:
				d.SweepAIDay(ctx, 24*time.Hour, time.Now())
				time.Sleep(time.Millisecond)
			}
		}
	}()
	now := time.Now()
	for i := 0; i < 100; i++ {
		if used, ok, err := d.AITake(ctx, uid, "t"+strconv.Itoa(i), 5, 24*time.Hour, now); err != nil || !ok || used != 1 {
			close(stop)
			<-done
			t.Fatalf("use %d: used=%d ok=%v err=%v", i, used, ok, err)
		}
	}
	close(stop)
	<-done
}
