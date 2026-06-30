#!/usr/bin/env bash
# scripts/lib/path_filter.sh
#
# Path-filter para auto-deploy (HU 38.12). Decide si un path cambiado en
# el repo debe triggear el auto-deploy segun la inclusion list de la spec:
#   - services/**       (todo lo bajo services/, recursivo)
#   - services/Makefile (cubierto por services/**)
#   - services/**/docker-compose*.yml (cubierto por services/**)
#   - .github/workflows/** (workflows propios)
#
# Exclusion (no implementada aqui, vive en test_deploy.sh):
#   - docs/, openspec/, *.md (root), .gitignore, LICENSE
# Si en el futuro se necesita exclusion, se anade aqui y se testea aparte.
#
# Contrato:
#   should_match <path>  -> exit 0 si match, 1 si no
#
# Uso esperado desde deploy.sh, test_path_filters.sh o cualquier script
# que quiera clasificar paths de git diff. Sin dependencias externas.

should_match() {
  local p="${1#./}"
  case "$p" in
    services/*|.github/workflows/*) return 0 ;;
    *) return 1 ;;
  esac
}
