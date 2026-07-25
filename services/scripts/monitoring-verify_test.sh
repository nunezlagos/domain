#!/usr/bin/env bash
# Test de la verificación del stack de monitoring (DOMAINSERV-149). Shim de `docker` en
# PATH, sin Docker real.
#
# El bug: install.sh chequeaba explícitamente UN container de los 7 del compose de
# monitoring (domain-crowdsec y su bouncer). Si Loki no levantaba, el deploy terminaba en
# verde, Alloy no tenía dónde escribir y se perdían logs sin que nada avisara.
set -uo pipefail

SCRIPT_DIR="$( cd "$( dirname "${BASH_SOURCE[0]}" )" && pwd )"
WORK="$(mktemp -d)"
trap 'rm -rf "$WORK"' EXIT

SHIM_DIR="$WORK/bin"; mkdir -p "$SHIM_DIR"
export PATH="$SHIM_DIR:$PATH"

# compose de mentira con la misma forma que el real: container_name indentado
cat > "$WORK/monitoring.yml" <<'YML'
services:
  prometheus:
    image: prom/prometheus:v2.55.1
    container_name: domain-prometheus
  grafana:
    image: grafana/grafana:11.3.0
    container_name: domain-grafana
  loki:
    image: grafana/loki:3.3.0
    container_name: domain-loki
  alloy:
    image: grafana/alloy:v1.5.0
    container_name: domain-alloy
volumes:
  loki-data:
YML

# shim: DOCKER_RUNNING = containers corriendo; DOCKER_LOKI_READY=1 → /ready dice ready
cat > "$SHIM_DIR/docker" <<'SHIM'
#!/usr/bin/env bash
case "$1" in
  ps)   printf '%s\n' ${DOCKER_RUNNING:-} ;;
  exec) # docker exec domain-loki wget -qO- http://localhost:3100/ready
        [[ "${DOCKER_LOKI_READY:-0}" == "1" ]] || exit 1
        echo ready ;;
esac
exit 0
SHIM
chmod +x "$SHIM_DIR/docker"
export DOCKER_RUNNING DOCKER_LOKI_READY

source "$SCRIPT_DIR/monitoring-verify.sh"

FAILS=0
check() { # descripción, esperado, actual
  if [[ "$2" == "$3" ]]; then
    printf 'PASS: %s\n' "$1"
  else
    printf 'FAIL: %s (esperado "%s", obtuve "%s")\n' "$1" "$2" "$3"; FAILS=$((FAILS + 1))
  fi
}

# --- la lista sale del compose, no de una constante ---
esperados=$(expected_monitoring_containers "$WORK/monitoring.yml" | tr '\n' ' ')
check "la lista de containers se deriva del compose" \
  "domain-prometheus domain-grafana domain-loki domain-alloy " "$esperados"

# Si mañana se agrega un servicio al stack, la verificación lo cubre SOLA. Ese es el
# punto de derivarla: una lista hardcodeada dejaría un hueco silencioso.
cat >> "$WORK/monitoring.yml" <<'YML'
  tempo:
    image: grafana/tempo:latest
    container_name: domain-tempo
YML
n=$(expected_monitoring_containers "$WORK/monitoring.yml" | wc -l)
check "un servicio nuevo en el compose entra sin tocar el script" 5 "$n"
esperados=$(expected_monitoring_containers "$WORK/monitoring.yml" | tr '\n' ' ')
[[ "$esperados" == *domain-tempo* ]]; check "  y aparece nombrado" 0 $?

check "compose inexistente -> lista vacía, sin ruido" "" \
  "$(expected_monitoring_containers "$WORK/no-existe.yml")"

# --- qué falta ---
DOCKER_RUNNING="domain-prometheus domain-grafana domain-loki domain-alloy domain-tempo"
check "stack completo -> nada faltante" "" \
  "$(missing_monitoring_containers "$WORK/monitoring.yml")"

DOCKER_RUNNING="domain-prometheus domain-grafana domain-alloy domain-tempo"
check "loki caído -> se reporta POR NOMBRE" "domain-loki" \
  "$(missing_monitoring_containers "$WORK/monitoring.yml")"

DOCKER_RUNNING="domain-prometheus"
faltan=$(missing_monitoring_containers "$WORK/monitoring.yml" | tr '\n' ' ')
check "varios caídos -> se reportan todos" \
  "domain-grafana domain-loki domain-alloy domain-tempo " "$faltan"

# el prefijo no alcanza: un container de otro proyecto que empiece igual no cuenta
DOCKER_RUNNING="domain-prometheus-viejo domain-grafana domain-loki domain-alloy domain-tempo"
[[ "$(missing_monitoring_containers "$WORK/monitoring.yml")" == "domain-prometheus" ]]
check "match por nombre EXACTO, no por prefijo" 0 $?

# --- Loki: verificación FUNCIONAL, no de presencia ---
# En prod domain-loki estaba Up y healthy para Docker mientras rechazaba escrituras en
# bucle (timestamp too old). `docker ps` no habría dicho nada: hay que preguntarle a Loki.
DOCKER_LOKI_READY=1
loki_ready 1; check "loki responde ready -> rc 0" 0 $?

DOCKER_LOKI_READY=0
loki_ready 1; check "loki NO responde ready -> rc 1 (aunque el container exista)" 1 $?

# los reintentos no pueden colgar el deploy: con 1 intento devuelve al toque
DOCKER_LOKI_READY=0
inicio=$SECONDS; loki_ready 1 >/dev/null 2>&1; transcurrido=$((SECONDS - inicio))
(( transcurrido <= 3 )); check "el chequeo no cuelga el deploy (1 intento, <=3s)" 0 $?

if [[ "$FAILS" -gt 0 ]]; then
  printf '\n%d test(s) fallaron\n' "$FAILS"; exit 1
fi
printf '\nTodos los tests verdes\n'
