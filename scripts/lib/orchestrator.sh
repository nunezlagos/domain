#!/usr/bin/env bash
# scripts/lib/orchestrator.sh
#
# Funciones puras del orquestador deploy.sh (HU 38.12). Pensadas para
# source-arse. Proveen log_phase, validate_env_readonly y la decision
# should_rollback. Las fases (fetch/detect/build/restart/verify) viven
# en deploy.sh porque dependen de make y del estado del runner.

log_phase() {
  local phase="$1" ts
  ts="$(date -u +%Y-%m-%dT%H:%M:%SZ)"
  printf '[%s] %s\n' "$ts" "$phase" | tee -a "${LOG_FILE:-.deploy.log}" >&2
}

validate_env_readonly() {
  local env_file="${DEPLOY_ENV_FILE:-${RUNTIME_DIR:-.}/services/.env}"
  [[ -f "$env_file" ]] || { log_phase "validate: .env ausente ($env_file)"; return 1; }
  [[ ! -w "$env_file" ]] || { log_phase "validate: .env writable — abort"; return 1; }
  grep -q '^DOMAIN_FIELD_ENC_KEY=' "$env_file" || { log_phase "validate: DOMAIN_FIELD_ENC_KEY ausente"; return 1; }
  return 0
}

should_rollback() {
  [[ -n "${NOOP:-}" ]] && return 1
  return 0
}
