#!/bin/bash
# Chequea containers; notifica ntfy si down/unhealthy. Estado en /var/run/domain-services-health/.
set -euo pipefail

SCRIPT_DIR="$( cd "$( dirname "${BASH_SOURCE[0]}" )" && pwd )"
ROOT_DIR="$( cd "$SCRIPT_DIR/.." && pwd )"
STATE_DIR="${HEALTH_STATE_DIR:-/var/run/domain-services-health}"
mkdir -p "$STATE_DIR"

[[ -f "$ROOT_DIR/.env" ]] && { set -a; source "$ROOT_DIR/.env"; set +a; }

notify() {
  local title="$1" msg="$2" prio="${3:-default}"
  if [[ -n "${NTFY_TOPIC:-}" ]]; then
    curl -fsS -X POST \
      -H "Title: $title" -H "Priority: $prio" -H "Tags: warning" \
      -d "$msg" \
      "${NTFY_SERVER:-https://ntfy.sh}/$NTFY_TOPIC" >/dev/null 2>&1 || true
  fi
  echo "[$title] $msg"
}

check_service() {
  local svc="$1" container="$2"
  local state_file="$STATE_DIR/$svc.state"

  if ! docker inspect "$container" >/dev/null 2>&1; then
    if [[ ! -f "$state_file" || "$(cat "$state_file")" != "missing" ]]; then
      notify "domain-services DOWN" "Container $container no existe" "urgent"
      echo "missing" > "$state_file"
    fi
    return
  fi

  local status health
  status=$(docker inspect -f '{{.State.Status}}' "$container" 2>/dev/null)
  health=$(docker inspect -f '{{if .State.Health}}{{.State.Health.Status}}{{else}}none{{end}}' "$container" 2>/dev/null)

  if [[ "$status" != "running" ]]; then
    if [[ ! -f "$state_file" || "$(cat "$state_file")" != "$status" ]]; then
      notify "domain-services" "$svc status=$status" "high"
      echo "$status" > "$state_file"
    fi
    return
  fi

  if [[ "$health" == "unhealthy" ]]; then
    if [[ ! -f "$state_file" || "$(cat "$state_file")" != "unhealthy" ]]; then
      notify "domain-services" "$svc unhealthy" "high"
      echo "unhealthy" > "$state_file"
    fi
    return
  fi

  if [[ -f "$state_file" ]]; then
    notify "domain-services" "$svc recuperado" "default"
    rm -f "$state_file"
  fi
}

check_service postgres domain-postgres
check_service minio    domain-minio
