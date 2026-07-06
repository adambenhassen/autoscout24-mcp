package fetch

import (
	"bufio"
	"context"
	"errors"
	"fmt"
	"log"
	"os/exec"
	"strings"
	"sync"
	"syscall"
	"time"

	"github.com/playwright-community/playwright-go"
)

// defaultIdleTimeout tears the camoufox sidecar down after this long with no
// fetches, so an idle server does not hold a Firefox in memory forever. The next
// fetch transparently starts a fresh one.
const defaultIdleTimeout = 2 * time.Minute

// CamoufoxFetcher fetches pages via a Camoufox stealth browser. The camoufox
// server sidecar is started lazily on first use, reused across fetches, and
// reaped after idleTimeout of inactivity (restarted on the next fetch).
type CamoufoxFetcher struct {
	cmd         string
	timeout     time.Duration
	idleTimeout time.Duration

	mu        sync.Mutex // guards all fields below; also serializes page use
	closed    bool       // Close() called: no further starts
	active    bool       // a browser+sidecar is currently up
	startedOK bool       // at least one start has succeeded
	initErr   error      // sticky only for a first start that never succeeded
	pw        *playwright.Playwright
	browser   playwright.Browser
	proc      *exec.Cmd
	idleTimer *time.Timer
	idleGen   uint64 // bumped whenever the idle timer is (re)armed or cancelled
	idleReaps int    // count of idle reaps performed (observed by tests)
}

// NewCamoufoxFetcher builds a camoufox fetcher. timeout bounds page navigation
// (zero means the playwright default).
func NewCamoufoxFetcher(cmd string, timeout time.Duration) *CamoufoxFetcher {
	if cmd == "" {
		cmd = "uvx camoufox server"
	}
	return &CamoufoxFetcher{cmd: cmd, timeout: timeout, idleTimeout: defaultIdleTimeout}
}

func parseWSEndpoint(line string) (string, bool) {
	for tok := range strings.FieldsSeq(line) {
		if strings.HasPrefix(tok, "ws://") || strings.HasPrefix(tok, "wss://") {
			return tok, true
		}
	}
	return "", false
}

func (f *CamoufoxFetcher) start() error {
	parts := strings.Fields(f.cmd)
	if len(parts) == 0 {
		return fmt.Errorf("%w: AS24_CAMOUFOX_CMD is empty", ErrUnavailable)
	}
	// context.Background: the sidecar outlives any single request; Close()/reap terminates it
	proc := exec.CommandContext(context.Background(), parts[0], parts[1:]...) //nolint:gosec // command comes from operator config, not user input
	proc.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
	stdout, err := proc.StdoutPipe()
	if err != nil {
		return err
	}
	proc.Stderr = proc.Stdout // camoufox may log the endpoint on stderr
	if err := proc.Start(); err != nil {
		return err
	}
	f.proc = proc

	endpointCh := make(chan string, 1)
	go func() {
		sc := bufio.NewScanner(stdout)
		for sc.Scan() {
			if ep, ok := parseWSEndpoint(sc.Text()); ok {
				select {
				case endpointCh <- ep: // first match; buffered, never blocks
				default: // already delivered
				}
			}
			// Keep draining stdout/stderr after the endpoint is found: the sidecar
			// is long-lived, and if nothing reads the pipe it blocks once the OS
			// buffer fills, stalling every later fetch.
			// ponytail: bufio.Scanner caps a line at 64KB; camoufox log lines are
			// short. Switch to io.Copy(io.Discard, stdout) if that ever changes.
		}
	}()
	var endpoint string
	select {
	case endpoint = <-endpointCh:
	case <-time.After(60 * time.Second):
		return errors.New("camoufox server did not print ws endpoint within 60s")
	}

	pw, err := playwright.Run()
	if err != nil {
		return fmt.Errorf("starting playwright driver: %w", err)
	}
	f.pw = pw // store now so teardown() stops the driver even if Connect below fails
	browser, err := pw.Firefox.Connect(endpoint)
	if err != nil {
		return fmt.Errorf("connecting to camoufox at %s: %w", endpoint, err)
	}
	f.browser = browser
	return nil
}

// ensure brings the browser up if it is not already. Callers hold f.mu.
func (f *CamoufoxFetcher) ensure() error {
	if f.initErr != nil {
		return f.initErr // a first start failed (misconfig); do not respawn every fetch
	}
	if f.active {
		return nil
	}
	if err := f.start(); err != nil {
		if !f.startedOK {
			f.initErr = err // never worked → sticky, so the chain just skips this stage
		}
		return err // a post-reap restart failure is transient: retried on the next fetch
	}
	f.active, f.startedOK = true, true
	return nil
}

// stopIdle cancels the pending idle reap. Bumping idleGen also neutralises a
// timer that has already fired and is blocked waiting on f.mu. Callers hold f.mu.
func (f *CamoufoxFetcher) stopIdle() {
	f.idleGen++
	if f.idleTimer != nil {
		f.idleTimer.Stop()
		f.idleTimer = nil
	}
}

// armIdle (re)starts the idle-reap countdown. Called at the end of every fetch,
// so each request pushes the reap deadline out by another idleTimeout. Callers
// hold f.mu.
func (f *CamoufoxFetcher) armIdle() {
	if !f.active || f.closed || f.idleTimeout <= 0 {
		return
	}
	gen := f.idleGen
	f.idleTimer = time.AfterFunc(f.idleTimeout, func() { f.onIdle(gen) })
}

// onIdle reaps the browser after an idle period. gen guards against a fire that
// a later request already superseded.
func (f *CamoufoxFetcher) onIdle(gen uint64) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if gen != f.idleGen {
		return // a fetch stopped or rearmed the timer after this one fired
	}
	f.idleTimer = nil
	f.idleReaps++
	// Best-effort reap: no caller is waiting, the sidecar is SIGTERM'd regardless,
	// and the next fetch restarts a fresh browser. Log a teardown error but carry on.
	if err := f.teardown(); err != nil {
		log.Printf("camoufox: idle reap teardown: %v", err)
	}
}

// teardown closes the browser, driver, and sidecar and resets session state so
// the next ensure() starts fresh. Callers hold f.mu.
func (f *CamoufoxFetcher) teardown() error {
	var firstErr error
	if f.browser != nil {
		firstErr = f.browser.Close()
		f.browser = nil
	}
	if f.pw != nil {
		if err := f.pw.Stop(); err != nil && firstErr == nil {
			firstErr = err
		}
		f.pw = nil
	}
	if f.proc != nil {
		if f.proc.Process != nil {
			if err := syscall.Kill(-f.proc.Process.Pid, syscall.SIGTERM); err != nil && firstErr == nil {
				firstErr = err
			}
		}
		// Reap the sidecar so it does not pile up as a zombie across reap/restart
		// cycles. Wait returns the SIGTERM exit as an *ExitError — that is expected;
		// only surface a real wait failure.
		if werr := f.proc.Wait(); werr != nil {
			var exitErr *exec.ExitError
			if !errors.As(werr, &exitErr) && firstErr == nil {
				firstErr = werr
			}
		}
	}
	f.proc = nil
	f.active = false
	return firstErr
}

func (f *CamoufoxFetcher) Get(ctx context.Context, url string) (p *Page, err error) {
	f.mu.Lock()
	defer f.mu.Unlock()

	if f.closed {
		return nil, fmt.Errorf("%w: camoufox: fetcher is closed", ErrUnavailable)
	}
	// Cancel any pending reap first so it cannot tear the browser down mid-fetch;
	// armIdle on the way out rearms the idle window for the next request.
	f.stopIdle()
	if err := f.ensure(); err != nil {
		// ErrUnavailable so the escalation chain moves on to the next stage
		// instead of aborting when camoufox is not installed/configured.
		return nil, fmt.Errorf("%w: camoufox: %w (install: pip install \"camoufox[geoip]\", or set AS24_CAMOUFOX_CMD)", ErrUnavailable, err)
	}
	defer f.armIdle()

	if err := ctx.Err(); err != nil {
		return nil, err
	}

	page, err := f.browser.NewPage()
	if err != nil {
		return nil, err
	}
	defer func() {
		if cerr := page.Close(); cerr != nil && err == nil {
			err = cerr
		}
	}()
	gotoOpts := playwright.PageGotoOptions{WaitUntil: playwright.WaitUntilStateDomcontentloaded}
	if f.timeout > 0 {
		ms := float64(f.timeout.Milliseconds())
		gotoOpts.Timeout = &ms
	}
	resp, err := page.Goto(url, gotoOpts)
	if err != nil {
		return nil, err
	}
	// best-effort wait for the Next.js payload; pages without it still load fine
	werr := page.Locator("script#__NEXT_DATA__").WaitFor(playwright.LocatorWaitForOptions{
		State:   playwright.WaitForSelectorStateAttached,
		Timeout: playwright.Float(10000),
	})
	if werr != nil && ctx.Err() != nil {
		return nil, ctx.Err()
	}
	html, err := page.Content()
	if err != nil {
		return nil, err
	}
	status := 200
	if resp != nil {
		status = resp.Status()
	}
	p = &Page{URL: page.URL(), Status: status, Body: []byte(html)}
	if serr := classifyStatus(p, url, " (even via camoufox)"); serr != nil {
		return nil, serr
	}
	return p, nil
}

// Close shuts down the browser connection and the camoufox sidecar and blocks
// further use. It takes f.mu so it waits for any in-flight Get() to finish
// instead of tearing the browser down underneath it.
func (f *CamoufoxFetcher) Close() error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.closed = true
	f.stopIdle()
	return f.teardown()
}
