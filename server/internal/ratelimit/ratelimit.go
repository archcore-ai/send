// Package ratelimit is a small in-memory token-bucket limiter keyed by an opaque
// client key (a hashed IP, per security-privacy R6). Idle buckets are evicted and
// the map is size-capped so a distributed flood cannot grow memory unbounded.
package ratelimit

import (
	"sync"
	"time"
)

const maxKeys = 100_000

// Limiter enforces a per-key rate of `perWindow` events per `window`, with a burst
// equal to perWindow. A limiter with perWindow <= 0 allows everything (disabled).
type Limiter struct {
	mu      sync.Mutex
	buckets map[string]*bucket
	rate    float64 // tokens per second
	burst   float64
	now     func() time.Time
}

type bucket struct {
	tokens float64
	last   time.Time
}

// New builds a limiter. now may be nil (defaults to time.Now); tests inject it.
func New(perWindow int, window time.Duration, now func() time.Time) *Limiter {
	if now == nil {
		now = time.Now
	}
	var rate float64
	if perWindow > 0 && window > 0 {
		rate = float64(perWindow) / window.Seconds()
	}
	return &Limiter{
		buckets: make(map[string]*bucket),
		rate:    rate,
		burst:   float64(perWindow),
		now:     now,
	}
}

// Allow reports whether an event for key is permitted now, consuming a token if so.
func (l *Limiter) Allow(key string) bool {
	if l.rate <= 0 {
		return true // disabled
	}
	l.mu.Lock()
	defer l.mu.Unlock()
	now := l.now()

	b := l.buckets[key]
	if b == nil {
		if len(l.buckets) >= maxKeys {
			l.evict(now)
		}
		l.buckets[key] = &bucket{tokens: l.burst - 1, last: now}
		return true
	}
	b.tokens = min(l.burst, b.tokens+now.Sub(b.last).Seconds()*l.rate)
	b.last = now
	if b.tokens < 1 {
		return false
	}
	b.tokens--
	return true
}

// evict drops buckets idle long enough to have fully refilled (so dropping them
// changes nothing observable). Caller holds the lock.
func (l *Limiter) evict(now time.Time) {
	idle := time.Duration(l.burst / l.rate * float64(time.Second))
	for k, b := range l.buckets {
		if now.Sub(b.last) >= idle {
			delete(l.buckets, k)
		}
	}
}
