package store

import (
	"context"
	"errors"
	"path/filepath"
	"sync"
	"testing"
	"time"
)

type clock struct {
	mu sync.Mutex
	t  time.Time
}

func (c *clock) now() time.Time {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.t
}

func (c *clock) advance(d time.Duration) {
	c.mu.Lock()
	c.t = c.t.Add(d)
	c.mu.Unlock()
}

func newState(t *testing.T) (*SQLiteState, *clock) {
	t.Helper()
	cl := &clock{t: time.Unix(1_700_000_000, 0).UTC()}
	st, err := OpenSQLiteState(filepath.Join(t.TempDir(), "test.db"), 10*time.Minute, cl.now)
	if err != nil {
		t.Fatalf("OpenSQLiteState: %v", err)
	}
	t.Cleanup(func() { _ = st.Close() })
	return st, cl
}

func TestOpenSQLiteStateCreatesParentDir(t *testing.T) {
	// The default SEND_DB_PATH lives under data/; a fresh run must create it.
	path := filepath.Join(t.TempDir(), "data", "nested", "sends.db")
	st, err := OpenSQLiteState(path, 10*time.Minute, nil)
	if err != nil {
		t.Fatalf("OpenSQLiteState with missing parent: %v", err)
	}
	_ = st.Close()
}

func twoParts() []PartMeta {
	return []PartMeta{
		{PartID: "manifest", EncryptedSize: 10, SHA256: "aa"},
		{PartID: "part_0001", EncryptedSize: 20, SHA256: "bb"},
	}
}

func makeFinalized(t *testing.T, st *SQLiteState, oneTime bool, ttl time.Duration) string {
	t.Helper()
	ctx := context.Background()
	in := CreateSendInput{Version: "send.v1", OneTime: oneTime, TTL: ttl, Parts: twoParts()}
	rec, err := st.CreateSend(ctx, in)
	if err != nil {
		t.Fatalf("CreateSend: %v", err)
	}
	for _, p := range in.Parts {
		if err := st.MarkUploaded(ctx, rec.ID, p.PartID); err != nil {
			t.Fatalf("MarkUploaded: %v", err)
		}
	}
	if err := st.FinalizeSend(ctx, rec.ID); err != nil {
		t.Fatalf("FinalizeSend: %v", err)
	}
	return rec.ID
}

func TestCreateAndMeta(t *testing.T) {
	st, _ := newState(t)
	ctx := context.Background()
	rec, err := st.CreateSend(ctx, CreateSendInput{Version: "send.v1", OneTime: true, TTL: time.Hour, Parts: twoParts()})
	if err != nil {
		t.Fatalf("CreateSend: %v", err)
	}
	meta, err := st.GetSendMeta(ctx, rec.ID)
	if err != nil {
		t.Fatalf("GetSendMeta: %v", err)
	}
	if meta.Status != SendStatusCreating {
		t.Errorf("status = %q, want creating", meta.Status)
	}
	if meta.PartCount != 2 || meta.TotalEncryptedSize != 30 {
		t.Errorf("part_count=%d total=%d, want 2/30", meta.PartCount, meta.TotalEncryptedSize)
	}
	if _, err := st.GetSendMeta(ctx, "snd_nope"); !errors.Is(err, ErrNotFound) {
		t.Errorf("unknown meta err = %v, want ErrNotFound", err)
	}
}

func TestFinalizeIncompleteThenComplete(t *testing.T) {
	st, _ := newState(t)
	ctx := context.Background()
	rec, err := st.CreateSend(ctx, CreateSendInput{Version: "send.v1", OneTime: true, TTL: time.Hour, Parts: twoParts()})
	if err != nil {
		t.Fatalf("CreateSend: %v", err)
	}
	if err := st.FinalizeSend(ctx, rec.ID); !errors.Is(err, ErrIncomplete) {
		t.Fatalf("finalize incomplete err = %v, want ErrIncomplete", err)
	}
	for _, p := range twoParts() {
		if err := st.MarkUploaded(ctx, rec.ID, p.PartID); err != nil {
			t.Fatalf("MarkUploaded: %v", err)
		}
	}
	if err := st.FinalizeSend(ctx, rec.ID); err != nil {
		t.Fatalf("finalize complete: %v", err)
	}
	if err := st.FinalizeSend(ctx, rec.ID); err != nil {
		t.Fatalf("re-finalize (idempotent) should be nil, got %v", err)
	}
}

func TestRedeemOneTimeIsSingleUse(t *testing.T) {
	st, _ := newState(t)
	ctx := context.Background()
	id := makeFinalized(t, st, true, time.Hour)

	g, err := st.RedeemSend(ctx, id)
	if err != nil {
		t.Fatalf("first redeem: %v", err)
	}
	if g.Token == "" || len(g.Parts) != 2 {
		t.Errorf("grant = %+v, want token + 2 parts", g)
	}
	if _, err := st.RedeemSend(ctx, id); !errors.Is(err, ErrAlreadyRedeemed) {
		t.Errorf("second redeem err = %v, want ErrAlreadyRedeemed", err)
	}
}

func TestRedeemErrors(t *testing.T) {
	st, cl := newState(t)
	ctx := context.Background()

	if _, err := st.RedeemSend(ctx, "snd_nope"); !errors.Is(err, ErrNotFound) {
		t.Errorf("unknown redeem = %v, want ErrNotFound", err)
	}

	rec, _ := st.CreateSend(ctx, CreateSendInput{Version: "send.v1", OneTime: true, TTL: time.Hour, Parts: twoParts()})
	if _, err := st.RedeemSend(ctx, rec.ID); !errors.Is(err, ErrNotFinalized) {
		t.Errorf("unfinalized redeem = %v, want ErrNotFinalized", err)
	}

	id := makeFinalized(t, st, true, time.Minute)
	cl.advance(2 * time.Minute)
	if _, err := st.RedeemSend(ctx, id); !errors.Is(err, ErrExpired) {
		t.Errorf("expired redeem = %v, want ErrExpired", err)
	}
}

func TestRedeemNonOneTimeRepeatable(t *testing.T) {
	st, _ := newState(t)
	ctx := context.Background()
	id := makeFinalized(t, st, false, time.Hour)
	for i := range 3 {
		if _, err := st.RedeemSend(ctx, id); err != nil {
			t.Fatalf("non-one-time redeem #%d: %v", i, err)
		}
	}
}

func TestConcurrentRedeemExactlyOneWins(t *testing.T) {
	st, _ := newState(t)
	id := makeFinalized(t, st, true, time.Hour)

	const N = 24
	var wg sync.WaitGroup
	results := make([]error, N)
	start := make(chan struct{})
	for i := range N {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			<-start
			_, results[i] = st.RedeemSend(context.Background(), id)
		}(i)
	}
	close(start)
	wg.Wait()

	ok := 0
	for _, err := range results {
		switch {
		case err == nil:
			ok++
		case errors.Is(err, ErrAlreadyRedeemed):
		default:
			t.Fatalf("unexpected redeem error: %v", err)
		}
	}
	if ok != 1 {
		t.Fatalf("got %d successful redeems, want exactly 1", ok)
	}
}

func TestValidateGrant(t *testing.T) {
	st, cl := newState(t)
	ctx := context.Background()
	id := makeFinalized(t, st, true, time.Hour)
	g, err := st.RedeemSend(ctx, id)
	if err != nil {
		t.Fatalf("redeem: %v", err)
	}

	if err := st.ValidateGrant(ctx, id, HashToken(g.Token)); err != nil {
		t.Errorf("valid grant rejected: %v", err)
	}
	if err := st.ValidateGrant(ctx, id, HashToken("red_wrong")); !errors.Is(err, ErrInvalidGrant) {
		t.Errorf("wrong token = %v, want ErrInvalidGrant", err)
	}
	cl.advance(11 * time.Minute)
	if err := st.ValidateGrant(ctx, id, HashToken(g.Token)); !errors.Is(err, ErrInvalidGrant) {
		t.Errorf("expired grant = %v, want ErrInvalidGrant", err)
	}
}

func TestDeleteExpiredReapsStaleUnfinished(t *testing.T) {
	st, cl := newState(t)
	ctx := context.Background()

	// An unfinished ("creating") send with no parts uploaded.
	rec, err := st.CreateSend(ctx, CreateSendInput{Version: "send.v1", OneTime: true, TTL: time.Hour, Parts: twoParts()})
	if err != nil {
		t.Fatalf("CreateSend: %v", err)
	}

	// Still fresh (well under the TTL and creatingMaxAge): not reaped yet.
	if n, _, err := st.DeleteExpired(ctx, cl.now()); err != nil || n != 0 {
		t.Fatalf("premature reap: n=%d err=%v", n, err)
	}

	// Past creatingMaxAge but still within the (1h) TTL: reaped as a stale upload,
	// not as an expired send.
	cl.advance(creatingMaxAge + time.Minute)
	n, keys, err := st.DeleteExpired(ctx, cl.now())
	if err != nil {
		t.Fatalf("DeleteExpired: %v", err)
	}
	if n != 1 {
		t.Errorf("reaped sends = %d, want 1", n)
	}
	if len(keys) != 2 {
		t.Errorf("purge keys = %d, want 2", len(keys))
	}
	if _, err := st.GetSendMeta(ctx, rec.ID); !errors.Is(err, ErrNotFound) {
		t.Errorf("stale unfinished send still present: %v", err)
	}
}

func TestDeleteExpiredReapsConsumedPastGrant(t *testing.T) {
	st, cl := newState(t)
	ctx := context.Background()
	id := makeFinalized(t, st, true, time.Hour) // 1h TTL, well beyond the grant window

	if _, err := st.RedeemSend(ctx, id); err != nil {
		t.Fatalf("redeem: %v", err)
	}
	// Within the grant window the consumed send is still downloadable, so not reaped.
	if n, _, err := st.DeleteExpired(ctx, cl.now()); err != nil || n != 0 {
		t.Fatalf("premature reap of consumed send: n=%d err=%v", n, err)
	}
	// Past the grant window (but still before TTL) the consumed one-time send is reaped.
	cl.advance(11 * time.Minute)
	n, _, err := st.DeleteExpired(ctx, cl.now())
	if err != nil {
		t.Fatalf("DeleteExpired: %v", err)
	}
	if n != 1 {
		t.Errorf("reaped sends = %d, want 1", n)
	}
	if _, err := st.GetSendMeta(ctx, id); !errors.Is(err, ErrNotFound) {
		t.Errorf("consumed send past grant still present: %v", err)
	}
}

func TestDeleteExpired(t *testing.T) {
	st, cl := newState(t)
	ctx := context.Background()
	expired := makeFinalized(t, st, true, time.Minute)
	live := makeFinalized(t, st, true, time.Hour)

	cl.advance(2 * time.Minute) // expired's TTL passes; live still has ~58m
	sends, keys, err := st.DeleteExpired(ctx, cl.now())
	if err != nil {
		t.Fatalf("DeleteExpired: %v", err)
	}
	if sends != 1 {
		t.Errorf("deleted sends = %d, want 1", sends)
	}
	if len(keys) != 2 {
		t.Errorf("purge keys = %d, want 2 (the expired send's parts)", len(keys))
	}
	if _, err := st.GetSendMeta(ctx, expired); !errors.Is(err, ErrNotFound) {
		t.Errorf("expired send still present: %v", err)
	}
	if _, err := st.GetSendMeta(ctx, live); err != nil {
		t.Errorf("live send wrongly deleted: %v", err)
	}
}
