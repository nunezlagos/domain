#!/usr/bin/env bash
# services/scripts/normalizar-duenos.sh
#
# Deja $INSTALL_DIR con un dueño ÚNICO. DOMAINSERV-258.
#
# Por qué existe: el instalador corre con sudo y git escribe como root, pero los dirs raíz
# son de sysadmin. La mezcla resultante —3363 archivos de root medidos en el VPS— deja a
# sysadmin sin poder crear .git/refs/remotes/origin/*.lock, o sea sin poder operar su propio
# repo. Cada corrida con sudo agravaba la mezcla porque el instalador marcaba safe.directory
# y nunca normalizaba propiedad.
#
# Lo que NO arregla: el auto-deploy. Ese servicio corre como root, así que normalizar a
# sysadmin deja intacta la condición que dispara el "dubious ownership" — el fix de eso es
# Environment=HOME=/root en el unit. Son dos averías distintas con dos fijaciones distintas,
# y confundirlas hace que una tape a la otra.
#
# Uso:
#   source services/scripts/normalizar-duenos.sh
#   normalizar_duenos <install_dir> [dueno=sysadmin]

# Cambia PROPIETARIO y nunca MODO. Un chmod en masa le quitaría a prometheus (nobody), loki
# (10001) y grafana (472) la lectura de sus configs montadas read-only: esos containers leen
# por el permiso de otros, no por dueño, así que el chown es inocuo para ellos y el chmod no.
normalizar_duenos() {
  local dir="${1:-}" dueno="${2:-sysadmin}"
  DUENOS_CAMBIADOS=0

  # la guarda va ANTES del comando y no como validación posterior: un chown -R con la ruta
  # vacía o en / es destructivo e irreversible, y no hay registro de qué archivo era de quién
  if [[ -z "$dir" ]]; then
    echo "normalizar_duenos: INSTALL_DIR vacío, no se toca la propiedad de nada" >&2
    return 1
  fi
  if [[ "$dir" != /* ]]; then
    echo "normalizar_duenos: '$dir' no es ruta absoluta, no se toca la propiedad de nada" >&2
    return 1
  fi
  if [[ "$dir" == "/" ]]; then
    echo "normalizar_duenos: '/' como destino es destructivo, abortando" >&2
    return 1
  fi
  if [[ ! -d "$dir" ]]; then
    echo "normalizar_duenos: '$dir' no existe o no es directorio" >&2
    return 1
  fi

  # -c reporta SOLO lo que cambió: es lo que hace al paso idempotente y medible. Contar lo
  # recorrido en vez de lo cambiado haría que la segunda corrida reporte trabajo inexistente.
  local cambios
  cambios="$(chown -Rc "$dueno" "$dir" 2>/dev/null)" || return 1
  DUENOS_CAMBIADOS="$(printf '%s' "$cambios" | grep -c . || true)"

  return 0
}
