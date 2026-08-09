#!/usr/bin/env bash
# domain-hooks-lib_stack_test.sh — DOMAINSERV-265
#
# El commit-gate acepta 8 runners (domain-post-test.sh:53) pero su mensaje de deny
# hardcodea `go test`. En un repo Node el mensaje es imposible de seguir, así que
# quien lo lee concluye "el gate no soporta este stack" y va directo al bypass —
# cuando la suite era ejecutable. Estos tests cubren la función que deriva el
# comando del stack real.

set -uo pipefail

AQUI="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
LIB="$AQUI/domain-hooks-lib.sh"

PASS=0; FAIL=0
pass() { echo "PASS: $1"; PASS=$((PASS + 1)); }
fail() { echo "FAIL: $1"; FAIL=$((FAIL + 1)); }

WORK=$(mktemp -d)
trap 'rm -rf "$WORK"' EXIT

if [ ! -r "$LIB" ]; then
  fail "no existe $LIB"
  echo "RED — la lib no está"
  exit 1
fi
# shellcheck source=/dev/null
source "$LIB"

if ! declare -F domain_test_cmd_sugerido >/dev/null; then
  fail "la lib no define domain_test_cmd_sugerido: fase roja"
  echo "RED — falta la función"
  exit 1
fi

# cada caso arma un repo de mentira con SOLO su manifest y comprueba qué comando sugiere
caso() {
  local nombre="$1" manifest="$2" esperado="$3"
  local dir="$WORK/caso-$nombre"
  mkdir -p "$dir"
  [ -n "$manifest" ] && printf '{}\n' > "$dir/$manifest"
  local obtenido
  obtenido=$(domain_test_cmd_sugerido "$dir")
  if [ "$obtenido" = "$esperado" ]; then
    pass "$nombre ($manifest) -> '$esperado'"
  else
    fail "$nombre ($manifest): esperaba '$esperado', obtuve '$obtenido'"
  fi
}

echo "--- un manifest por stack"
caso go       go.mod         "go test -count=1 ./..."
caso node     package.json   "npm test"
caso python   pyproject.toml "pytest"
caso rust     Cargo.toml     "cargo test"
caso php      composer.json  "phpunit"
caso ruby     Gemfile        "rspec"

echo "--- sin manifest reconocible"
# vacío y NO un default a go test: el mensaje tiene que poder decir "no pude
# determinar cómo correr tests acá" en vez de mandar a correr algo imposible
caso desconocido "" ""

echo "--- sin manifest pero CON suite de shell (DOMAINSERV-254)"
# este repo no tiene manifest en la raíz y sí tiene scripts/tests/test_*.sh. Callar acá
# hacía que el deny declarara la suite "de otro tipo" y mandara al bypass una suite que
# el post-test ya reconoce: las dos puntas del gate divergían.
d="$WORK/shell-suite"; mkdir -p "$d/scripts/tests"; printf '#!/bin/bash\n' > "$d/scripts/tests/test_deploy.sh"
obtenido=$(domain_test_cmd_sugerido "$d")
if [ "$obtenido" = "bash scripts/tests/test_deploy.sh" ]; then
  pass "sin manifest y con scripts/tests/test_*.sh -> sugiere la suite de shell"
else
  fail "suite de shell: esperaba 'bash scripts/tests/test_deploy.sh', obtuve '$obtenido'"
fi

# y el manifest sigue ganando: el fallback es último recurso, no un atajo que tape al stack
d="$WORK/go-y-shell"; mkdir -p "$d/scripts/tests"; printf 'module x\n' > "$d/go.mod"
printf '#!/bin/bash\n' > "$d/scripts/tests/test_x.sh"
obtenido=$(domain_test_cmd_sugerido "$d")
if [ "$obtenido" = "go test -count=1 ./..." ]; then
  pass "con go.mod presente el manifest le gana al fallback de shell"
else
  fail "go+shell: esperaba que ganara Go, obtuve '$obtenido'"
fi

echo "--- monorepo: go.mod y package.json juntos"
d="$WORK/mixto"; mkdir -p "$d"; printf 'module x\n' > "$d/go.mod"; printf '{}\n' > "$d/package.json"
obtenido=$(domain_test_cmd_sugerido "$d")
if [ "$obtenido" = "go test -count=1 ./..." ]; then
  pass "con go.mod presente gana Go (el gate ya trata a Go distinto por alcance)"
else
  fail "monorepo: esperaba que ganara Go, obtuve '$obtenido'"
fi

echo "--- el directorio no existe"
obtenido=$(domain_test_cmd_sugerido "$WORK/no-existe" 2>/dev/null)
if [ -z "$obtenido" ]; then
  pass "directorio inexistente -> vacío, no rompe ni inventa"
else
  fail "directorio inexistente: esperaba vacío, obtuve '$obtenido'"
fi

echo "--- sin argumento"
obtenido=$(domain_test_cmd_sugerido 2>/dev/null)
if [ -z "$obtenido" ]; then
  pass "sin argumento -> vacío"
else
  fail "sin argumento: esperaba vacío, obtuve '$obtenido'"
fi

echo ""
if [ "$FAIL" -gt 0 ]; then
  echo "RED — $FAIL fallaron, $PASS pasaron"
  exit 1
fi
echo "GREEN — los $PASS asserts pasaron"
