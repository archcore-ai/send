// Package logx is a minimal, privacy-respecting request logger. It emits only the
// fields allowed by security-privacy R5 — method, path, status, byte count,
// duration, coarse error_code — and NEVER bodies, query strings, URL fragments,
// Authorization/redeem tokens, or referrers.
package logx

import (
	"fmt"
	"io"
	"sync"
	"time"
)

// Logger writes one structured line per request to an underlying writer.
type Logger struct {
	mu sync.Mutex
	w  io.Writer
}

// New returns a Logger writing to w.
func New(w io.Writer) *Logger {
	return &Logger{w: w}
}

// Request logs a completed request. path MUST be the URL path only (no query
// string), so redeem tokens passed as ?redeem_token= never reach the log.
func (l *Logger) Request(method, path string, status int, bytes int64, dur time.Duration, errCode string) {
	line := fmt.Sprintf("%s %s %s %d dur=%dms",
		time.Now().UTC().Format(time.RFC3339),
		method, path, status, dur.Milliseconds())
	if bytes > 0 {
		line += fmt.Sprintf(" bytes=%d", bytes)
	}
	if errCode != "" {
		line += " err=" + errCode
	}
	line += "\n"

	l.mu.Lock()
	defer l.mu.Unlock()
	_, _ = io.WriteString(l.w, line)
}
