#!/usr/bin/env bash
#
# Archcore Send — skill client (POSIX/bash).
#
# The skill's only executable: orchestrates `age` + `gzip` + `curl` to package,
# encrypt, upload, and load end-to-end-encrypted session context. It performs
# crypto, transport, secret scanning, size checks, and temp-file hygiene only.
# It MUST NOT summarize, read arbitrary repo files, or mutate project files.
#
# Contracts: skill-contract.spec, send-format.spec (send.v1), backend-http-api.spec.
# Rules: security-privacy, content-policy, size-limits, skill-scripting-conventions.
#
# Output discipline: exactly one JSON object on stdout; all human text on stderr.

if [ -z "${BASH_VERSION:-}" ]; then
  printf 'send.sh requires bash\n' >&2
  exit 1
fi

set -euo pipefail
umask 077

# ----------------------------------------------------------------------------
# Constants
# ----------------------------------------------------------------------------
readonly SEND_VERSION="send.v1"
readonly DEFAULT_TTL="24h"
readonly MAX_TTL_SECONDS=$((7 * 24 * 3600))

# Size caps (bytes) — canonical values from size-limits.rule. Do not redefine elsewhere.
readonly COMPACT_SOFT=$((30 * 1024))
readonly COMPACT_HARD=$((50 * 1024))
readonly EVIDENCE_SOFT=$((300 * 1024))
readonly EVIDENCE_HARD=$((800 * 1024))
readonly DETAIL_SOFT=$((10 * 1024 * 1024))
readonly DETAIL_HARD=$((50 * 1024 * 1024))
readonly TOTAL_ENC_SOFT=$((10 * 1024 * 1024))
readonly TOTAL_ENC_HARD=$((25 * 1024 * 1024))

# Grant cache (cross-invocation, short-lived) for one-time load + lazy load-detail.
readonly GRANT_DIR="${TMPDIR:-/tmp}/archcore-send-grants"
readonly GRANT_WINDOW_SECONDS=540 # conservative (< server's 10 min)

# ----------------------------------------------------------------------------
# Logging (human → stderr). Never put formatted human text on stdout.
# ----------------------------------------------------------------------------
if [ -t 2 ]; then
  C_RESET=$(printf '\033[0m'); C_INFO=$(printf '\033[36m')
  C_OK=$(printf '\033[32m');  C_WARN=$(printf '\033[33m')
else
  C_RESET=""; C_INFO=""; C_OK=""; C_WARN=""
fi

info()    { printf '%s%s%s\n' "$C_INFO" "$*" "$C_RESET" >&2; }
success() { printf '%s%s%s\n' "$C_OK"   "$*" "$C_RESET" >&2; }
warn()    { printf '%s%s%s\n' "$C_WARN" "$*" "$C_RESET" >&2; }

# ----------------------------------------------------------------------------
# Temp hygiene: all plaintext + identity live in OS temp, 0600, removed always.
# ----------------------------------------------------------------------------
TMP=""
cleanup() { [ -n "$TMP" ] && rm -rf "$TMP" 2>/dev/null || true; }
trap cleanup EXIT INT TERM
mktmp() { TMP="$(mktemp -d "${TMPDIR:-/tmp}/archcore-send.XXXXXX")"; }

# ----------------------------------------------------------------------------
# JSON helpers (no jq dependency — only age/curl/gzip are required tools).
# ----------------------------------------------------------------------------

# Escape stdin into a JSON string body (no surrounding quotes). Multiline → \n.
json_escape() {
  awk '
    BEGIN { ORS="" }
    {
      gsub(/\\/, "\\\\")
      gsub(/"/, "\\\"")
      gsub(/\t/, "\\t")
      gsub(/\r/, "\\r")
      if (NR > 1) printf "\\n"
      printf "%s", $0
    }
  '
}

# JSON value extractors. Use ERE (sed -E) so `(true|false)` alternation works on
# both BSD (macOS) and GNU sed; BSD BRE has no \| alternation.

# Extract a top-level JSON string value by key from stdin (controlled responses).
json_str() {
  sed -E -n 's/.*"'"$1"'"[[:space:]]*:[[:space:]]*"([^"]*)".*/\1/p' | head -n1
}

# Extract a JSON string value by key from a single-line fragment.
field_str() {
  printf '%s' "$2" | sed -E -n 's/.*"'"$1"'"[[:space:]]*:[[:space:]]*"([^"]*)".*/\1/p' | head -n1
}
field_num() {
  printf '%s' "$2" | sed -E -n 's/.*"'"$1"'"[[:space:]]*:[[:space:]]*([0-9]+).*/\1/p' | head -n1
}
field_bool() {
  printf '%s' "$2" | sed -E -n 's/.*"'"$1"'"[[:space:]]*:[[:space:]]*(true|false).*/\1/p' | head -n1
}

# Turn a JSON array/object into one object-per-line for robust iteration.
split_objects() { tr '{' '\n'; }

# ----------------------------------------------------------------------------
# Error emission: one error object on stdout + exit code (per error-catalog).
# ----------------------------------------------------------------------------
emit_error() { # code message remediation exitcode
  local code="$1" msg="$2" rem="$3" rc="$4"
  printf '{"ok":false,"error_code":"%s","message":"%s","remediation":"%s"}\n' \
    "$code" \
    "$(printf '%s' "$msg" | json_escape)" \
    "$(printf '%s' "$rem" | json_escape)"
  exit "$rc"
}

# ----------------------------------------------------------------------------
# Dependency preflight
# ----------------------------------------------------------------------------
has_cmd() { command -v "$1" >/dev/null 2>&1; }

need_cmd() {
  has_cmd "$1" && return 0
  case "$1" in
    age|age-keygen)
      emit_error AGE_NOT_FOUND "age not found on PATH" \
        "macOS: brew install age | Windows: winget install FiloSottile.age | Linux: install via distro package" 3 ;;
    curl)
      emit_error CURL_NOT_FOUND "curl not found on PATH" \
        "install curl, or use the PowerShell skill (send.ps1)" 3 ;;
    gzip)
      emit_error GZIP_NOT_FOUND "gzip not found on PATH" \
        "install gzip, or use the PowerShell skill (send.ps1)" 3 ;;
    *)
      emit_error AGE_NOT_FOUND "$1 not found on PATH" "install $1" 3 ;;
  esac
}

# ----------------------------------------------------------------------------
# Crypto / hashing primitives
# ----------------------------------------------------------------------------
sha256_hex() { # reads stdin
  if has_cmd sha256sum; then sha256sum | awk '{print $1}'
  elif has_cmd shasum; then shasum -a 256 | awk '{print $1}'
  else emit_error AGE_NOT_FOUND "no sha256 tool" "install coreutils (sha256sum) or shasum" 3
  fi
}

bytesize() { wc -c < "$1" | tr -d '[:space:]'; }

# Map filename extension → send.v1 part kind.
kind_for() {
  case "$1" in
    *.md)            printf 'markdown' ;;
    *.patch|*.diff)  printf 'patch' ;;
    *.log)           printf 'log' ;;
    *.json)          printf 'json' ;;
    *.txt)           printf 'text' ;;
    *)               printf 'binary' ;;
  esac
}

# ----------------------------------------------------------------------------
# Duration parsing: 24h / 7d / 3600s / 30m → seconds
# ----------------------------------------------------------------------------
ttl_to_seconds() {
  local v="$1" n unit
  n="${v%[smhd]}"; unit="${v##*[0-9]}"
  case "$n" in ''|*[!0-9]*) return 1 ;; esac
  case "$unit" in
    s) printf '%s' "$n" ;;
    m) printf '%s' "$((n * 60))" ;;
    h|'') printf '%s' "$((n * 3600))" ;;
    d) printf '%s' "$((n * 86400))" ;;
    *) return 1 ;;
  esac
}

# ----------------------------------------------------------------------------
# Secret scan (content-policy R1 + minimum pattern set). Logs counts/types only.
# Returns 0 if clean, 1 if a high-confidence secret is present.
# ----------------------------------------------------------------------------
secret_scan() { # workdir → prints "<type> <count>" lines to stderr, sets global SCAN_HITS
  local wd="$1" total=0 c
  SCAN_HITS=0
  scan_one() { # label  ere
    c=$(grep -rEl --binary-files=without-match "$2" "$wd" 2>/dev/null | wc -l | tr -d '[:space:]')
    if [ "${c:-0}" -gt 0 ]; then warn "  secret-scan: $1 in $c file(s)"; total=$((total + c)); fi
  }
  scan_one "private-key"     '-----BEGIN (RSA |EC |OPENSSH |)PRIVATE KEY-----'
  scan_one "aws-access-key"  'AKIA[0-9A-Z]{16}'
  scan_one "env-assignment"  '(SECRET|TOKEN|API_KEY|PASSWORD)[[:space:]]*=[[:space:]]*[^[:space:]]+'
  scan_one "github-token"    'ghp_[0-9A-Za-z]{36}|github_pat_[0-9A-Za-z_]{59}'
  scan_one "slack-token"     'xox[baprs]-[0-9A-Za-z-]+'
  scan_one "jwt"             'eyJ[A-Za-z0-9_-]+\.[A-Za-z0-9_-]+\.[A-Za-z0-9_-]+'
  scan_one "openai-key"      'sk-[A-Za-z0-9]{20,}'
  scan_one "db-uri"          'postgres://|mysql://|mongodb(\+srv)?://'
  SCAN_HITS="$total"
  [ "$total" -eq 0 ]
}

# ----------------------------------------------------------------------------
# Workdir discovery → parallel arrays (bash 3.2: no associative arrays).
# Convention (send-format.spec): compact.md + evidence/* are required &
# load-by-default; details/* are optional & lazy.
# ----------------------------------------------------------------------------
discover_parts() { # workdir
  local wd="$1" f base sid n=0
  P_SEM=(); P_TID=(); P_KIND=(); P_REQ=(); P_LBD=(); P_FILE=(); P_PSIZE=()

  if [ ! -f "$wd/compact.md" ]; then
    emit_error UNSUPPORTED_VERSION "workdir has no compact.md" \
      "the agent must assemble compact.md before calling send" 2
  fi

  n=$((n + 1))
  P_SEM+=("compact"); P_TID+=("$(printf 'part_%04d' "$n")"); P_KIND+=("markdown")
  P_REQ+=("true"); P_LBD+=("true"); P_FILE+=("$wd/compact.md")
  P_PSIZE+=("$(bytesize "$wd/compact.md")")

  shopt -s nullglob 2>/dev/null || true
  for f in "$wd"/evidence/*; do
    [ -f "$f" ] || continue
    base="$(basename "$f")"; sid="evidence.${base%.*}"
    n=$((n + 1))
    P_SEM+=("$sid"); P_TID+=("$(printf 'part_%04d' "$n")"); P_KIND+=("$(kind_for "$f")")
    P_REQ+=("true"); P_LBD+=("true"); P_FILE+=("$f"); P_PSIZE+=("$(bytesize "$f")")
  done
  for f in "$wd"/details/*; do
    [ -f "$f" ] || continue
    base="$(basename "$f")"; sid="detail.${base%.*}"
    n=$((n + 1))
    P_SEM+=("$sid"); P_TID+=("$(printf 'part_%04d' "$n")"); P_KIND+=("$(kind_for "$f")")
    P_REQ+=("false"); P_LBD+=("false"); P_FILE+=("$f"); P_PSIZE+=("$(bytesize "$f")")
  done
  shopt -u nullglob 2>/dev/null || true
}

# Enforce size caps over discovered parts. Honors --include-large for detail/total.
check_sizes() { # include_large(0|1)
  local include_large="$1" i sem psize evid_total=0 soft_hit=0
  for i in "${!P_SEM[@]}"; do
    sem="${P_SEM[$i]}"; psize="${P_PSIZE[$i]}"
    case "$sem" in
      compact)
        [ "$psize" -gt "$COMPACT_HARD" ] && emit_error SEND_TOO_LARGE \
          "compact is ${psize}B (hard cap ${COMPACT_HARD}B)" \
          "split overflow into details/*; compact hard cap is not overridable" 5
        [ "$psize" -gt "$COMPACT_SOFT" ] && soft_hit=1 ;;
      evidence.*)
        evid_total=$((evid_total + psize)) ;;
      detail.*)
        if [ "$psize" -gt "$DETAIL_HARD" ] && [ "$include_large" -ne 1 ]; then
          emit_error SEND_TOO_LARGE "$sem is ${psize}B (hard cap ${DETAIL_HARD}B)" \
            "drop the detail, or pass --include-large" 5
        fi
        [ "$psize" -gt "$DETAIL_SOFT" ] && soft_hit=1 ;;
    esac
  done
  if [ "$evid_total" -gt "$EVIDENCE_HARD" ]; then
    emit_error SEND_TOO_LARGE "required evidence is ${evid_total}B (hard cap ${EVIDENCE_HARD}B)" \
      "move material into details/*; evidence hard cap is not overridable" 5
  fi
  [ "$evid_total" -gt "$EVIDENCE_SOFT" ] && soft_hit=1
  SIZE_SOFT_HIT="$soft_hit"
}

# ----------------------------------------------------------------------------
# Canonical private manifest (send.v1). Written to $TMP/manifest.json, then
# encrypted as the reserved part `manifest`. Parts are one-per-line for the
# loader's line-oriented parser while staying valid JSON.
# ----------------------------------------------------------------------------
build_manifest() { # workdir title
  local wd="$1" title="$2" created out i default_load="" first=1
  created="$(date -u +%Y-%m-%dT%H:%M:%SZ)"
  out="$TMP/manifest.json"

  {
    printf '{\n'
    printf '  "version": "%s",\n' "$SEND_VERSION"
    printf '  "title": "%s",\n' "$(printf '%s' "$title" | json_escape)"
    printf '  "created_at": "%s",\n' "$created"
    printf '  "source": { "agent": "archcore-send-skill" },\n'
    # policy.default_load
    for i in "${!P_SEM[@]}"; do
      [ "${P_LBD[$i]}" = "true" ] || continue
      if [ "$first" -eq 1 ]; then default_load="\"${P_SEM[$i]}\""; first=0
      else default_load="$default_load, \"${P_SEM[$i]}\""; fi
    done
    printf '  "policy": { "raw_transcript_included": false, "secrets_included": false, "default_load": [%s] },\n' "$default_load"
    printf '  "parts": [\n'
    for i in "${!P_SEM[@]}"; do
      printf '    {"id":"%s","transport_id":"%s","kind":"%s","required":%s,"load_by_default":%s,"plaintext_size":%s}' \
        "${P_SEM[$i]}" "${P_TID[$i]}" "${P_KIND[$i]}" "${P_REQ[$i]}" "${P_LBD[$i]}" "${P_PSIZE[$i]}"
      if [ "$i" -lt $(( ${#P_SEM[@]} - 1 )) ]; then printf ',\n'; else printf '\n'; fi
    done
    printf '  ]\n'
    printf '}\n'
  } > "$out"
}

# Title from compact.md "# Context: <title>" line; fallback generic.
title_from_compact() {
  local t
  t="$(sed -n 's/^#[[:space:]]*Context:[[:space:]]*\(.*\)$/\1/p' "$1/compact.md" | head -n1)"
  [ -n "$t" ] && printf '%s' "$t" || printf 'Send context'
}

# ----------------------------------------------------------------------------
# Preview (human → stderr). Discloses included / optional / and the trust model.
# ----------------------------------------------------------------------------
print_preview() { # title
  local i sem psize kb
  info "Send preview — \"$1\""
  for i in "${!P_SEM[@]}"; do
    sem="${P_SEM[$i]}"; psize="${P_PSIZE[$i]}"; kb=$(( (psize + 1023) / 1024 ))
    if [ "${P_LBD[$i]}" = "true" ]; then
      printf '  included : %-22s %sKB\n' "$sem" "$kb" >&2
    else
      printf '  optional : %-22s %sKB (lazy detail)\n' "$sem" "$kb" >&2
    fi
  done
  info "The server receives ciphertext + opaque sizes only. The full link (with #agekey) decrypts — treat it like a secret."
}

# ----------------------------------------------------------------------------
# HTTP helpers (base path only — the #agekey fragment never reaches the network).
# ----------------------------------------------------------------------------
api() { printf '%s/v1%s' "$SERVER" "$1"; }

http_post_json() { # path body
  curl -fsS --retry 3 --retry-delay 2 -X POST \
    -H 'Content-Type: application/json' \
    ${SEND_TEAM_TOKEN:+-H "Authorization: Bearer $SEND_TEAM_TOKEN"} \
    --data "$2" "$(api "$1")"
}

# ============================================================================
# Subcommand: doctor
# ============================================================================
cmd_doctor() {
  local age_found=false age_ver="" curl_ok=false gzip_ok=false srv_ok=false srv="$SERVER"
  if has_cmd age; then age_found=true; age_ver="$(age --version 2>/dev/null | head -n1 | tr -d '\r')"; fi
  has_cmd curl && curl_ok=true
  has_cmd gzip && gzip_ok=true
  if [ -n "$srv" ] && has_cmd curl; then
    if curl -fsS --max-time 5 "$srv/healthz" >/dev/null 2>&1; then srv_ok=true; fi
  fi
  printf '{"ok":true,"age":{"found":%s,"version":"%s"},"curl":%s,"gzip":%s,"server":{"url":"%s","reachable":%s}}\n' \
    "$age_found" "$(printf '%s' "$age_ver" | json_escape)" "$curl_ok" "$gzip_ok" \
    "$(printf '%s' "$srv" | json_escape)" "$srv_ok"
}

# ============================================================================
# Subcommand: send / inspect
# ============================================================================
cmd_send() { # workdir  dry_run(0|1)
  local wd="$1" dry_run="$2"
  [ -d "$wd" ] || emit_error UNSUPPORTED_VERSION "workdir not found: $wd" "pass a valid workdir path" 2

  need_cmd age; need_cmd age-keygen; need_cmd gzip
  [ "$dry_run" -eq 1 ] || need_cmd curl
  [ "$dry_run" -eq 1 ] || [ -n "$SERVER" ] || \
    emit_error SERVER_UNREACHABLE "no server configured" "set SEND_SERVER_URL or pass --server URL" 6

  # Validate TTL up front so inspect/--dry-run catches it too.
  local ttl_s; ttl_s="$(ttl_to_seconds "$TTL")" || \
    emit_error BAD_REQUEST "invalid --ttl: $TTL" "use forms like 24h, 7d, 3600s" 2
  [ "$ttl_s" -le "$MAX_TTL_SECONDS" ] || \
    emit_error BAD_REQUEST "ttl exceeds 7d max" "use --ttl 7d or less" 2

  # 1. Secret scan (pre-encryption; the zero-knowledge server cannot scan ciphertext).
  if ! secret_scan "$wd"; then
    if [ "$ALLOW_SECRETS" -ne 1 ]; then
      emit_error SECRET_DETECTED "high-confidence secret(s) detected ($SCAN_HITS file matches)" \
        "redact the secrets, or pass --allow-secrets to override" 4
    fi
    warn "--allow-secrets: proceeding despite $SCAN_HITS secret match(es)"
  fi

  # 2. Discover parts + size enforcement.
  discover_parts "$wd"
  check_sizes "$INCLUDE_LARGE"

  local title; title="$(title_from_compact "$wd")"

  # 3. Build canonical manifest.
  build_manifest "$wd" "$title"

  # 4. Preview + confirmation gate.
  print_preview "$title"
  [ "$SIZE_SOFT_HIT" = "1" ] && warn "Soft size cap exceeded — review before confirming."

  if [ "$dry_run" -eq 1 ]; then
    emit_send_json "" "" "true" "$INCLUDE_LARGE" 1
    return 0
  fi

  if [ "$YES" -ne 1 ]; then
    if [ -r /dev/tty ]; then
      printf 'Proceed with encrypted upload? [y/N] ' >&2
      local ans=""; read -r ans < /dev/tty || ans=""
      case "$ans" in y|Y|yes|YES) ;; *) emit_error BAD_REQUEST "upload not confirmed" "re-run with --yes to skip the prompt" 2 ;; esac
    else
      emit_error BAD_REQUEST "confirmation required (no TTY)" "re-run with --yes" 2
    fi
  fi

  # 5. Ephemeral age identity (never persisted beyond temp).
  local idfile="$TMP/id.age" recipient secret
  age-keygen -o "$idfile" 2>/dev/null
  recipient="$(age-keygen -y "$idfile" 2>/dev/null)"
  secret="$(grep '^AGE-SECRET-KEY-' "$idfile" | head -n1)"
  [ -n "$recipient" ] && [ -n "$secret" ] || \
    emit_error DECRYPTION_FAILED "failed to generate age identity" "check the age installation" 7

  # 6. Encrypt every part (manifest first, then content) → gzip → age.
  local enc_dir="$TMP/enc"; mkdir -p "$enc_dir"
  local tids=() sizes=() shas=() i tid src sha sz total_enc=0

  encrypt_part() { # transport_id  srcfile
    local t="$1" s="$2" out="$enc_dir/$1.age"
    gzip -c -- "$s" | age -r "$recipient" > "$out"
    sha="$(sha256_hex < "$out")"; sz="$(bytesize "$out")"
    tids+=("$t"); sizes+=("$sz"); shas+=("$sha")
    total_enc=$((total_enc + sz))
  }

  encrypt_part "manifest" "$TMP/manifest.json"
  for i in "${!P_SEM[@]}"; do encrypt_part "${P_TID[$i]}" "${P_FILE[$i]}"; done

  if [ "$total_enc" -gt "$TOTAL_ENC_HARD" ] && [ "$INCLUDE_LARGE" -ne 1 ]; then
    emit_error SEND_TOO_LARGE "total encrypted size ${total_enc}B (hard cap ${TOTAL_ENC_HARD}B)" \
      "drop details, or pass --include-large" 5
  fi
  [ "$total_enc" -gt "$TOTAL_ENC_SOFT" ] && warn "Total encrypted size ${total_enc}B exceeds soft cap."

  # 7. Create → upload → finalize.
  local parts_json="" sep=""
  for i in "${!tids[@]}"; do
    parts_json="$parts_json$sep{\"part_id\":\"${tids[$i]}\",\"encrypted_size\":${sizes[$i]},\"sha256\":\"${shas[$i]}\"}"
    sep=","
  done
  local create_body create_resp send_id public_url expires_at
  create_body="{\"version\":\"$SEND_VERSION\",\"one_time\":$ONE_TIME,\"ttl_seconds\":$ttl_s,\"parts\":[$parts_json]}"
  create_resp="$(http_post_json "/sends" "$create_body")" || \
    emit_error SERVER_UNREACHABLE "create request failed" "check --server / connectivity" 6
  send_id="$(printf '%s' "$create_resp" | json_str id)"
  public_url="$(printf '%s' "$create_resp" | json_str public_url)"
  expires_at="$(printf '%s' "$create_resp" | json_str expires_at)"
  [ -n "$send_id" ] || emit_error STORAGE_ERROR "server did not return a send id" "inspect the server response/logs" 6

  for i in "${!tids[@]}"; do
    tid="${tids[$i]}"; src="$enc_dir/$tid.age"
    curl -fsS --retry 3 --retry-delay 2 -X PUT \
      -H 'Content-Type: application/octet-stream' \
      -H "X-Send-Ciphertext-Sha256: ${shas[$i]}" \
      ${SEND_TEAM_TOKEN:+-H "Authorization: Bearer $SEND_TEAM_TOKEN"} \
      --data-binary "@$src" "$(api "/sends/$send_id/parts/$tid")" >/dev/null || \
      emit_error SERVER_UNREACHABLE "upload failed for $tid" "retry; check connectivity" 6
  done

  http_post_json "/sends/$send_id/finalize" "{}" >/dev/null || \
    emit_error SERVER_UNREACHABLE "finalize failed" "retry; check connectivity" 6

  # 8. Append the key locally — the fragment never touches the network.
  local full_url="${public_url}#agekey=${secret}"
  success "Send finalized."
  emit_send_json "$full_url" "$expires_at" "$ONE_TIME" "$INCLUDE_LARGE" 0
}

# Emit the send/inspect result JSON (stdout). dry=1 → inspect form (no url).
emit_send_json() { # url expires_at one_time include_large dry
  local url="$1" exp="$2" ot="$3" dry="$5" i included="" optional="" sep
  sep=""
  for i in "${!P_SEM[@]}"; do
    if [ "${P_LBD[$i]}" = "true" ]; then included="$included$sep\"${P_SEM[$i]}\""; sep=","; fi
  done
  sep=""
  for i in "${!P_SEM[@]}"; do
    if [ "${P_LBD[$i]}" != "true" ]; then optional="$optional$sep\"${P_SEM[$i]}\""; sep=","; fi
  done
  if [ "$dry" -eq 1 ]; then
    printf '{"ok":true,"dry_run":true,"one_time":%s,"included":[%s],"optional_parts":[%s]}\n' \
      "$ot" "$included" "$optional"
  else
    printf '{"ok":true,"url":"%s","expires_at":"%s","one_time":%s,"included":[%s],"optional_parts":[%s]}\n' \
      "$(printf '%s' "$url" | json_escape)" "$(printf '%s' "$exp" | json_escape)" "$ot" "$included" "$optional"
  fi
}

# ============================================================================
# Load helpers
# ============================================================================

# Split "<base>#agekey=<secret>" locally. Sets URL_BASE, AGE_KEY, SEND_ID, SERVER.
parse_load_url() { # url
  local url="$1" frag
  case "$url" in *\#*) ;; *) emit_error FRAGMENT_MISSING "URL has no #agekey fragment" "re-copy the full link including everything after #" 7 ;; esac
  URL_BASE="${url%%#*}"
  frag="${url#*#}"
  case "$frag" in
    agekey=*) AGE_KEY="${frag#agekey=}" ;;
    k=*)
      local b64="${frag#k=}"
      if has_cmd base64; then
        AGE_KEY="$(printf '%s' "$b64" | tr '_-' '/+' | base64 -d 2>/dev/null)"
      else
        AGE_KEY="$b64"
      fi ;;
    *)        emit_error FRAGMENT_MISSING "unrecognized fragment encoding" "expected #agekey=… or #k=…" 7 ;;
  esac
  [ -n "$AGE_KEY" ] || emit_error FRAGMENT_MISSING "empty key in fragment" "re-copy the full link" 7
  SEND_ID="${URL_BASE##*/}"
  [ -n "$SERVER_OVERRIDE" ] && SERVER="$SERVER_OVERRIDE" || SERVER="${URL_BASE%/s/*}"
}

# Obtain a redeem grant, reusing a cached grant within the window if present.
# A redemption opens a 10-minute grant (practical-one-time-redemption.adr), so
# the grant is cached to let a follow-up load-detail fetch lazily within it.
get_grant() { # → sets REDEEM_TOKEN, REDEEM_RESP; writes grant cache
  mkdir -p "$GRANT_DIR" 2>/dev/null || true
  chmod 700 "$GRANT_DIR" 2>/dev/null || true
  local gf="$GRANT_DIR/$SEND_ID" now cached_exp cached_tok status body srv_code
  now="$(date +%s)"
  if [ -f "$gf" ]; then
    cached_exp="$(sed -n '1p' "$gf")"; cached_tok="$(sed -n '2p' "$gf")"
    if [ -n "$cached_exp" ] && [ "$now" -lt "$cached_exp" ] && [ -n "$cached_tok" ]; then
      REDEEM_TOKEN="$cached_tok"; REDEEM_RESP=""; return 0
    fi
  fi
  body="$TMP/redeem.json"
  status="$(curl -sS -o "$body" -w '%{http_code}' --retry 2 --retry-delay 1 \
    -X POST "$(api "/sends/$SEND_ID/redeem")" 2>/dev/null || printf '000')"
  case "$status" in
    200) ;;
    000) emit_error SERVER_UNREACHABLE "redeem request failed" "check --server / connectivity" 6 ;;
    *)
      srv_code="$(json_str error_code < "$body")"
      emit_error "${srv_code:-SERVER_UNREACHABLE}" "redeem rejected (HTTP $status)" \
        "the link may be expired or already opened; request a fresh link" 6 ;;
  esac
  REDEEM_TOKEN="$(json_str redeem_token < "$body")"
  REDEEM_RESP="$(cat "$body")"
  [ -n "$REDEEM_TOKEN" ] || emit_error SERVER_UNREACHABLE "no redeem token returned" \
    "inspect the server response" 6
  printf '%s\n%s\n' "$((now + GRANT_WINDOW_SECONDS))" "$REDEEM_TOKEN" > "$gf"
  chmod 600 "$gf" 2>/dev/null || true
}

# Download + decrypt one part (by transport id) → stdout plaintext.
fetch_decrypt() { # transport_id  idfile → writes to stdout
  local tid="$1" idf="$2"
  local enc="$TMP/dl_$tid.age"
  curl -fsS --retry 3 --retry-delay 2 \
    -H "Authorization: Bearer $REDEEM_TOKEN" \
    "$(api "/sends/$SEND_ID/parts/$tid")" -o "$enc" || \
    emit_error INTEGRITY_FAILED "download failed for $tid" "re-download; the link may be corrupted" 7
  age -d -i "$idf" < "$enc" 2>/dev/null | gunzip 2>/dev/null || \
    emit_error DECRYPTION_FAILED "could not decrypt $tid" "wrong/truncated key fragment or corrupt download" 7
}

# ============================================================================
# Subcommand: load
# ============================================================================
cmd_load() { # url
  parse_load_url "$1"
  need_cmd age; need_cmd curl; need_cmd gzip
  mktmp

  local idf="$TMP/id.key"
  printf '%s\n' "$AGE_KEY" > "$idf"; chmod 600 "$idf"

  get_grant

  # Manifest first.
  local man="$TMP/manifest.json"
  fetch_decrypt "manifest" "$idf" > "$man"
  local version; version="$(json_str version < "$man")"
  case "$version" in send.v1) ;; *) emit_error UNSUPPORTED_VERSION "unsupported format: $version" "update the skill" 1 ;; esac
  local title; title="$(json_str title < "$man")"

  # Encrypted-size map (transport_id → size) from redeem response, if present.
  local sizes_tsv="$TMP/sizes.tsv"; : > "$sizes_tsv"
  if [ -n "${REDEEM_RESP:-}" ]; then
    printf '%s' "$REDEEM_RESP" | split_objects | while IFS= read -r line; do
      case "$line" in *part_id*) ;; *) continue ;; esac
      printf '%s\t%s\n' "$(field_str part_id "$line")" "$(field_num encrypted_size "$line")"
    done > "$sizes_tsv"
  fi
  size_of() { awk -F'\t' -v t="$1" '$1==t{print $2; exit}' "$sizes_tsv"; }

  # Walk manifest parts: compact + load_by_default → fetch; details → metadata only.
  local compact_ctx="" req_json="" det_json="" rsep="" dsep=""
  while IFS=$'\t' read -r tid sid kind lbd; do
    [ -n "$tid" ] || continue
    if [ "$sid" = "compact" ]; then
      compact_ctx="$(fetch_decrypt "$tid" "$idf")"
    elif [ "$lbd" = "true" ]; then
      local content; content="$(fetch_decrypt "$tid" "$idf")"
      req_json="$req_json$rsep{\"part_id\":\"$sid\",\"content\":\"$(printf '%s' "$content" | json_escape)\"}"
      rsep=","
    else
      local esz; esz="$(size_of "$tid" 2>/dev/null)"; [ -n "$esz" ] || esz=0
      det_json="$det_json$dsep{\"part_id\":\"$sid\",\"kind\":\"$kind\",\"encrypted_size\":$esz}"
      dsep=","
    fi
  done <<EOF
$(parts_lines "$man")
EOF

  success "Loaded \"$title\" — compact-first. Details listed, not injected."
  printf '{"ok":true,"title":"%s","compact_context":"%s","required_evidence":[%s],"available_details":[%s]}\n' \
    "$(printf '%s' "$title" | json_escape)" \
    "$(printf '%s' "$compact_ctx" | json_escape)" \
    "$req_json" "$det_json"
}

# Emit "transport_id\tsemantic_id\tkind\tload_by_default" per content part.
parts_lines() { # manifest-file
  grep '"transport_id"' "$1" | while IFS= read -r line; do
    printf '%s\t%s\t%s\t%s\n' \
      "$(field_str transport_id "$line")" \
      "$(field_str id "$line")" \
      "$(field_str kind "$line")" \
      "$(field_bool load_by_default "$line")"
  done
}

# ============================================================================
# Subcommand: load-detail
# ============================================================================
cmd_load_detail() { # url part-id
  local want="$2"
  parse_load_url "$1"
  need_cmd age; need_cmd curl; need_cmd gzip
  mktmp

  local idf="$TMP/id.key"
  printf '%s\n' "$AGE_KEY" > "$idf"; chmod 600 "$idf"

  get_grant

  local man="$TMP/manifest.json"
  fetch_decrypt "manifest" "$idf" > "$man"

  local tid="" sid kind lbd ptid
  while IFS=$'\t' read -r ptid sid kind lbd; do
    [ "$sid" = "$want" ] && tid="$ptid"
  done <<EOF
$(parts_lines "$man")
EOF
  [ -n "$tid" ] || emit_error UNSUPPORTED_VERSION "no such part: $want" "list parts via --load first" 2

  local content; content="$(fetch_decrypt "$tid" "$idf")"
  success "Loaded detail \"$want\"."
  printf '{"ok":true,"part_id":"%s","content":"%s"}\n' \
    "$(printf '%s' "$want" | json_escape)" \
    "$(printf '%s' "$content" | json_escape)"
}

# ============================================================================
# Argument parsing + dispatch
# ============================================================================
usage() {
  cat >&2 <<'EOF'
send.sh — Archcore Send skill client

  send.sh doctor
  send.sh send <workdir> [--ttl 24h] [--one-time|--no-one-time] [--yes]
                          [--allow-secrets] [--include-large] [--dry-run] [--server URL]
  send.sh inspect <workdir>
  send.sh load <url> [--server URL]
  send.sh load-detail <url> <part-id>
EOF
}

main() {
  SERVER="${SEND_SERVER_URL:-}"
  SERVER_OVERRIDE=""
  TTL="$DEFAULT_TTL"
  ONE_TIME="true"
  YES=0
  ALLOW_SECRETS=0
  INCLUDE_LARGE=0
  DRY_RUN=0

  [ $# -ge 1 ] || { usage; emit_error BAD_REQUEST "no subcommand" "see usage above" 2; }
  local sub="$1"; shift

  local positional=()
  while [ $# -gt 0 ]; do
    case "$1" in
      --ttl)            TTL="${2:-}"; shift 2 ;;
      --one-time)       ONE_TIME="true"; shift ;;
      --no-one-time)    ONE_TIME="false"; shift ;;
      --yes)            YES=1; shift ;;
      --allow-secrets)  ALLOW_SECRETS=1; shift ;;
      --include-large)  INCLUDE_LARGE=1; shift ;;
      --dry-run)        DRY_RUN=1; shift ;;
      --server)         SERVER="${2:-}"; SERVER_OVERRIDE="${2:-}"; shift 2 ;;
      -h|--help)        usage; exit 0 ;;
      --*)              emit_error BAD_REQUEST "unknown flag: $1" "see usage" 2 ;;
      *)                positional+=("$1"); shift ;;
    esac
  done

  case "$sub" in
    doctor)
      cmd_doctor ;;
    send)
      [ "${#positional[@]}" -ge 1 ] || emit_error BAD_REQUEST "send needs a <workdir>" "send.sh send <workdir>" 2
      mktmp; cmd_send "${positional[0]}" "$DRY_RUN" ;;
    inspect)
      [ "${#positional[@]}" -ge 1 ] || emit_error BAD_REQUEST "inspect needs a <workdir>" "send.sh inspect <workdir>" 2
      mktmp; cmd_send "${positional[0]}" 1 ;;
    load)
      [ "${#positional[@]}" -ge 1 ] || emit_error BAD_REQUEST "load needs a <url>" "send.sh load <url>" 2
      cmd_load "${positional[0]}" ;;
    load-detail)
      [ "${#positional[@]}" -ge 2 ] || emit_error BAD_REQUEST "load-detail needs <url> <part-id>" "send.sh load-detail <url> <part-id>" 2
      cmd_load_detail "${positional[0]}" "${positional[1]}" ;;
    -h|--help|help)
      usage ;;
    *)
      usage; emit_error BAD_REQUEST "unknown subcommand: $sub" "see usage above" 2 ;;
  esac
}

# Allow `source`-ing for unit tests without executing the dispatcher.
if [ "${SEND_SH_NO_MAIN:-0}" != "1" ]; then
  main "$@"
fi
