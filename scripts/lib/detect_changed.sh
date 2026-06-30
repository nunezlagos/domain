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

svc_for_dir() {
  case "$1" in
    domain-mcp) echo mcp ;;
    domain-admin) echo admin ;;
    postgres|minio|caddy) echo "$1" ;;
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
    local svc
    if svc="$(svc_for_dir "$first")"; then
      result="$result $svc"
    else
      echo "all"; return 0
    fi
  done <<< "$diff_out"

  echo "$result" | tr ' ' '\n' | grep -v '^$' | sort -u | paste -sd ' ' - | sed 's/ $//'
}
