// Package integration exercises the real handlers + real SendStore over httptest:
// full HTTP round-trips, concurrent-redeem atomicity end-to-end, the at-rest
// ciphertext-only guarantee, and (when age is installed) a real age round-trip.
package integration

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"sync"
	"testing"
	"time"

	"github.com/ivklgn/archcore-send/internal/api"
	"github.com/ivklgn/archcore-send/internal/config"
	"github.com/ivklgn/archcore-send/internal/logx"
	"github.com/ivklgn/archcore-send/internal/store"
)

type harness struct {
	url     string
	blobDir string
}

func newHarness(t *testing.T) *harness {
	t.Helper()
	blobDir := t.TempDir()
	st, err := store.OpenSQLiteState(filepath.Join(t.TempDir(), "i.db"), 10*time.Minute, nil)
	if err != nil {
		t.Fatalf("state: %v", err)
	}
	blob, err := store.OpenFilesystemBlob(blobDir)
	if err != nil {
		t.Fatalf("blob: %v", err)
	}
	coord := store.NewCoordinator(st, blob, 26214400)
	cfg := config.Config{
		DefaultTTL: time.Hour, MaxTTL: 24 * time.Hour, GrantTTL: 10 * time.Minute,
		MaxTotalBytes: 26214400, MaxPartBytes: 26214400, MaxPartCount: 64, RateWindow: time.Minute,
	}
	srv := httptest.NewServer(api.New(coord, cfg, logx.New(io.Discard), nil, nil))
	t.Cleanup(func() {
		srv.Close()
		_ = coord.Close()
	})
	return &harness{url: srv.URL, blobDir: blobDir}
}

func shaHex(b []byte) string {
	s := sha256.Sum256(b)
	return hex.EncodeToString(s[:])
}

// upload runs create→upload→finalize for the given parts and returns the send id.
func (h *harness) upload(t *testing.T, parts map[string][]byte) string {
	t.Helper()
	var decl []map[string]any
	for id, body := range parts {
		decl = append(decl, map[string]any{"part_id": id, "encrypted_size": len(body), "sha256": shaHex(body)})
	}
	body, _ := json.Marshal(map[string]any{"version": "send.v1", "one_time": true, "ttl_seconds": 3600, "parts": decl})
	resp, err := http.Post(h.url+"/v1/sends", "application/json", bytes.NewReader(body))
	if err != nil {
		t.Fatalf("create: %v", err)
	}
	var created struct {
		ID string `json:"id"`
	}
	_ = json.NewDecoder(resp.Body).Decode(&created)
	resp.Body.Close()

	for id, b := range parts {
		req, _ := http.NewRequest(http.MethodPut, h.url+"/v1/sends/"+created.ID+"/parts/"+id, bytes.NewReader(b))
		req.Header.Set("X-Send-Ciphertext-Sha256", shaHex(b))
		up, err := http.DefaultClient.Do(req)
		if err != nil {
			t.Fatalf("upload: %v", err)
		}
		up.Body.Close()
	}
	fin, err := http.Post(h.url+"/v1/sends/"+created.ID+"/finalize", "application/json", nil)
	if err != nil {
		t.Fatalf("finalize: %v", err)
	}
	fin.Body.Close()
	return created.ID
}

func (h *harness) redeem(t *testing.T, id string) (string, int) {
	t.Helper()
	resp, err := http.Post(h.url+"/v1/sends/"+id+"/redeem", "application/json", nil)
	if err != nil {
		t.Fatalf("redeem: %v", err)
	}
	defer resp.Body.Close()
	var b struct {
		RedeemToken string `json:"redeem_token"`
	}
	_ = json.NewDecoder(resp.Body).Decode(&b)
	return b.RedeemToken, resp.StatusCode
}

func (h *harness) download(t *testing.T, id, part, token string) []byte {
	t.Helper()
	req, _ := http.NewRequest(http.MethodGet, h.url+"/v1/sends/"+id+"/parts/"+part, nil)
	req.Header.Set("Authorization", "Bearer "+token)
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("download: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("download status = %d", resp.StatusCode)
	}
	got, _ := io.ReadAll(resp.Body)
	return got
}

func TestHTTPRoundTrip(t *testing.T) {
	h := newHarness(t)
	parts := map[string][]byte{"manifest": []byte("M"), "part_0001": []byte("opaque-bytes")}
	id := h.upload(t, parts)
	token, status := h.redeem(t, id)
	if status != http.StatusOK {
		t.Fatalf("redeem status = %d", status)
	}
	if got := h.download(t, id, "part_0001", token); !bytes.Equal(got, parts["part_0001"]) {
		t.Errorf("round-trip mismatch: %q", got)
	}
}

func TestConcurrentRedeemHTTP(t *testing.T) {
	h := newHarness(t)
	id := h.upload(t, map[string][]byte{"manifest": []byte("M"), "part_0001": []byte("c")})

	const N = 16
	var wg sync.WaitGroup
	codes := make([]int, N)
	start := make(chan struct{})
	for i := range N {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			<-start
			_, codes[i] = h.redeem(t, id)
		}(i)
	}
	close(start)
	wg.Wait()

	ok := 0
	for _, c := range codes {
		switch c {
		case http.StatusOK:
			ok++
		case http.StatusGone: // 410 for the losers
		default:
			t.Fatalf("unexpected redeem status %d", c)
		}
	}
	if ok != 1 {
		t.Fatalf("successful redeems = %d, want exactly 1", ok)
	}
}

func TestAtRestIsCiphertextOnly(t *testing.T) {
	h := newHarness(t)
	cipher := []byte("\x00\x01opaque ciphertext payload\xff")
	id := h.upload(t, map[string][]byte{"part_0001": cipher})

	// The stored blob equals exactly the uploaded bytes — the server adds/strips nothing.
	stored, err := os.ReadFile(filepath.Join(h.blobDir, id, "part_0001"))
	if err != nil {
		t.Fatalf("read blob: %v", err)
	}
	if !bytes.Equal(stored, cipher) {
		t.Errorf("stored blob != uploaded ciphertext")
	}

	// Public metadata carries no plaintext title or semantic part names.
	resp, err := http.Get(h.url + "/v1/sends/" + id)
	if err != nil {
		t.Fatalf("meta: %v", err)
	}
	defer resp.Body.Close()
	raw, _ := io.ReadAll(resp.Body)
	if bytes.Contains(bytes.ToLower(raw), []byte("title")) {
		t.Errorf("metadata leaked a title field: %s", raw)
	}
}

// TestRealAgeRoundTrip proves real age ciphertext survives the server unchanged
// and that what is stored at rest is a valid age file. Skipped if age is absent.
func TestRealAgeRoundTrip(t *testing.T) {
	ageBin, err1 := exec.LookPath("age")
	keygen, err2 := exec.LookPath("age-keygen")
	if err1 != nil || err2 != nil {
		t.Skip("age / age-keygen not on PATH")
	}

	dir := t.TempDir()
	idFile := filepath.Join(dir, "id.txt")
	if out, err := exec.Command(keygen, "-o", idFile).CombinedOutput(); err != nil {
		t.Fatalf("age-keygen: %v (%s)", err, out)
	}
	recipientOut, err := exec.Command(keygen, "-y", idFile).Output()
	if err != nil {
		t.Fatalf("age-keygen -y: %v", err)
	}
	recipient := string(bytes.TrimSpace(recipientOut))

	plaintext := []byte("the secret working context")
	enc := exec.Command(ageBin, "-r", recipient)
	enc.Stdin = bytes.NewReader(plaintext)
	cipher, err := enc.Output()
	if err != nil {
		t.Fatalf("age encrypt: %v", err)
	}

	h := newHarness(t)
	id := h.upload(t, map[string][]byte{"part_0001": cipher})
	token, status := h.redeem(t, id)
	if status != http.StatusOK {
		t.Fatalf("redeem status = %d", status)
	}
	downloaded := h.download(t, id, "part_0001", token)

	if !bytes.HasPrefix(downloaded, []byte("age-encryption.org/v1")) {
		t.Errorf("at-rest payload is not a valid age file")
	}
	dec := exec.Command(ageBin, "-d", "-i", idFile)
	dec.Stdin = bytes.NewReader(downloaded)
	got, err := dec.Output()
	if err != nil {
		t.Fatalf("age decrypt: %v", err)
	}
	if !bytes.Equal(got, plaintext) {
		t.Errorf("decrypted = %q, want %q", got, plaintext)
	}
}
