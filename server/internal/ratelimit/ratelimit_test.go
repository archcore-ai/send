package ratelimit

import (
	"testing"
	"time"
)

func TestAllowsBurstThenBlocks(t *testing.T) {
	cl := &clock{t: time.Unix(0, 0)}
	l := New(3, time.Minute, cl.now) // burst 3

	for i := range 3 {
		if !l.Allow("ip1") {
			t.Fatalf("event %d should be allowed within burst", i)
		}
	}
	if l.Allow("ip1") {
		t.Errorf("4th event should be blocked")
	}
	// A different key has its own bucket.
	if !l.Allow("ip2") {
		t.Errorf("independent key should be allowed")
	}
}

func TestRefillOverTime(t *testing.T) {
	cl := &clock{t: time.Unix(0, 0)}
	l := New(60, time.Minute, cl.now) // 1 token/sec, burst 60

	for range 60 {
		l.Allow("ip")
	}
	if l.Allow("ip") {
		t.Fatalf("bucket should be empty")
	}
	cl.advance(2 * time.Second) // refill ~2 tokens
	if !l.Allow("ip") {
		t.Errorf("should allow after refill")
	}
}

func TestDisabledLimiterAllowsAll(t *testing.T) {
	l := New(0, time.Minute, nil)
	for range 1000 {
		if !l.Allow("ip") {
			t.Fatalf("disabled limiter must allow everything")
		}
	}
}

type clock struct{ t time.Time }

func (c *clock) now() time.Time          { return c.t }
func (c *clock) advance(d time.Duration) { c.t = c.t.Add(d) }
