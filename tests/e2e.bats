#!/usr/bin/env bats
# End-to-end + security tests: send -> load round-trip against the python mock.

bats_require_minimum_version 1.5.0

load helper

setup() {
  use_fake_age_if_needed
  isolate_tmp
  start_mock
}

teardown() {
  stop_mock
}

# Capture a finalized send URL into $URL (one-time default).
do_send() {
  wd="$(make_workdir)"
  run --separate-stderr bash "$SEND_SH" send "$wd" --yes --server "$SERVER_URL"
  [ "$status" -eq 0 ]
  URL="$(echo "$output" | jq -r '.url')"
}

@test "send: finalizes and returns a URL carrying the #agekey fragment" {
  do_send
  echo "$output" | jq -e '.ok == true and .one_time == true' >/dev/null
  [[ "$URL" == *"/s/snd_"* ]]
  [[ "$URL" == *"#agekey="* ]]
}

@test "load: compact-first round-trip returns compact + evidence, lists detail" {
  do_send
  run --separate-stderr bash "$SEND_SH" load "$URL" --server "$SERVER_URL"
  [ "$status" -eq 0 ]
  echo "$output" | jq -e '.ok == true' >/dev/null
  echo "$output" | jq -e '.compact_context | test("Auth debugging handoff")' >/dev/null
  echo "$output" | jq -e '.required_evidence | map(.part_id) | index("evidence.errors") != null' >/dev/null
  echo "$output" | jq -e '.available_details | map(.part_id) | index("detail.full-diff") != null' >/dev/null
}

@test "fidelity: a verbatim ## Conversation compact survives send->load byte-identical" {
  # The whole point of capturing a thin session: the visible exchange must come back
  # exactly as written, not paraphrased. Proves the transport preserves a verbatim
  # ## Conversation block (the fix's load-side guarantee).
  local wd="$BATS_TEST_TMPDIR/wd-conv"
  rm -rf "$wd"; mkdir -p "$wd"
  cat > "$wd/compact.md" <<'EOF'
# Context: Thin greeting session
## Conversation
> **User:** hello vanya
> **Me:** Hey! What can I help you with today? If you're picking up work on the
> Send project, I can dig into the architecture docs, the Go server, or the skill.
## Current state
Fresh session, branch main, clean tree. No task started yet.
EOF

  run --separate-stderr bash "$SEND_SH" send "$wd" --yes --server "$SERVER_URL"
  [ "$status" -eq 0 ]
  local url; url="$(echo "$output" | jq -r '.url')"

  run --separate-stderr bash "$SEND_SH" load "$url" --server "$SERVER_URL"
  [ "$status" -eq 0 ]
  # Equal byte-for-byte. json_escape drops the one trailing newline and $(...) strips
  # trailing newlines on both sides, so the comparison is exact for the content.
  local expected got
  expected="$(cat "$wd/compact.md")"
  got="$(echo "$output" | jq -r '.compact_context')"
  [ "$got" = "$expected" ]
  # And the exact verbatim turn survived — not flattened into a summary.
  echo "$got" | grep -qF '> **User:** hello vanya'
}

@test "INV-2: load never fetches a load_by_default:false detail part" {
  do_send
  : > "$MOCK_LOG"  # focus the log on the load phase
  run --separate-stderr bash "$SEND_SH" load "$URL" --server "$SERVER_URL"
  [ "$status" -eq 0 ]
  # compact=part_0001, evidence.errors=part_0002, detail.full-diff=part_0003
  run grep -F "GET /v1/sends/" "$MOCK_LOG"
  [[ "$output" == *"part_0001"* ]]   # compact fetched
  [[ "$output" == *"part_0002"* ]]   # evidence fetched
  [[ "$output" != *"part_0003"* ]]   # detail NOT fetched
}

@test "SECURITY: the #agekey fragment never appears in any request to the server" {
  do_send
  run --separate-stderr bash "$SEND_SH" load "$URL" --server "$SERVER_URL"
  [ "$status" -eq 0 ]
  run grep -i "agekey" "$MOCK_LOG"
  [ "$status" -ne 0 ]   # no match
}

@test "load-detail: lazily fetches the optional detail within the grant window" {
  do_send
  run --separate-stderr bash "$SEND_SH" load "$URL" --server "$SERVER_URL"
  [ "$status" -eq 0 ]
  run --separate-stderr bash "$SEND_SH" load-detail "$URL" detail.full-diff --server "$SERVER_URL"
  [ "$status" -eq 0 ]
  echo "$output" | jq -e '.part_id == "detail.full-diff"' >/dev/null
  echo "$output" | jq -e '.content | test("authcache.go")' >/dev/null
}

@test "load: without --server redeems from the link's own host (self-hosted)" {
  # Regression guard (Codex P2): a self-hosted link loaded with no --server must use
  # the host embedded in the URL, not the built-in public default. The send URL carries
  # the mock's own host, so a correct client redeems against the mock (logged here); a
  # broken client would target the default server and never reach the mock.
  do_send
  : > "$MOCK_LOG"  # focus the log on the load phase
  run --separate-stderr bash "$SEND_SH" load "$URL"
  [ "$status" -eq 0 ]
  echo "$output" | jq -e '.ok == true' >/dev/null
  run grep -F "POST /v1/sends/" "$MOCK_LOG"
  [ "$status" -eq 0 ]
  [[ "$output" == *"/redeem"* ]]
}

@test "load-detail: without --server fetches the detail from the link's own host" {
  do_send
  run --separate-stderr bash "$SEND_SH" load "$URL"   # establishes the cached grant
  [ "$status" -eq 0 ]
  : > "$MOCK_LOG"  # focus the log on the load-detail phase
  run --separate-stderr bash "$SEND_SH" load-detail "$URL" detail.full-diff
  [ "$status" -eq 0 ]
  echo "$output" | jq -e '.part_id == "detail.full-diff"' >/dev/null
  run grep -F "GET /v1/sends/" "$MOCK_LOG"
  [ "$status" -eq 0 ]
}

@test "one-time: a fresh redeem after the link was consumed is rejected" {
  do_send
  run --separate-stderr bash "$SEND_SH" load "$URL" --server "$SERVER_URL"
  [ "$status" -eq 0 ]
  # Drop the cached grant so the next load must redeem again (and be refused).
  rm -rf "$TMPDIR/archcore-send-grants"
  run --separate-stderr bash "$SEND_SH" load "$URL" --server "$SERVER_URL"
  [ "$status" -eq 6 ]
  echo "$output" | jq -e '.error_code == "SEND_ALREADY_REDEEMED"' >/dev/null
}

@test "SECURITY: temp plaintext + ephemeral identity are gone after a send" {
  before="$(find "$TMPDIR" -maxdepth 1 -name 'archcore-send.*' 2>/dev/null | wc -l | tr -d ' ')"
  do_send
  after="$(find "$TMPDIR" -maxdepth 1 -name 'archcore-send.*' 2>/dev/null | wc -l | tr -d ' ')"
  [ "$after" -le "$before" ]
}

@test "SECURITY: temp plaintext + ephemeral identity are gone after a FAILED send" {
  # security-privacy: temp deleted on success AND failure. An unreachable server makes
  # the send fail at create — AFTER the ephemeral age identity is generated and parts
  # are encrypted — so this exercises the cleanup trap on the failure path.
  before="$(find "$TMPDIR" -maxdepth 1 -name 'archcore-send.*' 2>/dev/null | wc -l | tr -d ' ')"
  wd="$(make_workdir)"
  run --separate-stderr bash "$SEND_SH" send "$wd" --yes --server "http://127.0.0.1:9"
  [ "$status" -eq 6 ]
  echo "$output" | jq -e '.error_code == "SERVER_UNREACHABLE"' >/dev/null
  after="$(find "$TMPDIR" -maxdepth 1 -name 'archcore-send.*' 2>/dev/null | wc -l | tr -d ' ')"
  [ "$after" -le "$before" ]
}

@test "integrity: a non-one-time send supports repeated independent loads" {
  wd="$(make_workdir)"
  run --separate-stderr bash "$SEND_SH" send "$wd" --yes --no-one-time --server "$SERVER_URL"
  [ "$status" -eq 0 ]
  url="$(echo "$output" | jq -r '.url')"
  rm -rf "$TMPDIR/archcore-send-grants"
  run --separate-stderr bash "$SEND_SH" load "$url" --server "$SERVER_URL"
  [ "$status" -eq 0 ]
  rm -rf "$TMPDIR/archcore-send-grants"
  run --separate-stderr bash "$SEND_SH" load "$url" --server "$SERVER_URL"
  [ "$status" -eq 0 ]
}
