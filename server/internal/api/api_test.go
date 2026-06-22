package api

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/ivklgn/archcore-send/internal/config"
	"github.com/ivklgn/archcore-send/internal/logx"
	"github.com/ivklgn/archcore-send/internal/store"
)

func shaHex(b []byte) string {
	s := sha256.Sum256(b)
	return hex.EncodeToString(s[:])
}

func testConfig() config.Config {
	return config.Config{
		DefaultTTL:    time.Hour,
		MaxTTL:        24 * time.Hour,
		GrantTTL:      10 * time.Minute,
		MaxTotalBytes: 26214400,
		MaxPartBytes:  26214400,
		MaxPartCount:  64,
		RateWindow:    time.Minute,
	}
}

func newTestServer(t *testing.T) (*httptest.Server, *bytes.Buffer) {
	t.Helper()
	st, err := store.OpenSQLiteState(filepath.Join(t.TempDir(), "t.db"), 10*time.Minute, nil)
	if err != nil {
		t.Fatalf("state: %v", err)
	}
	blob, err := store.OpenFilesystemBlob(t.TempDir())
	if err != nil {
		t.Fatalf("blob: %v", err)
	}
	coord := store.NewCoordinator(st, blob, 26214400)
	var buf bytes.Buffer
	srv := httptest.NewServer(New(coord, testConfig(), logx.New(&buf), nil, nil))
	t.Cleanup(func() {
		srv.Close()
		_ = coord.Close()
	})
	return srv, &buf
}

// doSend creates, uploads, and finalizes a send with the given parts; returns the id.
func doSend(t *testing.T, base string, parts map[string][]byte) string {
	t.Helper()
	var decl []map[string]any
	for id, body := range parts {
		decl = append(decl, map[string]any{"part_id": id, "encrypted_size": len(body), "sha256": shaHex(body)})
	}
	createBody, _ := json.Marshal(map[string]any{"version": "send.v1", "one_time": true, "ttl_seconds": 3600, "parts": decl})
	resp, err := http.Post(base+"/v1/sends", "application/json", bytes.NewReader(createBody))
	if err != nil {
		t.Fatalf("create: %v", err)
	}
	if resp.StatusCode != http.StatusCreated {
		t.Fatalf("create status = %d", resp.StatusCode)
	}
	var created struct {
		ID string `json:"id"`
	}
	_ = json.NewDecoder(resp.Body).Decode(&created)
	resp.Body.Close()

	for id, body := range parts {
		req, _ := http.NewRequest(http.MethodPut, base+"/v1/sends/"+created.ID+"/parts/"+id, bytes.NewReader(body))
		req.Header.Set("Content-Type", "application/octet-stream")
		req.Header.Set("X-Send-Ciphertext-Sha256", shaHex(body))
		up, err := http.DefaultClient.Do(req)
		if err != nil {
			t.Fatalf("upload %s: %v", id, err)
		}
		if up.StatusCode != http.StatusOK {
			t.Fatalf("upload %s status = %d", id, up.StatusCode)
		}
		up.Body.Close()
	}

	fin, err := http.Post(base+"/v1/sends/"+created.ID+"/finalize", "application/json", strings.NewReader("{}"))
	if err != nil {
		t.Fatalf("finalize: %v", err)
	}
	if fin.StatusCode != http.StatusOK {
		t.Fatalf("finalize status = %d", fin.StatusCode)
	}
	fin.Body.Close()
	return created.ID
}

func redeem(t *testing.T, base, id string) (token string, status int) {
	t.Helper()
	resp, err := http.Post(base+"/v1/sends/"+id+"/redeem", "application/json", nil)
	if err != nil {
		t.Fatalf("redeem: %v", err)
	}
	defer resp.Body.Close()
	var body struct {
		RedeemToken string `json:"redeem_token"`
	}
	_ = json.NewDecoder(resp.Body).Decode(&body)
	return body.RedeemToken, resp.StatusCode
}

func TestHealth(t *testing.T) {
	srv, _ := newTestServer(t)
	resp, err := http.Get(srv.URL + "/healthz")
	if err != nil {
		t.Fatalf("get: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status = %d", resp.StatusCode)
	}
}

func TestFullFlow(t *testing.T) {
	srv, _ := newTestServer(t)
	parts := map[string][]byte{
		"manifest":  []byte(`{"version":"send.v1"}`),
		"part_0001": []byte("compact ciphertext"),
	}
	id := doSend(t, srv.URL, parts)

	token, status := redeem(t, srv.URL, id)
	if status != http.StatusOK || token == "" {
		t.Fatalf("redeem status=%d token=%q", status, token)
	}

	req, _ := http.NewRequest(http.MethodGet, srv.URL+"/v1/sends/"+id+"/parts/part_0001", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	dl, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("download: %v", err)
	}
	defer dl.Body.Close()
	if dl.StatusCode != http.StatusOK {
		t.Fatalf("download status = %d", dl.StatusCode)
	}
	got, _ := io.ReadAll(dl.Body)
	if string(got) != "compact ciphertext" {
		t.Errorf("download = %q", got)
	}
}

func TestCreateValidation(t *testing.T) {
	srv, _ := newTestServer(t)
	cases := []struct {
		name string
		body map[string]any
		want int
	}{
		{"bad version", map[string]any{"version": "send.v9", "parts": []any{map[string]any{"part_id": "p", "encrypted_size": 1, "sha256": "x"}}}, http.StatusBadRequest},
		{"no parts", map[string]any{"version": "send.v1", "parts": []any{}}, http.StatusBadRequest},
		{"ttl over max", map[string]any{"version": "send.v1", "ttl_seconds": 999999, "parts": []any{map[string]any{"part_id": "p", "encrypted_size": 1, "sha256": "x"}}}, http.StatusBadRequest},
		{"oversize part", map[string]any{"version": "send.v1", "parts": []any{map[string]any{"part_id": "p", "encrypted_size": 99999999, "sha256": "x"}}}, http.StatusRequestEntityTooLarge},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			b, _ := json.Marshal(tc.body)
			resp, err := http.Post(srv.URL+"/v1/sends", "application/json", bytes.NewReader(b))
			if err != nil {
				t.Fatalf("post: %v", err)
			}
			resp.Body.Close()
			if resp.StatusCode != tc.want {
				t.Errorf("status = %d, want %d", resp.StatusCode, tc.want)
			}
		})
	}
}

func TestCreateTotalOverCapRejected(t *testing.T) {
	st, err := store.OpenSQLiteState(filepath.Join(t.TempDir(), "t.db"), 10*time.Minute, nil)
	if err != nil {
		t.Fatalf("state: %v", err)
	}
	blob, err := store.OpenFilesystemBlob(t.TempDir())
	if err != nil {
		t.Fatalf("blob: %v", err)
	}
	coord := store.NewCoordinator(st, blob, 26214400)
	cfg := testConfig()
	// Each part fits the per-part cap, but two together exceed the total cap.
	cfg.MaxPartBytes = 1000
	cfg.MaxTotalBytes = 1500
	srv := httptest.NewServer(New(coord, cfg, logx.New(&bytes.Buffer{}), nil, nil))
	t.Cleanup(func() {
		srv.Close()
		_ = coord.Close()
	})

	body, _ := json.Marshal(map[string]any{
		"version": "send.v1", "ttl_seconds": 3600,
		"parts": []any{
			map[string]any{"part_id": "a", "encrypted_size": 1000, "sha256": shaHex([]byte("a"))},
			map[string]any{"part_id": "b", "encrypted_size": 1000, "sha256": shaHex([]byte("b"))},
		},
	})
	resp, err := http.Post(srv.URL+"/v1/sends", "application/json", bytes.NewReader(body))
	if err != nil {
		t.Fatalf("create: %v", err)
	}
	resp.Body.Close()
	if resp.StatusCode != http.StatusRequestEntityTooLarge {
		t.Fatalf("total over cap = %d, want 413", resp.StatusCode)
	}
}

func TestUploadMissingShaHeaderRejected(t *testing.T) {
	srv, _ := newTestServer(t)
	body := []byte("declared-bytes")
	createBody, _ := json.Marshal(map[string]any{
		"version": "send.v1", "ttl_seconds": 3600,
		"parts": []any{map[string]any{"part_id": "part_0001", "encrypted_size": len(body), "sha256": shaHex(body)}},
	})
	resp, _ := http.Post(srv.URL+"/v1/sends", "application/json", bytes.NewReader(createBody))
	var created struct {
		ID string `json:"id"`
	}
	_ = json.NewDecoder(resp.Body).Decode(&created)
	resp.Body.Close()

	for _, tc := range []struct {
		name, sha string
	}{
		{"missing", ""},
		{"too short", "abc"},
		{"non-hex", strings.Repeat("z", 64)},
		{"uppercase", strings.Repeat("A", 64)},
	} {
		t.Run(tc.name, func(t *testing.T) {
			req, _ := http.NewRequest(http.MethodPut, srv.URL+"/v1/sends/"+created.ID+"/parts/part_0001", bytes.NewReader(body))
			if tc.sha != "" {
				req.Header.Set("X-Send-Ciphertext-Sha256", tc.sha)
			}
			up, err := http.DefaultClient.Do(req)
			if err != nil {
				t.Fatalf("upload: %v", err)
			}
			up.Body.Close()
			if up.StatusCode != http.StatusBadRequest {
				t.Errorf("malformed sha (%s) status = %d, want 400", tc.name, up.StatusCode)
			}
		})
	}
}

func TestUploadIdempotentRePut(t *testing.T) {
	srv, _ := newTestServer(t)
	body := []byte("opaque-ciphertext")
	createBody, _ := json.Marshal(map[string]any{
		"version": "send.v1", "ttl_seconds": 3600,
		"parts": []any{map[string]any{"part_id": "part_0001", "encrypted_size": len(body), "sha256": shaHex(body)}},
	})
	resp, _ := http.Post(srv.URL+"/v1/sends", "application/json", bytes.NewReader(createBody))
	var created struct {
		ID string `json:"id"`
	}
	_ = json.NewDecoder(resp.Body).Decode(&created)
	resp.Body.Close()

	// An identical re-PUT (same sha256) MUST succeed (backend-http-api: upload idempotent).
	for i := range 2 {
		req, _ := http.NewRequest(http.MethodPut, srv.URL+"/v1/sends/"+created.ID+"/parts/part_0001", bytes.NewReader(body))
		req.Header.Set("X-Send-Ciphertext-Sha256", shaHex(body))
		up, err := http.DefaultClient.Do(req)
		if err != nil {
			t.Fatalf("upload #%d: %v", i, err)
		}
		up.Body.Close()
		if up.StatusCode != http.StatusOK {
			t.Fatalf("upload #%d status = %d, want 200", i, up.StatusCode)
		}
	}
}

func TestFinalizeIncompleteRejected(t *testing.T) {
	srv, _ := newTestServer(t)
	// Declare two parts, upload only one, then finalize → 400 INCOMPLETE.
	one := []byte("one")
	two := []byte("two")
	createBody, _ := json.Marshal(map[string]any{
		"version": "send.v1", "ttl_seconds": 3600,
		"parts": []any{
			map[string]any{"part_id": "part_0001", "encrypted_size": len(one), "sha256": shaHex(one)},
			map[string]any{"part_id": "part_0002", "encrypted_size": len(two), "sha256": shaHex(two)},
		},
	})
	resp, _ := http.Post(srv.URL+"/v1/sends", "application/json", bytes.NewReader(createBody))
	var created struct {
		ID string `json:"id"`
	}
	_ = json.NewDecoder(resp.Body).Decode(&created)
	resp.Body.Close()

	req, _ := http.NewRequest(http.MethodPut, srv.URL+"/v1/sends/"+created.ID+"/parts/part_0001", bytes.NewReader(one))
	req.Header.Set("X-Send-Ciphertext-Sha256", shaHex(one))
	up, _ := http.DefaultClient.Do(req)
	up.Body.Close()

	fin, err := http.Post(srv.URL+"/v1/sends/"+created.ID+"/finalize", "application/json", nil)
	if err != nil {
		t.Fatalf("finalize: %v", err)
	}
	fin.Body.Close()
	if fin.StatusCode != http.StatusBadRequest {
		t.Fatalf("incomplete finalize = %d, want 400", fin.StatusCode)
	}
}

func TestUploadToFinalizedConflict(t *testing.T) {
	srv, _ := newTestServer(t)
	body := []byte("c")
	id := doSend(t, srv.URL, map[string][]byte{"manifest": []byte("m"), "part_0001": body})
	// Re-upload to an already-finalized send → 409 SEND_FINALIZED.
	req, _ := http.NewRequest(http.MethodPut, srv.URL+"/v1/sends/"+id+"/parts/part_0001", bytes.NewReader(body))
	req.Header.Set("X-Send-Ciphertext-Sha256", shaHex(body))
	up, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("upload: %v", err)
	}
	up.Body.Close()
	if up.StatusCode != http.StatusConflict {
		t.Fatalf("upload to finalized = %d, want 409", up.StatusCode)
	}
}

func TestRedeemNotFinalizedConflict(t *testing.T) {
	srv, _ := newTestServer(t)
	// Create (but do not finalize), then redeem → 409 SEND_NOT_FINALIZED.
	body := []byte("c")
	createBody, _ := json.Marshal(map[string]any{
		"version": "send.v1", "ttl_seconds": 3600,
		"parts": []any{map[string]any{"part_id": "part_0001", "encrypted_size": len(body), "sha256": shaHex(body)}},
	})
	resp, _ := http.Post(srv.URL+"/v1/sends", "application/json", bytes.NewReader(createBody))
	var created struct {
		ID string `json:"id"`
	}
	_ = json.NewDecoder(resp.Body).Decode(&created)
	resp.Body.Close()

	rd, err := http.Post(srv.URL+"/v1/sends/"+created.ID+"/redeem", "application/json", nil)
	if err != nil {
		t.Fatalf("redeem: %v", err)
	}
	rd.Body.Close()
	if rd.StatusCode != http.StatusConflict {
		t.Fatalf("redeem unfinalized = %d, want 409", rd.StatusCode)
	}
}

func TestRedeemUnknownNotFound(t *testing.T) {
	srv, _ := newTestServer(t)
	if _, status := redeem(t, srv.URL, "snd_does_not_exist"); status != http.StatusNotFound {
		t.Fatalf("redeem unknown = %d, want 404", status)
	}
}

func TestUploadIntegrityFails(t *testing.T) {
	srv, _ := newTestServer(t)
	body := []byte("declared-bytes")
	createBody, _ := json.Marshal(map[string]any{
		"version": "send.v1", "ttl_seconds": 3600,
		"parts": []any{map[string]any{"part_id": "part_0001", "encrypted_size": len(body), "sha256": shaHex(body)}},
	})
	resp, _ := http.Post(srv.URL+"/v1/sends", "application/json", bytes.NewReader(createBody))
	var created struct {
		ID string `json:"id"`
	}
	_ = json.NewDecoder(resp.Body).Decode(&created)
	resp.Body.Close()

	req, _ := http.NewRequest(http.MethodPut, srv.URL+"/v1/sends/"+created.ID+"/parts/part_0001", bytes.NewReader([]byte("tampered!!")))
	req.Header.Set("X-Send-Ciphertext-Sha256", shaHex(body))
	up, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("upload: %v", err)
	}
	defer up.Body.Close()
	if up.StatusCode != http.StatusUnprocessableEntity {
		t.Fatalf("tampered upload status = %d, want 422", up.StatusCode)
	}
}

func TestRedeemOneTimeThenGone(t *testing.T) {
	srv, _ := newTestServer(t)
	id := doSend(t, srv.URL, map[string][]byte{"manifest": []byte("m"), "part_0001": []byte("c")})
	if _, status := redeem(t, srv.URL, id); status != http.StatusOK {
		t.Fatalf("first redeem = %d", status)
	}
	if _, status := redeem(t, srv.URL, id); status != http.StatusGone {
		t.Fatalf("second redeem = %d, want 410", status)
	}
}

func TestDownloadWithoutGrantForbidden(t *testing.T) {
	srv, _ := newTestServer(t)
	id := doSend(t, srv.URL, map[string][]byte{"manifest": []byte("m"), "part_0001": []byte("c")})
	req, _ := http.NewRequest(http.MethodGet, srv.URL+"/v1/sends/"+id+"/parts/part_0001", nil)
	req.Header.Set("Authorization", "Bearer red_bogus")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("download: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusForbidden {
		t.Fatalf("no-grant download = %d, want 403", resp.StatusCode)
	}
}

func TestLogScrubbing(t *testing.T) {
	srv, buf := newTestServer(t)
	id := doSend(t, srv.URL, map[string][]byte{"manifest": []byte("m"), "part_0001": []byte("c")})
	token, _ := redeem(t, srv.URL, id)

	// Download via Bearer header and via ?redeem_token= query — neither may leak.
	req, _ := http.NewRequest(http.MethodGet, srv.URL+"/v1/sends/"+id+"/parts/part_0001", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	if r, err := http.DefaultClient.Do(req); err == nil {
		r.Body.Close()
	}
	if r, err := http.Get(fmt.Sprintf("%s/v1/sends/%s/parts/part_0001?redeem_token=%s", srv.URL, id, token)); err == nil {
		r.Body.Close()
	}

	logged := buf.String()
	if strings.Contains(logged, token) {
		t.Errorf("request log leaked the redeem token")
	}
	if strings.Contains(strings.ToLower(logged), "agekey") {
		t.Errorf("request log contains a fragment key")
	}
	if strings.Contains(logged, "Bearer") {
		t.Errorf("request log leaked an Authorization header")
	}
	if !strings.Contains(logged, "GET /v1/sends/") || !strings.Contains(logged, "part_0001") {
		t.Errorf("request log missing expected path entries:\n%s", logged)
	}
}

func TestTeamTokenEnforcement(t *testing.T) {
	st, err := store.OpenSQLiteState(filepath.Join(t.TempDir(), "t.db"), 10*time.Minute, nil)
	if err != nil {
		t.Fatalf("state: %v", err)
	}
	blob, err := store.OpenFilesystemBlob(t.TempDir())
	if err != nil {
		t.Fatalf("blob: %v", err)
	}
	coord := store.NewCoordinator(st, blob, 26214400)
	cfg := testConfig()
	cfg.TeamToken = "s3cr3t-team"
	srv := httptest.NewServer(New(coord, cfg, logx.New(&bytes.Buffer{}), nil, nil))
	t.Cleanup(func() {
		srv.Close()
		_ = coord.Close()
	})

	createBody := []byte(`{"version":"send.v1","ttl_seconds":3600,"parts":[{"part_id":"part_0001","encrypted_size":1,"sha256":"x"}]}`)
	post := func(token string) *http.Response {
		t.Helper()
		req, _ := http.NewRequest(http.MethodPost, srv.URL+"/v1/sends", bytes.NewReader(createBody))
		req.Header.Set("Content-Type", "application/json")
		if token != "" {
			req.Header.Set("Authorization", "Bearer "+token)
		}
		resp, err := http.DefaultClient.Do(req)
		if err != nil {
			t.Fatalf("create: %v", err)
		}
		return resp
	}

	// No token and wrong token are both rejected before any side effect.
	for _, tc := range []struct{ name, token string }{{"anonymous", ""}, {"wrong", "nope"}} {
		resp := post(tc.token)
		resp.Body.Close()
		if resp.StatusCode != http.StatusUnauthorized {
			t.Fatalf("%s create = %d, want 401", tc.name, resp.StatusCode)
		}
	}

	// The correct token lets the write through.
	resp := post("s3cr3t-team")
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusCreated {
		t.Fatalf("authorized create = %d, want 201", resp.StatusCode)
	}
	var created struct {
		ID string `json:"id"`
	}
	_ = json.NewDecoder(resp.Body).Decode(&created)

	// Reads stay anonymous even in team mode: meta is reachable without a token.
	meta, err := http.Get(srv.URL + "/v1/sends/" + created.ID)
	if err != nil {
		t.Fatalf("meta: %v", err)
	}
	meta.Body.Close()
	if meta.StatusCode != http.StatusOK {
		t.Fatalf("anonymous meta = %d, want 200", meta.StatusCode)
	}
}

func TestLandingPrecedence(t *testing.T) {
	srv, _ := newTestServer(t)
	id := doSend(t, srv.URL, map[string][]byte{"manifest": []byte("m"), "part_0001": []byte("c")})

	// "/" serves the landing, not a catch-all that shadows real routes.
	root, err := http.Get(srv.URL + "/")
	if err != nil {
		t.Fatalf("landing: %v", err)
	}
	defer root.Body.Close()
	body, _ := io.ReadAll(root.Body)
	if root.StatusCode != http.StatusOK || !strings.Contains(string(body), "Archcore Send") {
		t.Errorf("landing status=%d", root.StatusCode)
	}
	// /s/{id} serves the cosmetic page.
	page, _ := http.Get(srv.URL + "/s/" + id)
	page.Body.Close()
	if page.StatusCode != http.StatusOK {
		t.Errorf("link page status = %d", page.StatusCode)
	}
	// The real metadata route is still reachable (not shadowed).
	meta, _ := http.Get(srv.URL + "/v1/sends/" + id)
	meta.Body.Close()
	if meta.StatusCode != http.StatusOK {
		t.Errorf("meta route status = %d, want 200", meta.StatusCode)
	}
}
