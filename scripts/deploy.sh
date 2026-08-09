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

# Caddy es el UNICO borde HTTP del host y ademas sirve otros sitios. Se valida contra la
# imagen con plugins ANTES de tocar nada: si el Caddyfile no adapta, el deploy aborta con
# el stack anterior intacto y sirviendo. Validarlo despues del restart seria diagnosticar
# con el sitio ya caido. Mismo pre-flight que services/install.sh:406-418.
preflight_phase() {
  if [[ "${NOOP:-0}" == "1" ]]; then
    log_phase "preflight: skipped (noop)"; return 0
  fi
  log_phase "preflight: Caddyfile contra la imagen con plugins"
  if (( DRY_RUN )); then
    echo "dry-run: docker compose build caddy + caddy validate"
    return 0
  fi
  local caddyfile="$DEPLOY_REPO_ROOT/services/caddy/Caddyfile"
  [[ -f "$caddyfile" ]] || { log_phase "preflight: sin Caddyfile en $caddyfile"; return 0; }
  ( cd "$DEPLOY_REPO_ROOT/services" && docker compose -f caddy/docker-compose.yml --env-file .env build ) \
    || { log_phase "preflight: no se pudo buildear la imagen de Caddy — abortando con el stack intacto"; return 1; }
  docker run --rm -v "$caddyfile:/etc/caddy/Caddyfile:ro" \
    -e CROWDSEC_BOUNCER_API_KEY=preflight \
    domain-caddy:plugins caddy validate --config /etc/caddy/Caddyfile >/dev/null 2>&1 \
    || { log_phase "preflight: el Caddyfile NO valida — abortando con el stack intacto"; return 1; }
}

# El init-container domain-migrate ABORTA si la BD tiene datos y no hubo pg_dump, y su
# mensaje dice "Redeploy via services/install.sh": fue escrito para ESE camino. El
# auto-deploy usa este script, asi que sin esto nunca podia satisfacerlo — y el 2026-08-09
# se llevo puestos a mcp, admin y caddy por depends_on.
backup_phase() {
  if [[ "${NOOP:-0}" == "1" ]]; then
    log_phase "backup: skipped (noop, no se levanta nada)"; return 0
  fi
  if (( DRY_RUN )); then
    echo "dry-run: backup.sh pre-migracion"
    return 0
  fi
  local guard="$DEPLOY_REPO_ROOT/services/scripts/pg-backup-guard.sh"
  # shellcheck source=/dev/null
  [[ -f "$guard" ]] && source "$guard"
  if declare -F should_run_pre_migration_backup >/dev/null && ! should_run_pre_migration_backup; then
    log_phase "backup: sin volumen de datos, se omite"
    return 0
  fi
  log_phase "backup: pre-migracion"
  "$DEPLOY_REPO_ROOT/services/scripts/backup.sh" \
    || { log_phase "backup: FALLO — abortando el deploy para no migrar sin respaldo"; return 1; }
  # la marca que el guard de migrate espera
  export DOMAIN_BACKUP_DONE=1
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
  ( cd "$DEPLOY_REPO_ROOT/services" && make wait-healthy ) || return 1
  # wait-healthy mira los healthchecks de docker, y un container puede estar healthy con
  # el sitio sin responder: el 2026-08-09 quedaron containers healthy y el endpoint en 000
  # porque caddy directamente no existia. Solo un GET real lo distingue.
  log_phase "verify: self-check HTTP"
  local code
  for _ in $(seq 1 60); do
    code=$(curl -fsS -o /dev/null -w '%{http_code}' -m 5 http://localhost/healthz 2>/dev/null || true)
    [[ "$code" == "200" ]] && { log_phase "verify: /healthz 200"; return 0; }
    sleep 2
  done
  log_phase "verify: /healthz no respondio 200 (ultimo code=${code:-sin-respuesta})"
  return 1
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
  validate_env_no_laxo
  fetch_phase
  detect_phase
  if [[ "${NOOP:-0}" == "1" ]]; then
    log_phase "=== deploy noop done ==="
    return 0
  fi
  build_phase
  # el preflight va antes del backup a proposito: es el chequeo barato y aborta sin haber
  # pagado un pg_dump de decenas de MB
  preflight_phase
  backup_phase
  restart_phase
  verify_phase
  log_phase "=== deploy done ==="
}

trap rollback_handler ERR
main "$@"
