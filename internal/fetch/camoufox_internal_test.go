package fetch

import (
	"testing"
	"time"
)

// Drives the idle state machine directly (no real browser): a completed fetch
// arms the reap; it fires after idleTimeout; and each subsequent "fetch"
// (stopIdle+armIdle) rearms the window, so the reap only happens once the
// requests actually stop.
func TestCamoufoxIdleReapAndRearm(t *testing.T) {
	f := NewCamoufoxFetcher("x", 0)
	f.idleTimeout = 30 * time.Millisecond

	reaps := func() int {
		f.mu.Lock()
		defer f.mu.Unlock()
		return f.idleReaps
	}
	fetch := func() { // simulate one Get: cancel pending reap, then rearm
		f.mu.Lock()
		f.active = true // pretend the browser is up (teardown() nil-checks are safe)
		f.stopIdle()
		f.armIdle()
		f.mu.Unlock()
	}

	fetch()
	time.Sleep(90 * time.Millisecond)
	if got := reaps(); got != 1 {
		t.Fatalf("want 1 reap after idle, got %d", got)
	}
	f.mu.Lock()
	stillActive := f.active
	f.mu.Unlock()
	if stillActive {
		t.Fatal("session should be marked down after an idle reap")
	}

	// Rapid fetches, each shorter than idleTimeout apart: rearm must keep the
	// browser alive across a span longer than idleTimeout.
	for range 5 {
		fetch()
		time.Sleep(12 * time.Millisecond)
	}
	if got := reaps(); got != 1 {
		t.Fatalf("rearm should have prevented a reap; want 1, got %d", got)
	}

	// Stop fetching: the last rearm should now reap.
	time.Sleep(90 * time.Millisecond)
	if got := reaps(); got != 2 {
		t.Fatalf("want 2 reaps after going idle again, got %d", got)
	}
}

// A stale timer fire (its generation superseded by a later fetch) must not reap.
func TestCamoufoxIdleStaleFireIgnored(t *testing.T) {
	f := NewCamoufoxFetcher("x", 0)
	f.idleTimeout = time.Hour // never fires on its own

	f.mu.Lock()
	f.active = true
	staleGen := f.idleGen
	f.stopIdle() // bumps idleGen, superseding staleGen
	f.mu.Unlock()

	f.onIdle(staleGen) // simulate the old timer firing late

	f.mu.Lock()
	got := f.idleReaps
	f.mu.Unlock()
	if got != 0 {
		t.Fatalf("stale fire must not reap; got %d", got)
	}
}
