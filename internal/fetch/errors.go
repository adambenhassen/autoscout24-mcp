// Package fetch provides the page-fetching layer with anti-bot escalation.
package fetch

import "errors"

var (
	ErrBlocked  = errors.New("blocked by anti-bot protection")
	ErrNotFound = errors.New("not found")
	ErrParse    = errors.New("page structure not recognized (site may have changed)")
)
