#!/usr/bin/env bash
# services/scripts/auto-deploy-alert.sh
#
# La dispara OnFailure= de domain-auto-deploy.service. DOMAINSERV-258.
#
# Por qué es una unit aparte y no un trap dentro de auto-deploy-check.sh: el fallo que motivó
# esto ocurre en el primer git del ciclo, y uno más temprano —el cd al repo, o el exec del
# lock— pasaría antes de que el trap exista. Un reportero que vive dentro del proceso que
# reporta comparte su destino. OnFailure= vive en systemd y dispara ante cualquier exit != 0,
# timeout o señal.
#
# El auto-deploy falló 4 veces seguidas sin que nadie se enterara: eso es lo que esto cierra.
#
# Uso:
#   auto-deploy-alert.sh [motivo]    # sin motivo, lo saca del journal

set -uo pipefail

UNIT="domain-auto-deploy.service"
AQUI="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"

# bajo systemd el .env llega por EnvironmentFile=; a mano no, así que se busca. Se extraen
# SOLO las dos claves del canal en vez de sourcear el archivo entero, que trae todas las
# credenciales del stack a este proceso sin ninguna necesidad.
leer_canal_del_env() {
  [[ -n "${NTFY_TOPIC:-}" ]] && return 0
  local env_file
  for env_file in "$AQUI/../.env" "$AQUI/../../.env"; do
    [[ -r "$env_file" ]] || continue
    : "${NTFY_TOPIC:=$(sed -n 's/^NTFY_TOPIC=//p' "$env_file" | tail -n1)}"
    : "${NTFY_SERVER:=$(sed -n 's/^NTFY_SERVER=//p' "$env_file" | tail -n1)}"
    [[ -n "${NTFY_TOPIC:-}" ]] && return 0
  done
  return 0
}

# el motivo sale del journal porque OnFailure= no puede pasarlo: systemd solo sabe QUE falló
motivo_del_journal() {
  journalctl -u "$UNIT" -n 5 --no-pager -o cat 2>/dev/null | grep -vE '^[[:space:]]*$' | tail -n 3
}

# el cuerpo NUNCA incluye el topic ni la URL del canal: el topic ES la credencial, y quien lo
# lee puede publicar y suscribirse. Tampoco se vuelca el entorno, que trae el .env entero.
notificar() {
  local titulo="$1" cuerpo="$2"
  if [[ -z "${NTFY_TOPIC:-}" ]]; then
    echo "[$titulo] $cuerpo"
    echo "auto-deploy-alert: sin NTFY_TOPIC, el fallo queda solo en el journal" >&2
    return 0
  fi
  curl -fsS -X POST \
    -H "Title: $titulo" -H "Priority: high" -H "Tags: rotating_light" \
    -d "$cuerpo" \
    "${NTFY_SERVER:-https://ntfy.sh}/$NTFY_TOPIC" >/dev/null 2>&1 || true
  echo "[$titulo] $cuerpo"
}

leer_canal_del_env

razon="${1:-$(motivo_del_journal)}"
[[ -n "$razon" ]] || razon="sin líneas en el journal; revisar con: journalctl -u $UNIT -n 50"

notificar "auto-deploy FALLÓ en $(uname -n)" "$UNIT no completó su ciclo.
$razon"
