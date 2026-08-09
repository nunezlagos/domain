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
DEPLOY_LOG="${DEPLOY_LOG:-$AQUI/../../.deploy.log}"
# /run se limpia al reiniciar, y ahí el throttle debe olvidarse: tras un reboot el primer
# fallo vuelve a ser noticia
ESTADO="${ALERTA_ESTADO:-/run/domain-auto-deploy-alert.huella}"
MAX_LINEA=512

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

# La causa vive en el .deploy.log, NO en el journal de la unit. El journal la tiene, pero
# sepultada: sus últimas líneas son siempre las que emite systemd al fallar ("Main process
# exited", "Failed with result"), y un `tail` se queda con esas. Medido: 61 avisos que
# decían "Failed with result exit-code" mientras el log decía "validate: .env writable".
motivo_del_fallo() {
  local del_log
  del_log="$(tail -n 3 "$DEPLOY_LOG" 2>/dev/null | grep -vE '^[[:space:]]*$')"
  if [[ -n "$del_log" ]]; then
    printf '%s\n' "$del_log"
    return 0
  fi
  # sin log en disco queda el journal, y ahí hay que descartar a mano lo que systemd agrega
  journalctl -u "$UNIT" -n 20 --no-pager -o cat 2>/dev/null \
    | grep -vE 'Main process exited|Failed with result|Triggering OnFailure|Scheduled restart|Consumed .* CPU time' \
    | grep -vE '^[[:space:]]*$' | tail -n 3
}

# El destino es un canal público y el cuerpo sale de un log: sin cota, cualquier línea que
# el script escriba a stderr se publica entera. El ADR-4 prometió mandarlas recortadas.
recortar() { cut -c1-"$MAX_LINEA"; }

# OnFailure dispara una vez por ciclo del timer, o sea cada 10 minutos mientras el fallo
# persista: el canal es compartido con healthcheck y backup, y un fallo permanente lo
# vuelve ruido. La huella es del MOTIVO y no del hecho de fallar, así que una causa nueva
# siempre avisa — silenciar un cambio de causa seria perder la informacion que importa.
huella_del_motivo() { printf '%s' "$1" | cksum | cut -d' ' -f1; }

ya_se_aviso_de_esto() {
  [[ -r "$ESTADO" ]] && [[ "$(cat "$ESTADO" 2>/dev/null)" == "$1" ]]
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

razon="$(printf '%s' "${1:-$(motivo_del_fallo)}" | recortar)"
[[ -n "$razon" ]] || razon="sin motivo en $DEPLOY_LOG ni en el journal; revisar con: journalctl -u $UNIT -n 50"

huella="$(huella_del_motivo "$razon")"
if ya_se_aviso_de_esto "$huella"; then
  echo "auto-deploy-alert: mismo motivo que el aviso anterior, no se repite la notificación" >&2
  exit 0
fi

notificar "auto-deploy FALLÓ en $(uname -n)" "$UNIT no completó su ciclo.
$razon"

printf '%s' "$huella" > "$ESTADO" 2>/dev/null || true
