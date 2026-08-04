#!/bin/bash
# Chequea containers; notifica ntfy si down/unhealthy. Estado en /var/run/domain-services-health/.
set -euo pipefail

SCRIPT_DIR="$( cd "$( dirname "${BASH_SOURCE[0]}" )" && pwd )"
ROOT_DIR="$( cd "$SCRIPT_DIR/.." && pwd )"
STATE_DIR="${HEALTH_STATE_DIR:-/var/run/domain-services-health}"
mkdir -p "$STATE_DIR"

[[ -f "$ROOT_DIR/.env" ]] && { set -a; source "$ROOT_DIR/.env"; set +a; }

# long-running que install.sh gestiona. domain-migrate y domain-seed quedan FUERA a
# propósito: son one-shot y terminan en Exited(0), así que exigirles running alertaría
# en cada corrida. Hasta DOMAINSERV-236 la lista era solo postgres y minio, o sea que
# domain-mcp —el backend— podía estar caído sin que nadie se enterara
CRITICOS=(domain-postgres domain-minio domain-mcp domain-admin domain-caddy)

# dependen de la config: ollama solo si el embedding provider es ollama, opencode solo
# si el cerebro LLM server-side está prendido. Su ausencia NO es una caída
OPCIONALES=(domain-ollama opencode)

canal_configurado() {
  [[ -n "${NTFY_TOPIC:-}" ]]
}

notify() {
  local title="$1" msg="$2" prio="${3:-default}"
  if canal_configurado; then
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

# un opcional que no existe es una instalación sin ese componente, no una caída
check_optional() {
  local svc="$1" container="$2"
  docker inspect "$container" >/dev/null 2>&1 || return 0
  check_service "$svc" "$container"
}

main() {
  # un guard sin canal de salida no avisa nada: que al menos quede en el journal,
  # que es lo único que queda cuando NTFY_TOPIC está vacío
  canal_configurado || echo "[domain-services] NTFY_TOPIC vacío: los chequeos corren pero NO hay canal de alerta — setealo en el .env"

  for container in "${CRITICOS[@]}"; do
    check_service "${container#domain-}" "$container"
  done
  for container in "${OPCIONALES[@]}"; do
    check_optional "${container#domain-}" "$container"
  done
}

# sourceado desde la suite de test: define las funciones y no ejecuta nada
if [[ "${BASH_SOURCE[0]}" == "${0}" ]]; then
  main
fi
