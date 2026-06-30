#!/usr/bin/env bash
# scripts/tests/test_path_filters.sh
#
# TDD red para HU 38.12 — valida el path-filter que decide si un cambio
# en el repo debe triggear el auto-deploy. La lista de inclusion vive en
# la spec (services/**, services/Makefile, services/**/docker-compose*.yml,
# .github/workflows/**); la exclusion (docs, openspec, *.md root,
# .gitignore, LICENSE) se valida en test_deploy.sh.
#
# Contrato esperado:
#   source scripts/lib/path_filter.sh
#   should_match <path>  -> exit 0 si el path debe triggear deploy, 1 si no
#
# Uso: ./scripts/tests/test_path_filters.sh

set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
REPO_ROOT="$(cd "$SCRIPT_DIR/../.." && pwd)"
LIB="$REPO_ROOT/scripts/lib/path_filter.sh"

if [[ ! -f "$LIB" ]]; then
  echo "RED: $LIB no existe aun — esperado para green phase" >&2
  exit 1
fi

# shellcheck source=/dev/null
source "$LIB"

failed=0

assert_match() {
  local path="$1"
  if should_match "$path"; then
    echo "PASS: match      $path"
  else
    echo "FAIL: expected match      $path"
    failed=$((failed + 1))
  fi
}

assert_no_match() {
  local path="$1"
  if should_match "$path"; then
    echo "FAIL: expected no-match  $path"
    failed=$((failed + 1))
  else
    echo "PASS: no-match  $path"
  fi
}

# Inclusiones — paths bajo services/ o bajo .github/workflows/
assert_match "services/foo"
assert_match "services/legacy/foo"
assert_match "services/static"
assert_match "services/Makefile"
assert_match "services/domain-mcp/Dockerfile"
assert_match "services/domain-mcp/docker-compose.yml"
assert_match "services/foo/bar/docker-compose.prod.yml"
assert_match ".github/workflows/deploy.yml"

# Exclusiones — todo lo que no toca runtime no debe triggear
assert_no_match "docs/index.md"
assert_no_match "docs/auto-deploy.md"
assert_no_match "openspec/changes/REQ-38/state.yaml"
assert_no_match ".gitignore"
assert_no_match "README.md"
assert_no_match "LICENSE"
assert_no_match "Makefile"

if (( failed > 0 )); then
  echo "RED — $failed tests fallaron"
  exit 1
fi

echo "GREEN — todos los tests pasaron"
exit 0
