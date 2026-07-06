package fetch

import (
	"context"
	"errors"
	"os"
	"os/exec"
	"syscall"
	"testing"
	"time"
)

// Drives the idle state machine directly (no real browser): a completed fetch
// arms the reap; it fires after idleTimeout; and each subsequent "fetch"
// (stopIdle+armIdle) rearms the window, so the reap only happens once the
// requests actually stop.
func TestCamoufoxIdleReapAndRearm(t *testing.T) {
	f := NewCamoufoxFetcher("x", 0)
	const idle = 100 * time.Millisecond // wide margins so a loaded CI scheduler stays reliable
	f.idleTimeout = idle

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
	time.Sleep(3 * idle)
	if got := reaps(); got != 1 {
		t.Fatalf("want 1 reap after idle, got %d", got)
	}
	f.mu.Lock()
	stillActive := f.active
	f.mu.Unlock()
	if stillActive {
		t.Fatal("session should be marked down after an idle reap")
	}

	// Rapid fetches, each well under idleTimeout apart: rearm must keep the
	// browser alive across a span longer than idleTimeout.
	for range 5 {
		fetch()
		time.Sleep(idle / 3)
	}
	if got := reaps(); got != 1 {
		t.Fatalf("rearm should have prevented a reap; want 1, got %d", got)
	}

	// Stop fetching: the last rearm should now reap.
	time.Sleep(3 * idle)
	if got := reaps(); got != 2 {
		t.Fatalf("want 2 reaps after going idle again, got %d", got)
	}
}

// ensure() must latch a launch failure (the command never started) as sticky so
// the stage is skipped, not respawned on every fetch.
func TestCamoufoxEnsureStickyOnLaunchFailure(t *testing.T) {
	f := NewCamoufoxFetcher("x", 0)
	calls := 0
	f.startFn = func() error { calls++; return errors.New("exec: not found") } // f.proc stays nil → never launched

	f.mu.Lock()
	defer f.mu.Unlock()
	if err := f.ensure(); err == nil {
		t.Fatal("want error from failed start")
	}
	if err := f.ensure(); err == nil {
		t.Fatal("want sticky error on second call")
	}
	if calls != 1 {
		t.Fatalf("launch failure must be sticky (1 start attempt), got %d", calls)
	}
}

// A start that fails *after* the command launched (slow boot, connect error) is
// transient: ensure() must reap the partial start and retry on the next fetch.
func TestCamoufoxEnsureRetriesAfterLaunchedFailure(t *testing.T) {
	f := NewCamoufoxFetcher("x", 0)
	calls := 0
	f.startFn = func() error {
		calls++
		f.proc = &exec.Cmd{} // launched (proc set, Process nil so teardown is a no-op) but failed later
		return errors.New("connect refused")
	}

	f.mu.Lock()
	defer f.mu.Unlock()
	if err := f.ensure(); err == nil {
		t.Fatal("want error from failed start")
	}
	if err := f.ensure(); err == nil {
		t.Fatal("want error from second failed start")
	}
	if calls != 2 {
		t.Fatalf("a launched-then-failed start must be retried; want 2 attempts, got %d", calls)
	}
	if f.initErr != nil {
		t.Fatal("a post-launch failure must not be latched sticky")
	}
}

// killAndReap must terminate and reap a real process group promptly via SIGTERM.
func TestKillAndReapSIGTERM(t *testing.T) {
	proc := exec.CommandContext(context.Background(), "sleep", "60")
	proc.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
	if err := proc.Start(); err != nil {
		t.Fatal(err)
	}
	start := time.Now()
	if err := killAndReap(proc, teardownGrace); err != nil {
		t.Fatalf("killAndReap: %v", err)
	}
	if d := time.Since(start); d >= teardownGrace {
		t.Fatalf("SIGTERM-responsive process should be reaped well within grace, took %v", d)
	}
	if err := proc.Wait(); err == nil {
		t.Fatal("process should already be reaped by killAndReap")
	}
}

// killAndReap must escalate to SIGKILL when the sidecar ignores SIGTERM, so a
// wedged process can never hold the lock past the grace period.
func TestKillAndReapEscalatesToSIGKILL(t *testing.T) {
	// trap '' TERM ignores SIGTERM; a builtin busy-loop (no child that would die on
	// TERM and cascade) keeps the group leader alive until SIGKILL. It touches a
	// file once the trap is installed so we don't signal before it takes effect.
	ready := t.TempDir() + "/ready"
	proc := exec.CommandContext(context.Background(), "sh", "-c", //nolint:gosec // ready path is a test-controlled t.TempDir()
		"trap '' TERM; : > "+ready+"; while :; do :; done")
	proc.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
	if err := proc.Start(); err != nil {
		t.Fatal(err)
	}
	// Wait until the trap is installed, else SIGTERM would hit the default handler.
	deadline := time.Now().Add(2 * time.Second)
	for {
		if _, err := os.Stat(ready); err == nil {
			break
		}
		if time.Now().After(deadline) {
			t.Fatal("trap process never became ready")
		}
		time.Sleep(5 * time.Millisecond)
	}

	grace := 100 * time.Millisecond
	start := time.Now()
	if err := killAndReap(proc, grace); err != nil {
		t.Fatalf("killAndReap: %v", err)
	}
	d := time.Since(start)
	if d < grace {
		t.Fatalf("should have waited out the grace before SIGKILL, took only %v", d)
	}
	if d > grace+2*time.Second {
		t.Fatalf("SIGKILL escalation should be prompt after grace, took %v", d)
	}
}

// Close() must supersede a pending idle reap: a timer that fires after Close must
// not run teardown a second time.
func TestCamoufoxCloseCancelsPendingReap(t *testing.T) {
	f := NewCamoufoxFetcher("x", 0)

	f.mu.Lock()
	f.active = true
	staleGen := f.idleGen
	f.mu.Unlock()

	if err := f.Close(); err != nil {
		t.Fatalf("close: %v", err)
	}
	f.onIdle(staleGen) // an already-armed timer fires late, after Close

	f.mu.Lock()
	got := f.idleReaps
	f.mu.Unlock()
	if got != 0 {
		t.Fatalf("Close must supersede the pending reap; got %d reaps", got)
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
