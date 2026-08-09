#!/usr/bin/env bash
# scripts/lib/detect_changed.sh
#
# Deteccion de servicios cambiados para auto-deploy (HU 38.12).
# Opera sobre el git repo del CWD (el runner siempre corre dentro del repo).
#
# Reglas (de la spec 38.12, task 3):
#   - services/Makefile                            -> 'all'
#   - services/<dir>/docker-compose*.yml (c/any d)  -> 'all'
#   - services/<svc>/...                           -> svc (mapeo abajo)
#   - services/<desconocido>/...                   -> 'all' (defensivo)
#
# Mapeo dir -> SVC (de services/Makefile):
#   domain-mcp   -> mcp
#   domain-admin -> admin
#   postgres, minio, caddy -> mismo nombre
#
# Contrato:
#   detect_changed_services <prev_sha> [<new_sha>=HEAD]
#     -> imprime lista SVC separadas por espacio; "all" si Makefile,
#        compose o dir desconocido; vacio si sin cambios.
#     -> exit 0 siempre.

# rc 0 = es un servicio y su SVC va por stdout; rc 2 = vive bajo services/ pero NO es un
# servicio; rc 1 = desconocido. Los tres son distintos a propósito: colapsar el 2 en el 1
# hacía que un script del host pidiera rebuild de todo el stack.
svc_for_dir() {
  case "$1" in
    domain-mcp) echo mcp ;;
    domain-admin) echo admin ;;
    postgres|minio|caddy) echo "$1" ;;
    # corren en el host bajo systemd: no hay imagen que reconstruir
    scripts|systemd) return 2 ;;
    *) return 1 ;;
  esac
}

detect_changed_services() {
  local prev="$1"
  local new="${2:-HEAD}"

  local diff_out
  diff_out=$(git diff --name-only "$prev..$new" -- services/ 2>/dev/null) || true
  if [[ -z "$diff_out" ]]; then
    return 0
  fi

  local result=""
  while IFS= read -r path; do
    [[ -z "$path" ]] && continue
    case "$path" in
      services/Makefile) echo "all"; return 0 ;;
      services/*/docker-compose*.yml) echo "all"; return 0 ;;
      services/*/*/docker-compose*.yml) echo "all"; return 0 ;;
    esac
    local first="${path#services/}"
    first="${first%%/*}"
    local svc rc
    svc="$(svc_for_dir "$first")"; rc=$?
    case "$rc" in
      0) result="$result $svc" ;;
      # host-only: sigue al próximo path en vez de cortar, porque el mismo rango puede
      # traer ademas un servicio de verdad y ese sí tiene que desplegarse
      2) ;;
      *) echo "all"; return 0 ;;
    esac
  done <<< "$diff_out"

  echo "$result" | tr ' ' '\n' | grep -v '^$' | sort -u | paste -sd ' ' - | sed 's/ $//'
}
