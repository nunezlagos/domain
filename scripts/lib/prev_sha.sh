#!/usr/bin/env bash
# scripts/lib/prev_sha.sh
#
# Resolucion del SHA "previo" para auto-deploy (HU 38.12). Si el candidate
# no es un commit resolvable, cae a HEAD~1. Opera sobre el git repo del CWD.
#
# Contrato:
#   resolve_prev_sha [<candidate>]
#     -> imprime SHA canonizado si candidate es valido; HEAD~1 si no.
#     -> exit 0 mientras git rev-parse funcione.

resolve_prev_sha() {
  local candidate="${1:-}"
  if [[ -n "$candidate" ]] && git rev-parse --verify "$candidate^{commit}" >/dev/null 2>&1; then
    git rev-parse "$candidate^{commit}"
    return 0
  fi
  git rev-parse HEAD~1
}
