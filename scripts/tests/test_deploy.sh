#!/usr/bin/env bash
# scripts/tests/test_deploy.sh
#
# TDD red para HU 38.12 — valida la logica central de deploy.sh:
# deteccion de servicios cambiados, validacion del SHA previo, y pull
# fast-forward. Se monta un repo git temporal por test para no tocar el
# repo real.
#
# Contrato esperado (HU 38.12, task 8):
#   source scripts/lib/detect_changed.sh
#   detect_changed_services <prev_sha> [new_sha=HEAD]
#     -> imprime "all" si cambio Makefile/compose raiz, lista de SVC
#        separados por espacio si cambios scoped, vacio si sin cambios;
#        exit 0 siempre.
#   source scripts/lib/prev_sha.sh
#   resolve_prev_sha [<candidate>]
#     -> imprime SHA a usar como prev; si candidate invalido o vacio,
#        cae a HEAD~1; exit 0 siempre que git rev-parse funcione.
#   source scripts/lib/pull_ff.sh
#   pull_ff
#     -> git pull --ff-only contra origin main; exit 0 si ff, exit != 0
#        con mensaje claro en stderr si no es ff; NUNCA hace rollback.
#
# Uso: ./scripts/tests/test_deploy.sh

set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
REPO_ROOT="$(cd "$SCRIPT_DIR/../.." && pwd)"
LIB_DIR="$REPO_ROOT/scripts/lib"

DETECT="$LIB_DIR/detect_changed.sh"
PREV_SHA="$LIB_DIR/prev_sha.sh"
PULL_FF="$LIB_DIR/pull_ff.sh"

for f in "$DETECT" "$PREV_SHA" "$PULL_FF"; do
  if [[ ! -f "$f" ]]; then
    echo "RED: $f no existe aun — esperado para green phase" >&2
    exit 1
  fi
done

# shellcheck source=/dev/null
source "$DETECT"
# shellcheck source=/dev/null
source "$PREV_SHA"
# shellcheck source=/dev/null
source "$PULL_FF"

WORK="$(mktemp -d)"
trap 'rm -rf "$WORK"' EXIT

failed=0

assert_eq() {
  local label="$1" expected="$2" actual="$3"
  if [[ "$expected" == "$actual" ]]; then
    echo "PASS: $label"
  else
    echo "FAIL: $label — expected='$expected' actual='$actual'"
    failed=$((failed + 1))
  fi
}

assert_contains() {
  local label="$1" needle="$2" haystack="$3"
  if [[ " $haystack " == *" $needle "* ]]; then
    echo "PASS: $label"
  else
    echo "FAIL: $label — '$needle' no esta en '$haystack'"
    failed=$((failed + 1))
  fi
}

git_init_with_layout() {
  local dir="$1"
  mkdir -p "$dir/services/domain-mcp" "$dir/services/admin" "$dir/.github/workflows"
  git -C "$dir" init -q -b main
  git -C "$dir" config user.email "t@test"
  git -C "$dir" config user.name "t"
  printf 'placeholder\n' > "$dir/README.md"
  git -C "$dir" add -A
  git -C "$dir" commit -q -m "first"
  printf 'mcp\n' > "$dir/services/domain-mcp/Dockerfile"
  printf 'admin\n' > "$dir/services/admin/Dockerfile"
  printf 'all\n' > "$dir/services/Makefile"
  printf 'noop\n' > "$dir/.github/workflows/ci.yml"
  git -C "$dir" add -A
  git -C "$dir" commit -q -m "second"
}

# Caso (a): sin cambios desde el ultimo deploy -> lista vacia, exit 0
git_init_with_layout "$WORK/repo-a"
SHA_A=$(git -C "$WORK/repo-a" rev-parse HEAD)
out_a=$(cd "$WORK/repo-a" && detect_changed_services "$SHA_A") && rc_a=$? || rc_a=$?
assert_eq "caso (a) sin cambios -> exit 0" "0" "$rc_a"
assert_eq "caso (a) sin cambios -> lista vacia" "" "$out_a"

# Caso (b): cambio en services/domain-mcp/Dockerfile -> solo mcp
git_init_with_layout "$WORK/repo-b"
SHA_B0=$(git -C "$WORK/repo-b" rev-parse HEAD)
printf 'mcp v2\n' > "$WORK/repo-b/services/domain-mcp/Dockerfile"
git -C "$WORK/repo-b" -c user.email=t@t -c user.name=t commit -aq -m "mcp bump"
SHA_B1=$(git -C "$WORK/repo-b" rev-parse HEAD)
out_b=$(cd "$WORK/repo-b" && detect_changed_services "$SHA_B0" "$SHA_B1") && rc_b=$? || rc_b=$?
assert_eq "caso (b) cambio mcp -> exit 0" "0" "$rc_b"
assert_contains "caso (b) cambio mcp -> incluye 'mcp'" "mcp" "$out_b"

# Caso (c): cambio en services/Makefile -> 'all'
git_init_with_layout "$WORK/repo-c"
SHA_C0=$(git -C "$WORK/repo-c" rev-parse HEAD)
printf 'all v2\n' > "$WORK/repo-c/services/Makefile"
git -C "$WORK/repo-c" -c user.email=t@t -c user.name=t commit -aq -m "make bump"
SHA_C1=$(git -C "$WORK/repo-c" rev-parse HEAD)
out_c=$(cd "$WORK/repo-c" && detect_changed_services "$SHA_C0" "$SHA_C1") && rc_c=$? || rc_c=$?
assert_eq "caso (c) cambio Makefile -> exit 0" "0" "$rc_c"
assert_contains "caso (c) cambio Makefile -> incluye 'all'" "all" "$out_c"

# Caso (d): SHA invalido en prev -> fallback a HEAD~1
git_init_with_layout "$WORK/repo-d"
SHA_D_HEAD=$(git -C "$WORK/repo-d" rev-parse HEAD)
SHA_D_PARENT=$(git -C "$WORK/repo-d" rev-parse HEAD~1)
res_d=$(cd "$WORK/repo-d" && resolve_prev_sha "nonexistent-deadbeef") && rc_d=$? || rc_d=$?
assert_eq "caso (d) SHA invalido -> exit 0" "0" "$rc_d"
assert_eq "caso (d) SHA invalido -> cae a HEAD~1" "$SHA_D_PARENT" "$res_d"

# Caso (e): git pull --ff-only con divergencia -> exit != 0, mensaje claro, sin rollback
# Setup: bare origin (main como HEAD), working repo push inicial, luego
# divergen por una rama local y una rama remota via clone-push.
git init --bare -q -b main "$WORK/origin-e.git"
git_init_with_layout "$WORK/repo-e"
git -C "$WORK/repo-e" remote remove origin 2>/dev/null || true
git -C "$WORK/repo-e" remote add origin "$WORK/origin-e.git"
git -C "$WORK/repo-e" push -q origin main
git -C "$WORK/repo-e" branch --set-upstream-to=origin/main main
git -C "$WORK/repo-e" -c user.email=l@l -c user.name=l commit -q --allow-empty -m local-ahead
git clone -q -b main "$WORK/origin-e.git" "$WORK/clone-e"
git -C "$WORK/clone-e" config user.email r@r
git -C "$WORK/clone-e" config user.name r
git -C "$WORK/clone-e" commit -q --allow-empty -m remote-ahead
git -C "$WORK/clone-e" push -q origin main 2>/dev/null
cd "$WORK/repo-e" && pull_ff 2>"$WORK/err_e.txt" || rc_e=$?
cd - >/dev/null
rc_e=${rc_e:-1}
err_e_msg="$(cat "$WORK/err_e.txt" 2>/dev/null || true)"
if (( rc_e != 0 )) && [[ -n "$err_e_msg" ]]; then
  echo "PASS: caso (e) divergencia -> exit != 0 con mensaje"
else
  echo "FAIL: caso (e) divergencia -> rc=$rc_e err='$err_e_msg'"
  failed=$((failed + 1))
fi

if (( failed > 0 )); then
  echo "RED — $failed tests fallaron"
  exit 1
fi

echo "GREEN — todos los tests pasaron"
exit 0
