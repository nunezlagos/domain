#!/usr/bin/env bash
# services/scripts/monitoring-verify.sh — verificación del stack de monitoring
# (DOMAINSERV-149).
#
# EL PROBLEMA
# El compose de monitoring declara 7 containers y install.sh chequeaba explícitamente
# UNO: domain-crowdsec y su bouncer (DOMAINSERV-124). `make monitoring-up` es
# `up -d --wait` y su fallo es un warn —correcto, monitoring es opt-in— pero después
# nadie confirmaba qué quedó arriba. Si Loki no levanta, el deploy termina en verde,
# Alloy no tiene dónde escribir y se pierden logs sin que nada avise. Mismo linaje que
# DOMAINSERV-84: el instalador no debe solo ejecutar, debe confirmar el efecto.
#
# LO QUE ESTO NO ES
# No cubre config stale: eso es config-drift.sh. Y la imagen de Loki no se buildea nunca
# (es la oficial grafana/loki:3.3.0), así que no hay "rebuild" que verificar — ante un
# cambio de config lo que corresponde es recreate, y eso ya lo hace el sweep.
#
# POR QUÉ "Up" NO ALCANZA
# Medido en prod 2026-07-25: domain-loki estaba Up y healthy a ojos de Docker mientras
# rechazaba escrituras en bucle (timestamp too old). Un `docker ps` no habría dicho nada,
# así que a Loki hay que PREGUNTARLE.

LOKI_CONTAINER="${LOKI_CONTAINER:-domain-loki}"

if ! declare -f warn >/dev/null 2>&1; then
  log()  { printf '\033[36m[install]\033[0m %s\n' "$*" >&2; }
  ok()   { log "✓ $*"; }
  warn() { log "! $*"; }
fi

# Containers que el compose declara. Se derivan del PROPIO archivo y no de una lista
# hardcodeada: así, agregar un servicio al stack queda cubierto sin tocar este script —
# una constante desactualizada es justo el hueco silencioso que se quiere evitar.
expected_monitoring_containers() {
  local compose="$1"
  [[ -r "$compose" ]] || return 0
  sed -n 's/^[[:space:]]*container_name:[[:space:]]*\([^[:space:]]*\).*/\1/p' "$compose"
}

# Los declarados que NO están corriendo, uno por línea. Match por nombre EXACTO: un
# container de otro proyecto con nombre parecido no cuenta como presente.
missing_monitoring_containers() {
  local compose="$1" c corriendo
  corriendo=$(docker ps --format '{{.Names}}' 2>/dev/null)
  while IFS= read -r c; do
    [[ -z "$c" ]] && continue
    printf '%s\n' "$corriendo" | grep -qx -- "$c" || printf '%s\n' "$c"
  done < <(expected_monitoring_containers "$compose")
}

# Loki SIRVIENDO, no solo presente. El puerto 3100 no se publica (expose, no ports), así
# que la vía es el exec: la imagen trae busybox wget. Reintentos cortos porque esto corre
# apenas termina el `up` y Loki tarda unos segundos en pasar a ready — con tope, para que
# un Loki que nunca arranca no cuelgue el deploy.
loki_ready() {
  local intentos="${1:-10}" i
  for (( i = 0; i < intentos; i++ )); do
    if docker exec "$LOKI_CONTAINER" wget -qO- http://localhost:3100/ready 2>/dev/null |
         grep -q ready; then
      return 0
    fi
    (( i + 1 < intentos )) && sleep 2
  done
  return 1
}

# Reporte completo para install.sh. Siempre rc 0: monitoring es opt-in y un deploy sano
# no puede volverse rojo porque falte un container de observabilidad.
verify_monitoring_stack() {
  local compose="$1" faltan c
  faltan=$(missing_monitoring_containers "$compose")

  if [[ -n "$faltan" ]]; then
    while IFS= read -r c; do
      [[ -n "$c" ]] && warn "el stack de monitoring declara $c pero NO está corriendo"
    done <<< "$faltan"
    warn "  revisá: docker compose -f $compose --env-file .env ps"
  fi

  if printf '%s\n' "$faltan" | grep -qx -- "$LOKI_CONTAINER"; then
    warn "sin Loki no hay destino para los logs: Alloy los descarta y Grafana no muestra nada"
  elif loki_ready; then
    ok "Loki responde ready (los logs tienen dónde aterrizar)"
  else
    warn "$LOKI_CONTAINER está corriendo pero NO responde /ready: puede estar rechazando ingesta"
    warn "  revisá: docker logs $LOKI_CONTAINER 2>&1 | tail -30"
  fi

  [[ -z "$faltan" ]] && ok "los $(expected_monitoring_containers "$compose" | grep -c .) containers del stack de monitoring están arriba"
  return 0
}
