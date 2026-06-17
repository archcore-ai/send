# Shared bats helpers for the Send skill tests.
# shellcheck shell=bash

REPO_ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
SEND_SH="$REPO_ROOT/skill/send/scripts/send.sh"
FAKES_DIR="$REPO_ROOT/tests/fakes"
MOCK="$REPO_ROOT/tests/mock_sendd.py"
SENDD_BIN="$REPO_ROOT/tests/bin/sendd"

# SEND_BACKEND=sendd runs the real Go server instead of the python mock. Both speak
# the same backend-http-api contract, so the e2e suite is backend-agnostic.
BACKEND="${SEND_BACKEND:-mock}"

# Prepend fake age/age-keygen to PATH only if real age is missing (or forced).
use_fake_age_if_needed() {
  if [ "${SEND_FORCE_FAKE:-0}" = "1" ] || ! command -v age >/dev/null 2>&1; then
    PATH="$FAKES_DIR:$PATH"
    export PATH
  fi
}

# Isolate temp + grant cache per test so one-time state is removable.
isolate_tmp() {
  export TMPDIR="$BATS_TEST_TMPDIR"
}

# Build a minimal valid workdir under $BATS_TEST_TMPDIR/wd. Echoes the path.
make_workdir() {
  local wd="$BATS_TEST_TMPDIR/wd"
  rm -rf "$wd"; mkdir -p "$wd/evidence" "$wd/details"
  cat > "$wd/compact.md" <<'EOF'
# Context: Auth debugging handoff
## Goal
Fix the auth cache invalidation bug.
## Current state
Tokens are not refreshed after logout.
## Suggested next steps
Inspect the cache key derivation.
EOF
  cat > "$wd/evidence/errors.md" <<'EOF'
# Errors
panic: nil map read in authcache.go:42
EOF
  cat > "$wd/details/full-diff.patch" <<'EOF'
--- a/authcache.go
+++ b/authcache.go
@@ -39,7 +39,7 @@
-  cache[key] = tok
+  cache.Store(key, tok)
EOF
  printf '%s' "$wd"
}

# Start the configured backend. Sets SERVER_URL, MOCK_PID, MOCK_LOG so the e2e
# suite stays identical across backends.
start_mock() {
  if [ "$BACKEND" = "sendd" ]; then
    start_sendd
  else
    start_mock_python
  fi
}

start_mock_python() {
  MOCK_LOG="$BATS_TEST_TMPDIR/requests.log"; : > "$MOCK_LOG"
  local urlfile="$BATS_TEST_TMPDIR/mock.url"; : > "$urlfile"
  python3 "$MOCK" --log "$MOCK_LOG" > "$urlfile" 2>/dev/null &
  MOCK_PID=$!
  local i
  for i in $(seq 1 50); do
    [ -s "$urlfile" ] && break
    sleep 0.1
  done
  SERVER_URL="$(cat "$urlfile")"
  [ -n "$SERVER_URL" ] || { echo "mock did not start" >&2; return 1; }
}

# Build the real server once (the Makefile sendd-e2e target pre-builds it).
ensure_sendd_built() {
  [ -x "$SENDD_BIN" ] && return 0
  ( cd "$REPO_ROOT/server" && go build -o "$SENDD_BIN" . ) \
    || { echo "go build sendd failed" >&2; return 1; }
}

# Start the real Go server on an ephemeral port; it prints its URL on stdout and
# writes an R5-scrubbed request log to MOCK_LOG. Rate limiting is disabled for tests.
start_sendd() {
  ensure_sendd_built || return 1
  MOCK_LOG="$BATS_TEST_TMPDIR/requests.log"; : > "$MOCK_LOG"
  local urlfile="$BATS_TEST_TMPDIR/sendd.url"; : > "$urlfile"
  SEND_LISTEN=127.0.0.1:0 \
  SEND_DB_PATH="$BATS_TEST_TMPDIR/sendd.db" \
  SEND_BLOB_DIR="$BATS_TEST_TMPDIR/blobs" \
  SEND_REQUEST_LOG="$MOCK_LOG" \
  SEND_MAX_TTL=168h SEND_DEFAULT_TTL=24h \
  SEND_RATE_CREATE_PER_MIN=0 SEND_RATE_UPLOAD_PER_MIN=0 \
    "$SENDD_BIN" > "$urlfile" 2>/dev/null &
  MOCK_PID=$!
  local i
  for i in $(seq 1 50); do
    [ -s "$urlfile" ] && break
    sleep 0.1
  done
  SERVER_URL="$(head -n1 "$urlfile")"
  [ -n "$SERVER_URL" ] || { echo "sendd did not start" >&2; return 1; }
}

stop_mock() {
  [ -n "${MOCK_PID:-}" ] && kill "$MOCK_PID" 2>/dev/null || true
  wait "${MOCK_PID:-}" 2>/dev/null || true
}
