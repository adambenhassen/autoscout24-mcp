package fetch

import (
	"context"
	"errors"
	"fmt"
	"net/url"
	"sync"
	"time"
)

// Stage is one step in the escalation chain. A nil Fetcher means the stage
// is in the configured chain but unavailable (missing config/install).
type Stage struct {
	Name    string
	Fetcher Fetcher
}

// Escalating tries stages in order, escalating on block signals and
// remembering per-host blocks for a cooldown window.
type Escalating struct {
	stages   []Stage
	cooldown time.Duration

	mu      sync.Mutex
	blocked map[string]time.Time // "host|stageName" -> when blocked
}

func NewEscalating(stages []Stage, cooldown time.Duration) *Escalating {
	return &Escalating{stages: stages, cooldown: cooldown, blocked: map[string]time.Time{}}
}

func (e *Escalating) Get(ctx context.Context, rawURL string) (*Page, error) {
	u, err := url.Parse(rawURL)
	if err != nil {
		return nil, fmt.Errorf("invalid url %q: %w", rawURL, err)
	}
	var lastErr error
	for _, s := range e.stages {
		if e.inCooldown(u.Host, s.Name) {
			continue
		}
		if s.Fetcher == nil {
			return nil, fmt.Errorf("%w: fallback stage %q is not configured (see README for enabling it)", ErrBlocked, s.Name)
		}
		p, err := s.Fetcher.Get(ctx, rawURL)
		if err == nil {
			return p, nil
		}
		if !errors.Is(err, ErrBlocked) {
			return nil, err
		}
		e.markBlocked(u.Host, s.Name)
		lastErr = err
	}
	if lastErr == nil {
		lastErr = fmt.Errorf("%w: all fetch stages in cooldown", ErrBlocked)
	}
	return nil, lastErr
}

func (e *Escalating) inCooldown(host, stage string) bool {
	e.mu.Lock()
	defer e.mu.Unlock()
	t, ok := e.blocked[host+"|"+stage]
	if !ok {
		return false
	}
	if time.Since(t) > e.cooldown {
		delete(e.blocked, host+"|"+stage)
		return false
	}
	return true
}

func (e *Escalating) markBlocked(host, stage string) {
	e.mu.Lock()
	defer e.mu.Unlock()
	e.blocked[host+"|"+stage] = time.Now()
}
