#!/usr/bin/env bash
#
# Archcore Send — one-command deploy.
#
# Pulls the target branch, points the server at the canonical domain, and rolls
# the sendd + Caddy stack with `docker compose up -d --build`, then waits for the
# instance to report healthy. Idempotent — safe to re-run.
#
# Two modes, same script:
#   • remote — set DEPLOY_SSH (e.g. root@1.2.3.4 or an ssh Host alias); the build
#              and compose run on the box over SSH. Run this from your laptop.
#   • local  — leave DEPLOY_SSH empty and run it ON the box itself.
#
# Config (later wins): deploy/deploy.env  →  CLI flags.
# Copy deploy/deploy.env.example to deploy/deploy.env and fill it in.
#
# Usage:
#   ./deploy/deploy.sh [--domain D] [--branch B] [--ssh TARGET]
#                      [--remote-path PATH] [--skip-tests] [--no-build]
#                      [--dry-run] [-h|--help]

set -euo pipefail
[[ -n "${BASH_VERSION:-}" ]] || { printf 'run with bash\n' >&2; exit 1; }

# --- logging (human text → stderr) ------------------------------------------
if [[ -t 2 ]]; then C_B=$'\033[1m'; C_G=$'\033[32m'; C_Y=$'\033[33m'; C_R=$'\033[31m'; C_0=$'\033[0m'
else C_B=''; C_G=''; C_Y=''; C_R=''; C_0=''; fi
info()       { printf '%s•%s %s\n'  "$C_B" "$C_0" "$*" >&2; }
success()    { printf '%s✓%s %s\n'  "$C_G" "$C_0" "$*" >&2; }
warn()       { printf '%s!%s %s\n'  "$C_Y" "$C_0" "$*" >&2; }
error_exit() { printf '%s✗%s %s\n'  "$C_R" "$C_0" "$*" >&2; exit 1; }
need_cmd()   { command -v "$1" >/dev/null 2>&1 || error_exit "missing required command: $1"; }

# --- locate repo + defaults --------------------------------------------------
SCRIPT_DIR="$(cd -- "$(dirname -- "${BASH_SOURCE[0]}")" && pwd)"
REPO_DIR="$(cd -- "$SCRIPT_DIR/.." && pwd)"

DEPLOY_SSH="${DEPLOY_SSH:-}"
DEPLOY_REMOTE_PATH="${DEPLOY_REMOTE_PATH:-}"
DEPLOY_BRANCH="${DEPLOY_BRANCH:-main}"
SEND_DOMAIN="${SEND_DOMAIN:-send.archcore.ai}"
RUN_TESTS=1
DO_BUILD=1
DRY_RUN=0

# deploy.env is the primary config; CLI flags below override it.
ENV_FILE="$SCRIPT_DIR/deploy.env"
if [[ -f "$ENV_FILE" ]]; then
  # shellcheck source=/dev/null
  source "$ENV_FILE"
fi

# --- flags -------------------------------------------------------------------
usage() { sed -n '2,30p' "${BASH_SOURCE[0]}" | sed 's/^# \{0,1\}//'; exit "${1:-0}"; }
while [[ $# -gt 0 ]]; do
  case "$1" in
    --domain)      SEND_DOMAIN="${2:?--domain needs a value}"; shift 2;;
    --branch)      DEPLOY_BRANCH="${2:?--branch needs a value}"; shift 2;;
    --ssh)         DEPLOY_SSH="${2:?--ssh needs a value}"; shift 2;;
    --remote-path) DEPLOY_REMOTE_PATH="${2:?--remote-path needs a value}"; shift 2;;
    --skip-tests)  RUN_TESTS=0; shift;;
    --no-build)    DO_BUILD=0; shift;;
    --dry-run)     DRY_RUN=1; shift;;
    -h|--help)     usage 0;;
    *)             error_exit "unknown argument: $1 (try --help)";;
  esac
done

SEND_PUBLIC_URL="https://${SEND_DOMAIN}"
[[ -n "$DEPLOY_SSH" ]] && MODE="remote" || MODE="local"

# --- preflight ---------------------------------------------------------------
need_cmd git
need_cmd curl
[[ "$MODE" == "remote" ]] && need_cmd ssh

if [[ "$MODE" == "remote" && -z "$DEPLOY_REMOTE_PATH" ]]; then
  error_exit "remote mode needs DEPLOY_REMOTE_PATH (the repo path on the box). Set it in deploy/deploy.env or pass --remote-path."
fi

# Don't deploy code that isn't pushed — the box pulls from origin.
if git -C "$REPO_DIR" rev-parse --verify "origin/$DEPLOY_BRANCH" >/dev/null 2>&1; then
  unpushed="$(git -C "$REPO_DIR" rev-list --count "origin/$DEPLOY_BRANCH..$DEPLOY_BRANCH" 2>/dev/null || echo 0)"
  [[ "$unpushed" == "0" ]] || warn "$unpushed local commit(s) on '$DEPLOY_BRANCH' are not pushed to origin — the box pulls origin, so they won't deploy."
fi

# DNS sanity (soft — Caddy needs the A/AAAA record before first cert issuance).
if command -v dig >/dev/null 2>&1; then
  if [[ -z "$(dig +short "$SEND_DOMAIN" 2>/dev/null)" ]]; then
    warn "$SEND_DOMAIN does not resolve from here — Caddy's Let's Encrypt challenge needs it pointing at the box before first start."
  fi
fi

info "Deploy plan"
printf '    mode    : %s\n'  "$MODE"        >&2
[[ "$MODE" == "remote" ]] && printf '    ssh     : %s\n' "$DEPLOY_SSH" >&2
[[ "$MODE" == "remote" ]] && printf '    path    : %s\n' "$DEPLOY_REMOTE_PATH" >&2
printf '    branch  : %s\n'  "$DEPLOY_BRANCH" >&2
printf '    domain  : %s\n'  "$SEND_DOMAIN"   >&2
printf '    build   : %s\n'  "$([[ $DO_BUILD == 1 ]] && echo yes || echo 'no (image reuse)')" >&2
printf '    tests   : %s\n'  "$([[ $RUN_TESTS == 1 ]] && echo 'run make test-all' || echo skipped)" >&2

# --- local verification gate -------------------------------------------------
if [[ "$RUN_TESTS" == 1 ]]; then
  command -v make >/dev/null 2>&1 || error_exit "make not found (needed for the test gate; pass --skip-tests to bypass)"
  info "running test gate: make test-all"
  if [[ "$DRY_RUN" == 1 ]]; then
    info "[dry-run] would run: make -C $REPO_DIR test-all"
  else
    make -C "$REPO_DIR" test-all >&2 || error_exit "tests failed — aborting deploy"
    success "tests passed"
  fi
fi

# --- remote/deploy step (runs on the box, via ssh or locally) ----------------
# Positional args: $1=repo  $2=branch  $3=domain  $4=public_url  $5=build(1/0)
read -r -d '' REMOTE_SCRIPT <<'REMOTE' || true
set -euo pipefail
repo="$1"; branch="$2"; domain="$3"; public_url="$4"; do_build="$5"

log() { printf '  [box] %s\n' "$*" >&2; }

[ -d "$repo/.git" ] || { printf 'no git repo at %s\n' "$repo" >&2; exit 1; }
cd "$repo"

log "fetching origin/$branch"
git fetch --prune origin "$branch"
git checkout "$branch"
git merge --ff-only "origin/$branch" || {
  printf 'cannot fast-forward %s to origin/%s (diverged) — resolve on the box\n' "$branch" "$branch" >&2
  exit 1
}
log "now at $(git rev-parse --short HEAD) — $(git log -1 --pretty=%s)"

cd deploy
[ -f .env ] || { cp .env.example .env; log "created deploy/.env from .env.example"; }

set_env() { # key value
  if grep -q "^$1=" .env; then sed -i "s|^$1=.*|$1=$2|" .env
  else printf '%s=%s\n' "$1" "$2" >> .env; fi
}
set_env SEND_DOMAIN     "$domain"
set_env SEND_PUBLIC_URL "$public_url"
log "deploy/.env → SEND_DOMAIN=$domain"

# docker compose v2 (plugin) or v1 (standalone)
if docker compose version >/dev/null 2>&1; then dc() { docker compose "$@"; }
elif command -v docker-compose >/dev/null 2>&1; then dc() { docker-compose "$@"; }
else printf 'docker compose not found on the box\n' >&2; exit 1; fi

VERSION="$(git -C .. describe --tags --always 2>/dev/null || echo dev)"
COMMIT="$(git -C .. rev-parse --short HEAD)"
export VERSION COMMIT

if [ "$do_build" = "1" ]; then
  log "docker compose up -d --build (VERSION=$VERSION COMMIT=$COMMIT)"
  dc up -d --build
else
  log "docker compose up -d (no rebuild)"
  dc up -d
fi
dc ps
docker image prune -f >/dev/null 2>&1 || true
log "deployed $VERSION ($COMMIT)"
REMOTE

run_deploy() {
  if [[ "$MODE" == "remote" ]]; then
    printf '%s' "$REMOTE_SCRIPT" | ssh "$DEPLOY_SSH" bash -s -- \
      "$DEPLOY_REMOTE_PATH" "$DEPLOY_BRANCH" "$SEND_DOMAIN" "$SEND_PUBLIC_URL" "$DO_BUILD"
  else
    printf '%s' "$REMOTE_SCRIPT" | bash -s -- \
      "$REPO_DIR" "$DEPLOY_BRANCH" "$SEND_DOMAIN" "$SEND_PUBLIC_URL" "$DO_BUILD"
  fi
}

if [[ "$DRY_RUN" == 1 ]]; then
  info "[dry-run] would run the deploy step in $MODE mode against $SEND_DOMAIN — stopping."
  exit 0
fi

info "rolling the stack ($MODE)"
run_deploy
success "stack rolled"

# --- post-deploy health check ------------------------------------------------
health_url="${SEND_PUBLIC_URL}/healthz"
info "waiting for $health_url (cert issuance can take a few seconds on first run)"
for _ in $(seq 1 30); do
  if curl -fsS --max-time 5 "$health_url" >/dev/null 2>&1; then
    success "healthz OK — $SEND_DOMAIN is live"
    info "next: round-trip test → /send on one machine, /send --load on another"
    exit 0
  fi
  sleep 3
done
error_exit "healthz did not come up within ~90s. Check 'docker compose logs caddy' (TLS/ACME) and 'docker compose logs sendd' on the box."
