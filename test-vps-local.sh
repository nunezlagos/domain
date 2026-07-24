#!/usr/bin/env bash
# test-vps-local.sh — levanta un container Ubuntu con systemd habilitado y
# corre el install.sh adentro, exponiendo el puerto 80 al host.
# Permite probar el deploy completo SIN tocar tu VPS real.
#
# Uso:
#   ./test-vps-local.sh up         # levanta container vacío + arranca install
#   ./test-vps-local.sh shell      # entra al container
#   ./test-vps-local.sh logs       # tail journalctl del container
#   ./test-vps-local.sh smoke      # curl tests contra http://localhost:8080
#   ./test-vps-local.sh down       # destruye container (limpio)
#   ./test-vps-local.sh reinstall  # destruye + vuelve a correr install desde cero
#
# Requisitos: docker daemon corriendo en tu laptop, ~4 GB RAM libres, ~10 GB disk.

set -euo pipefail

CONTAINER=domain-vps-test
IMAGE=jrei/systemd-ubuntu:24.04
REPO_DIR="$( cd "$( dirname "${BASH_SOURCE[0]}" )" && pwd )"
HOST_PORT=8080
GUEST_PORT=80

BOLD=$'\033[1m'; RESET=$'\033[0m'; GREEN=$'\033[32m'; YELLOW=$'\033[33m'; RED=$'\033[31m'
step() { echo ""; echo "${BOLD}==> $1${RESET}"; }
ok()   { echo "${GREEN}    ✓${RESET} $1"; }
warn() { echo "${YELLOW}    !${RESET} $1"; }
fail() { echo "${RED}    ✗${RESET} $1" >&2; }

cmd_up() {
  if docker inspect "$CONTAINER" >/dev/null 2>&1; then
    fail "Container '$CONTAINER' ya existe. Usá: $0 down (para limpiar) o $0 shell (para entrar)."
    exit 1
  fi

  step "Levantando container Ubuntu 24.04 con systemd"
  docker run -d \
    --name "$CONTAINER" \
    --hostname domain-vps-test \
    --privileged \
    --cgroupns=host \
    -v /sys/fs/cgroup:/sys/fs/cgroup:rw \
    -v "$REPO_DIR:/opt/domain-source:ro" \
    -v domain-vps-test-dockerd:/var/lib/docker \
    -p "$HOST_PORT:$GUEST_PORT" \
    --tmpfs /run --tmpfs /run/lock \
    "$IMAGE"

  ok "container '$CONTAINER' arriba"

  step "Esperando systemd boot (10s)"
  sleep 10
  docker exec "$CONTAINER" systemctl is-system-running --wait 2>/dev/null || true
  ok "systemd corriendo"

  step "Copiando repo al container (ro mount → rw copy en /opt/domain)"
  docker exec "$CONTAINER" bash -c "cp -r /opt/domain-source /opt/domain && chmod -R u+w /opt/domain"
  ok "/opt/domain listo (rw)"

  step "Corriendo install.sh dentro del container"
  docker exec "$CONTAINER" bash -c "cd /opt/domain && ./services/install.sh"
  ok "install.sh terminó (levanta el stack completo vía make up)"

  cat <<HINT

${BOLD}Container 'domain-vps-test' listo en modo install-completed.${RESET}

  Próximos pasos (manuales):
    $0 shell              # entrar al container
    Adentro:
      cd /opt/services
      # Editar /opt/services/.env si querés ajustar passwords reales
      make up             # levanta los 5 servicios anidados (build local)
      make ps             # verificá healthy
      exit
    
    En tu laptop:
      $0 smoke            # curls contra http://localhost:$HOST_PORT/

  Cleanup completo: $0 down

HINT
}

cmd_shell() {
  docker exec -it "$CONTAINER" bash
}

cmd_logs() {
  docker exec "$CONTAINER" journalctl -f --no-pager
}

cmd_smoke() {
  step "Smoke tests contra http://localhost:$HOST_PORT/"
  
  echo "  GET /healthz"
  curl -sS -o /dev/null -w "    HTTP %{http_code}\n" "http://localhost:$HOST_PORT/healthz" || warn "no responde aún"
  
  echo "  GET /"
  curl -sS -o /dev/null -w "    HTTP %{http_code}\n" "http://localhost:$HOST_PORT/" || warn "no responde aún"
  
  echo "  GET /api/v1/orgs (sin auth → esperado 401)"
  curl -sS -o /dev/null -w "    HTTP %{http_code}\n" "http://localhost:$HOST_PORT/api/v1/orgs" || true
  
  echo "  POST /mcp (sin auth → esperado 401)"
  curl -sS -o /dev/null -w "    HTTP %{http_code}\n" -X POST "http://localhost:$HOST_PORT/mcp" || true
  
  echo ""
  echo "  Containers dentro del container:"
  docker exec "$CONTAINER" docker ps --format "    {{.Names}}: {{.Status}}" 2>&1 || warn "docker no disponible adentro todavía"
}

cmd_down() {
  step "Destruyendo container '$CONTAINER'"
  docker rm -f "$CONTAINER" 2>/dev/null || warn "no existía"
  docker volume rm domain-vps-test-dockerd 2>/dev/null || true
  ok "limpio"
}

cmd_reinstall() {
  cmd_down
  cmd_up
}

case "${1:-}" in
  up)         cmd_up ;;
  shell)      cmd_shell ;;
  logs)       cmd_logs ;;
  smoke)      cmd_smoke ;;
  down)       cmd_down ;;
  reinstall)  cmd_reinstall ;;
  ""|help|-h|--help) sed -n '2,18p' "$0" ;;
  *) fail "comando desconocido: $1"; exit 2 ;;
esac
