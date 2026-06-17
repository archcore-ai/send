#!/usr/bin/env bats
# Unit/behavior tests for send.sh — offline, no network. Stubs age via fakes.

bats_require_minimum_version 1.5.0

load helper

setup() {
  use_fake_age_if_needed
  isolate_tmp
}

@test "doctor: emits a single valid JSON object with ok=true" {
  run --separate-stderr bash "$SEND_SH" doctor
  [ "$status" -eq 0 ]
  echo "$output" | jq -e '.ok == true' >/dev/null
  echo "$output" | jq -e 'has("age") and has("curl") and has("gzip") and has("server")' >/dev/null
}

@test "usage: unknown subcommand exits 2 with BAD_REQUEST" {
  run --separate-stderr bash "$SEND_SH" frobnicate
  [ "$status" -eq 2 ]
  echo "$output" | jq -e '.ok == false and .error_code == "BAD_REQUEST"' >/dev/null
}

@test "send: missing workdir argument exits 2" {
  run --separate-stderr bash "$SEND_SH" send
  [ "$status" -eq 2 ]
  echo "$output" | jq -e '.ok == false' >/dev/null
}

@test "inspect: clean workdir is a dry run with compact included" {
  wd="$(make_workdir)"
  run --separate-stderr bash "$SEND_SH" inspect "$wd"
  [ "$status" -eq 0 ]
  echo "$output" | jq -e '.ok == true and .dry_run == true' >/dev/null
  echo "$output" | jq -e '.included | index("compact") != null' >/dev/null
  echo "$output" | jq -e '.optional_parts | index("detail.full-diff") != null' >/dev/null
}

@test "secret scan: planted AWS key blocks by default (exit 4)" {
  wd="$(make_workdir)"
  printf 'leaked AKIA1234567890ABCDEF here\n' >> "$wd/evidence/errors.md"
  run --separate-stderr bash "$SEND_SH" inspect "$wd"
  [ "$status" -eq 4 ]
  echo "$output" | jq -e '.error_code == "SECRET_DETECTED"' >/dev/null
}

@test "secret scan: --allow-secrets overrides the block" {
  wd="$(make_workdir)"
  printf 'leaked AKIA1234567890ABCDEF here\n' >> "$wd/evidence/errors.md"
  run --separate-stderr bash "$SEND_SH" inspect "$wd" --allow-secrets
  [ "$status" -eq 0 ]
  echo "$output" | jq -e '.ok == true' >/dev/null
}

@test "size: compact over the hard cap is rejected (exit 5)" {
  wd="$(make_workdir)"
  # 60 KB compact > 50 KB hard cap
  { printf '# Context: big\n'; head -c 61440 /dev/zero | tr '\0' 'x'; } > "$wd/compact.md"
  run --separate-stderr bash "$SEND_SH" inspect "$wd"
  [ "$status" -eq 5 ]
  echo "$output" | jq -e '.error_code == "SEND_TOO_LARGE"' >/dev/null
}

@test "ttl: invalid duration is rejected up front (exit 2)" {
  wd="$(make_workdir)"
  run --separate-stderr bash "$SEND_SH" inspect "$wd" --ttl bogus
  [ "$status" -eq 2 ]
  echo "$output" | jq -e '.error_code == "BAD_REQUEST"' >/dev/null
}

@test "load: URL without a fragment fails FRAGMENT_MISSING (exit 7)" {
  run --separate-stderr bash "$SEND_SH" load "http://127.0.0.1:9/s/snd_abc"
  [ "$status" -eq 7 ]
  echo "$output" | jq -e '.error_code == "FRAGMENT_MISSING"' >/dev/null
}
