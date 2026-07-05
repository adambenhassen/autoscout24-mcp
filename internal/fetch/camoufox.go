package fetch

import (
	"bufio"
	"context"
	"errors"
	"fmt"
	"net/http"
	"os/exec"
	"strings"
	"sync"
	"syscall"
	"time"

	"github.com/playwright-community/playwright-go"
)

// CamoufoxFetcher fetches pages via a Camoufox stealth browser. The camoufox
// server sidecar is started lazily on first use and reused afterwards.
type CamoufoxFetcher struct {
	cmd     string
	timeout time.Duration

	once    sync.Once
	initErr error
	pw      *playwright.Playwright
	browser playwright.Browser
	proc    *exec.Cmd

	mu sync.Mutex // serializes page use on the single browser
}

// NewCamoufoxFetcher builds a camoufox fetcher. timeout bounds page navigation
// (zero means the playwright default).
func NewCamoufoxFetcher(cmd string, timeout time.Duration) *CamoufoxFetcher {
	if cmd == "" {
		cmd = "uvx camoufox server"
	}
	return &CamoufoxFetcher{cmd: cmd, timeout: timeout}
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
	// context.Background: the sidecar outlives any single request; Close() terminates it
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
				endpointCh <- ep
				return
			}
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
	browser, err := pw.Firefox.Connect(endpoint)
	if err != nil {
		return fmt.Errorf("connecting to camoufox at %s: %w", endpoint, err)
	}
	f.pw, f.browser = pw, browser
	return nil
}

func (f *CamoufoxFetcher) Get(ctx context.Context, url string) (p *Page, err error) {
	f.once.Do(func() { f.initErr = f.start() })
	if f.initErr != nil {
		// ErrUnavailable so the escalation chain moves on to the next stage
		// instead of aborting when camoufox is not installed/configured.
		return nil, fmt.Errorf("%w: camoufox: %w (install: pip install \"camoufox[geoip]\", or set AS24_CAMOUFOX_CMD)", ErrUnavailable, f.initErr)
	}
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	f.mu.Lock()
	defer f.mu.Unlock()

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
	switch {
	case status == http.StatusNotFound || status == http.StatusGone:
		return nil, fmt.Errorf("%s: %w", url, ErrNotFound)
	case IsBlocked(p):
		return nil, fmt.Errorf("%s: %w (even via camoufox)", url, ErrBlocked)
	}
	return p, nil
}

// Close shuts down the browser connection and the camoufox sidecar.
func (f *CamoufoxFetcher) Close() error {
	var firstErr error
	if f.browser != nil {
		firstErr = f.browser.Close()
	}
	if f.pw != nil {
		if err := f.pw.Stop(); err != nil && firstErr == nil {
			firstErr = err
		}
	}
	if f.proc != nil && f.proc.Process != nil {
		if err := syscall.Kill(-f.proc.Process.Pid, syscall.SIGTERM); err != nil && firstErr == nil {
			firstErr = err
		}
	}
	return firstErr
}
