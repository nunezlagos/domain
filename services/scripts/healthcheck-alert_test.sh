#!/usr/bin/env bash
# Test del healthcheck con notificación (DOMAINSERV-236). Aísla los predicados con un
# shim de `docker` y un notify() capturado — sin Docker real y sin pegarle a ntfy.
#
# Lo que motivó la suite, medido en prod el 2026-08-04: el script solo miraba postgres
# y minio, así que domain-mcp podía estar caído sin que nadie se enterara, y con
# NTFY_TOPIC vacío notify() no tenía canal y el guard entero era decorativo.
set -uo pipefail

SCRIPT_DIR="$( cd "$( dirname "${BASH_SOURCE[0]}" )" && pwd )"
SHIM_DIR="$(mktemp -d)"
STATE_DIR="$(mktemp -d)"
trap 'rm -rf "$SHIM_DIR" "$STATE_DIR"' EXIT

# shim de docker: DOCKER_MISSING lista containers que "no existen", DOCKER_UNHEALTHY
# los que corren pero están unhealthy. El resto responde running + healthy.
cat > "$SHIM_DIR/docker" <<'SHIM'
#!/usr/bin/env bash
target="${!#}"
for c in ${DOCKER_MISSING:-}; do [[ "$c" == "$target" ]] && exit 1; done
case "$*" in
  *"inspect -f"*".State.Status"*) echo "running" ;;
  *"inspect -f"*"Health"*)
    for c in ${DOCKER_UNHEALTHY:-}; do
      [[ "$c" == "$target" ]] && { echo "unhealthy"; exit 0; }
    done
    echo "healthy" ;;
esac
exit 0
SHIM
chmod +x "$SHIM_DIR/docker"
export PATH="$SHIM_DIR:$PATH"
export DOCKER_MISSING DOCKER_UNHEALTHY
export HEALTH_STATE_DIR="$STATE_DIR"

# NTFY_TOPIC vacío es el estado real del VPS: el test no debe depender de un canal
NTFY_TOPIC=""
source "$SCRIPT_DIR/healthcheck-alert.sh"

# captura las alertas en vez de mandarlas
ALERTAS=""
notify() { ALERTAS+="$1|$2"$'\n'; }

FAILS=0
check() { # descripción, esperado, actual
  if [[ "$2" == "$3" ]]; then
    printf 'PASS: %s\n' "$1"
  else
    printf 'FAIL: %s (esperado %s, obtuve %s)\n' "$1" "$2" "$3"; FAILS=$((FAILS + 1))
  fi
}
contiene() { [[ "$ALERTAS" == *"$1"* ]] && echo si || echo no; }
reset_alertas() { ALERTAS=""; rm -f "$STATE_DIR"/*.state; }

# domain-mcp es el servicio principal: su ausencia de la lista era el bug
check "domain-mcp está entre los críticos" si \
  "$(printf '%s\n' "${CRITICOS[@]}" | grep -qx "domain-mcp" && echo si || echo no)"
for c in domain-postgres domain-minio domain-admin domain-caddy; do
  check "$c sigue cubierto" si \
    "$(printf '%s\n' "${CRITICOS[@]}" | grep -qx "$c" && echo si || echo no)"
done

reset_alertas
DOCKER_MISSING="domain-mcp"; check_service mcp domain-mcp
check "crítico ausente -> alerta" si "$(contiene "no existe")"

reset_alertas
DOCKER_MISSING=""; DOCKER_UNHEALTHY="domain-mcp"; check_service mcp domain-mcp
check "crítico unhealthy -> alerta" si "$(contiene "unhealthy")"

reset_alertas
DOCKER_UNHEALTHY=""; check_service mcp domain-mcp
check "crítico sano -> sin alerta" no "$(contiene "domain-services")"

# ollama y opencode no existen en toda instalación: su ausencia NO es una caída
reset_alertas
DOCKER_MISSING="domain-ollama"; check_optional ollama domain-ollama
check "opcional ausente -> sin alerta falsa" no "$(contiene "no existe")"

reset_alertas
DOCKER_MISSING=""; DOCKER_UNHEALTHY="domain-ollama"; check_optional ollama domain-ollama
check "opcional presente pero unhealthy -> sí alerta" si "$(contiene "unhealthy")"

# one-shot: migrate y seed terminan en Exited(0), tratarlos como críticos alertaría siempre
for c in domain-migrate domain-seed; do
  check "$c NO está entre los críticos (es one-shot)" no \
    "$(printf '%s\n' "${CRITICOS[@]}" | grep -qx "$c" && echo si || echo no)"
done

# sin canal configurado el guard no puede alertar: tiene que decirlo, no callarse
check "sin NTFY_TOPIC el script avisa que no tiene canal" si \
  "$(NTFY_TOPIC="" canal_configurado && echo no || echo si)"
check "con NTFY_TOPIC el canal cuenta como configurado" si \
  "$(NTFY_TOPIC="domain-alertas" canal_configurado && echo si || echo no)"

if [[ "$FAILS" -gt 0 ]]; then
  printf '\n%d test(s) fallaron\n' "$FAILS"; exit 1
fi
printf '\nTodos los tests verdes\n'
