#!/usr/bin/env bash
# DOMAINSERV-124: boton de panico del baneo automatico.
#
# Existe para una sola pregunta: "¿y si nos auto-baneamos?". Respuesta: se corre
# esto. Uso:
#
#   ./unban.sh status            que hay baneado ahora
#   ./unban.sh me                desbanea la IP desde la que estas conectado
#   ./unban.sh ip 1.2.3.4        desbanea una IP concreta
#   ./unban.sh all               desbanea TODO (panico total)
#   ./unban.sh allow-me          agrega tu IP a la whitelist Y la desbanea
#
# `me` y `allow-me` sacan la IP de SSH_CLIENT, o sea de tu propia conexion. Eso
# resuelve el problema de la IP dinamica: no hay que saber cual es, se deduce.
#
# SI PERDISTE EL ACCESO SSH y no podes ni correr esto, ver "rescate" al final.

set -euo pipefail

CS="domain-crowdsec"
WHITELIST_HOST="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)/crowdsec-whitelist.yaml"

rojo()  { printf '\033[31m%s\033[0m\n' "$*"; }
verde() { printf '\033[32m%s\033[0m\n' "$*"; }
info()  { printf '\033[36m%s\033[0m\n' "$*"; }

cscli() { docker exec "$CS" cscli "$@"; }

vive() {
  docker ps --format '{{.Names}}' 2>/dev/null | grep -qx "$CS" || {
    rojo "El container $CS no esta corriendo."
    info "Si no corre, NO hay nada baneando: CrowdSec es quien decide y un bouncer quien aplica."
    exit 1
  }
}

mi_ip() {
  # SSH_CLIENT = "<ip origen> <puerto origen> <ip destino> <puerto destino>"
  if [[ -n "${SSH_CLIENT:-}" ]]; then
    awk '{print $1}' <<<"$SSH_CLIENT"
  elif [[ -n "${SSH_CONNECTION:-}" ]]; then
    awk '{print $1}' <<<"$SSH_CONNECTION"
  else
    rojo "No se pudo deducir tu IP: no estas en una sesion SSH."
    info "Usa: $0 ip <tu-ip>   (mirala con: curl -s ifconfig.me)"
    exit 1
  fi
}

case "${1:-status}" in
  status)
    vive
    info "=== decisiones activas (lo que esta baneado) ==="
    cscli decisions list || true
    echo
    info "=== ultimas alertas (detectado, no necesariamente baneado) ==="
    cscli alerts list --limit 15 || true
    echo
    info "Recorda: sin bouncer instalado, una decision se registra pero NADIE la aplica."
    ;;

  me)
    vive
    ip="$(mi_ip)"
    info "Desbaneando tu IP: $ip"
    cscli decisions delete --ip "$ip" && verde "OK: $ip desbaneada"
    ;;

  ip)
    vive
    [[ -n "${2:-}" ]] || { rojo "Falta la IP. Uso: $0 ip 1.2.3.4"; exit 1; }
    cscli decisions delete --ip "$2" && verde "OK: $2 desbaneada"
    ;;

  all)
    vive
    rojo "PANICO TOTAL: se van a borrar TODAS las decisiones, incluidas las legitimas."
    cscli decisions delete --all && verde "OK: no queda nada baneado"
    ;;

  allow-me)
    vive
    ip="$(mi_ip)"
    [[ -w "$WHITELIST_HOST" ]] || { rojo "No puedo escribir $WHITELIST_HOST (¿falta sudo?)"; exit 1; }

    if grep -q "\"$ip\"" "$WHITELIST_HOST"; then
      info "$ip ya estaba en la whitelist"
    else
      # se inserta despues del marcador, manteniendo la indentacion del bloque ip:
      sed -i "s|    # <-- unban.sh allow-me agrega aca la IP publica del admin|    - \"$ip\"\n    # <-- unban.sh allow-me agrega aca la IP publica del admin|" "$WHITELIST_HOST"
      verde "$ip agregada a la whitelist"
    fi

    # la whitelist solo previene bans NUEVOS: el que ya exista hay que borrarlo
    cscli decisions delete --ip "$ip" 2>/dev/null || true
    info "Recargando CrowdSec para que tome la whitelist..."
    docker restart "$CS" >/dev/null && verde "OK: $ip whitelisteada y desbaneada"
    ;;

  *)
    rojo "Comando desconocido: ${1:-}"
    sed -n '2,20p' "${BASH_SOURCE[0]}"
    exit 1
    ;;
esac

# ── RESCATE si perdiste el acceso SSH ────────────────────────────────────────
#
# 1. El bouncer de Caddy solo bloquea HTTP: SSH sigue entrando. Es el motivo por
#    el que conviene empezar con ese y no con el firewall bouncer.
# 2. Con el firewall bouncer, un ban puede alcanzar el puerto 22. Salida:
#    consola serial / VNC del proveedor (acceso out-of-band, no depende de la
#    red) y desde ahi correr:  docker exec domain-crowdsec cscli decisions delete --all
# 3. Prevencion real: whitelist ANTES de activar cualquier bouncer, y nunca dejar
#    que un bouncer de red actue sobre el puerto de SSH.
