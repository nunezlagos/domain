#!/usr/bin/env bash
# scripts/deploy.sh
#
# Orquestador auto-deploy (HU 38.12). Corre en el self-hosted runner
# dentro del repo. 5 fases con rollback automatico si build/restart/verify
# fallan. Fetch/detect fallan -> exit sin rollback (es del operador).
#
# Uso:
#   ./scripts/deploy.sh                       # deploy real
#   PREV_SHA=<sha> ./scripts/deploy.sh        # SHA explicito
#   PREV_SHA=HEAD~1 ./scripts/deploy.sh --dry-run
#
# Pre-req:
#   - CWD o DEPLOY_REPO_ROOT apunta al repo root
#   - origin/main fetchable (pull_ff)
#   - .git/ escribible para el sentinel DEPLOY_PREV_SHA
#   - .env read-only con DOMAIN_FIELD_ENC_KEY presente

set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
LIB_DIR="$SCRIPT_DIR/lib"
DEPLOY_REPO_ROOT="${DEPLOY_REPO_ROOT:-$(cd "$SCRIPT_DIR/.." && pwd)}"
export RUNTIME_DIR="$DEPLOY_REPO_ROOT"

source "$LIB_DIR/path_filter.sh"
source "$LIB_DIR/detect_changed.sh"
source "$LIB_DIR/prev_sha.sh"
source "$LIB_DIR/pull_ff.sh"
source "$LIB_DIR/orchestrator.sh"

cd "$DEPLOY_REPO_ROOT"

DRY_RUN=0
[[ "${1:-}" == "--dry-run" ]] && DRY_RUN=1

LOG_FILE="${LOG_FILE:-$DEPLOY_REPO_ROOT/.deploy.log}"
ROLLBACK_SENTINEL="$DEPLOY_REPO_ROOT/.git/DEPLOY_PREV_SHA"

fetch_phase() {
  log_phase "fetch: prev_sha=${PREV_SHA:-<unset>}"
  if [[ -z "${PREV_SHA:-}" ]]; then
    PREV_SHA=$(resolve_prev_sha "$(cat "$ROLLBACK_SENTINEL" 2>/dev/null || true)")
  fi
  [[ -n "$PREV_SHA" ]] || { log_phase "fetch: sin prev SHA, abort"; return 1; }
  if (( DRY_RUN )); then
    log_phase "fetch: dry-run skip git reset"
    return 0
  fi
  cp "$ROLLBACK_SENTINEL" "$ROLLBACK_SENTINEL.prev" 2>/dev/null || true
  pull_ff
  echo "$PREV_SHA" > "$ROLLBACK_SENTINEL"
}

detect_phase() {
  local diff_svc
  diff_svc="$(detect_changed_services "$PREV_SHA")"
  if [[ -z "$diff_svc" ]]; then
    log_phase "detect: 0 cambios -> noop"
    NOOP=1
    return 0
  fi
  CHANGED_SVC="$diff_svc"
  log_phase "detect: SVC=$CHANGED_SVC"
}

build_phase() {
  if [[ "${NOOP:-0}" == "1" ]]; then
    log_phase "build: skipped (noop)"; return 0
  fi
  log_phase "build: SVC=$CHANGED_SVC"
  if (( DRY_RUN )); then
    echo "dry-run: make -C services build SVC=$CHANGED_SVC"
    return 0
  fi
  ( cd "$DEPLOY_REPO_ROOT/services" && make build SVC="$CHANGED_SVC" )
}

restart_phase() {
  if [[ "${NOOP:-0}" == "1" ]]; then
    log_phase "restart: skipped (noop)"; return 0
  fi
  log_phase "restart: SVC=$CHANGED_SVC"
  if (( DRY_RUN )); then
    echo "dry-run: make -C services restart SVC=$CHANGED_SVC"
    return 0
  fi
  ( cd "$DEPLOY_REPO_ROOT/services" && make restart SVC="$CHANGED_SVC" )
}

verify_phase() {
  if [[ "${NOOP:-0}" == "1" ]]; then
    log_phase "verify: skipped (noop)"; return 0
  fi
  log_phase "verify: wait-healthy"
  if (( DRY_RUN )); then
    echo "dry-run: make -C services wait-healthy + curl healthz"
    return 0
  fi
  ( cd "$DEPLOY_REPO_ROOT/services" && make wait-healthy )
}

rollback_handler() {
  local rc=$?
  log_phase "ROLLBACK fired rc=$rc"
  if ! should_rollback; then
    log_phase "ROLLBACK skipped (noop o pre-build phase)"
    exit "$rc"
  fi
  if [[ -n "${CHANGED_SVC:-}" ]] && (( ! DRY_RUN )); then
    log_phase "ROLLBACK: restart SVC=all"
    ( cd "$DEPLOY_REPO_ROOT/services" && make restart SVC=all ) \
      || log_phase "ROLLBACK restart SVC=all fallo (continuo)"
  fi
  exit "$rc"
}

main() {
  log_phase "=== deploy start ==="
  validate_env_readonly
  fetch_phase
  detect_phase
  if [[ "${NOOP:-0}" == "1" ]]; then
    log_phase "=== deploy noop done ==="
    return 0
  fi
  build_phase
  restart_phase
  verify_phase
  log_phase "=== deploy done ==="
}

trap rollback_handler ERR
main "$@"
