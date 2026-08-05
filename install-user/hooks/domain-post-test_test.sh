#!/usr/bin/env bash
# Test del hook post-test (domain-post-test.sh). Verifica que el marker tests-ok
# se escriba/borre según el resultado inferido de la corrida.
#
# Regresión DOMAINSERV-108: el tool_response de Bash en Claude Code no expone
# exit_code; el fail-closed anterior nunca escribía el marker para `go test`.
set -uo pipefail

SCRIPT_DIR="$( cd "$( dirname "${BASH_SOURCE[0]}" )" && pwd )"
HOOK="$SCRIPT_DIR/domain-post-test.sh"
FAILS=0

# run <session_id> <payload> → corre el hook con HOME aislado; imprime "yes" si
# quedó el marker tests-ok, "no" si no.
run() {
  local sess="$1" payload="$2" home
  home="$(mktemp -d)"
  printf '%s' "$payload" | HOME="$home" bash "$HOOK" >/dev/null 2>&1
  if [ -f "$home/.local/state/domain/tests-ok-$sess" ]; then echo "yes"; else echo "no"; fi
  rm -rf "$home"
}
check() { # descripción, esperado, actual
  if [ "$2" = "$3" ]; then printf 'PASS: %s\n' "$1"
  else printf 'FAIL: %s (esperaba %q, obtuve %q)\n' "$1" "$2" "$3"; FAILS=$((FAILS + 1)); fi
}

resp_ok='{"stdout":"ok  \tpkg\t0.5s\nPASS","stderr":"","interrupted":false,"isImage":false}'
resp_fail='{"stdout":"--- FAIL: TestX\nFAIL\tpkg\t0.2s","stderr":"","interrupted":false,"isImage":false}'

# 1) go test que pasa (dict sin exit_code, sin señales de fallo) → marker escrito.
#    DOMAINSERV-237: el -count=1 es parte del contrato de qué corrida vale como
#    prueba. Este caso decía `go test ./...` y codificaba el contrato VIEJO.
check "go test OK con -count=1 -> marker escrito" "yes" \
  "$(run "s1" "{\"session_id\":\"s1\",\"tool_input\":{\"command\":\"go test -count=1 ./...\"},\"tool_response\":$resp_ok}")"

# 1b) DOMAINSERV-237: las tres formas de salir verde SIN haber evaluado nada. Son
#     el guard del lado escritor: sin ellas, el fix se sostiene por disciplina.
check "go test SIN -count=1 -> no es prueba (puede ser 100% cache)" "no" \
  "$(run "s1b" "{\"session_id\":\"s1b\",\"tool_input\":{\"command\":\"go test ./...\"},\"tool_response\":$resp_ok}")"
check "go test con -run acotado -> no es prueba" "no" \
  "$(run "s1c" "{\"session_id\":\"s1c\",\"tool_input\":{\"command\":\"go test -count=1 -run TestNada ./...\"},\"tool_response\":$resp_ok}")"
check "go test de un paquete suelto (sin ./...) -> no es prueba" "no" \
  "$(run "s1d" "{\"session_id\":\"s1d\",\"tool_input\":{\"command\":\"go test -count=1 ./internal/config/\"},\"tool_response\":$resp_ok}")"

# 2) go test que falla (FAIL en output) → sin marker. Con -count=1 para que el
#    caso siga midiendo la detección del ROJO y no el -count=1 que falta.
check "go test FAIL -> sin marker" "no" \
  "$(run "s2" "{\"session_id\":\"s2\",\"tool_input\":{\"command\":\"go test -count=1 ./...\"},\"tool_response\":$resp_fail}")"

# 3) comando que no es test → no-op (sin marker)
check "no-test -> sin marker" "no" \
  "$(run "s3" "{\"session_id\":\"s3\",\"tool_input\":{\"command\":\"ls -la\"},\"tool_response\":$resp_ok}")"

# 4) interrupted=true → sin marker aunque no haya FAIL en output
resp_intr='{"stdout":"running...","stderr":"","interrupted":true,"isImage":false}'
check "interrumpido -> sin marker" "no" \
  "$(run "s4" "{\"session_id\":\"s4\",\"tool_input\":{\"command\":\"go test -count=1 ./...\"},\"tool_response\":$resp_intr}")"

# 5) DOMAINSERV-111: suites en bash (`bash x_test.sh`, `./x_test.sh`, `make test`).
#    Los hooks de install-user se testean así: sin este patrón el commit-gate
#    quedaba insatisfacible al tocarlos (nunca se escribía el marker).
for c in 'bash install-user/hooks/domain-pre-edit_test.sh' './hooks/x_test.sh' 'make test'; do
  check "suite bash reconocida: $c" "yes" \
    "$(run "s5" "{\"session_id\":\"s5\",\"tool_input\":{\"command\":\"$c\"},\"tool_response\":$resp_ok}")"
done

# 6) DOMAINSERV-111 (contra-prueba): suite bash en rojo → sin marker.
resp_sh_fail='{"stdout":"FAIL: algo\n\n1 test(s) FALLARON","stderr":"","interrupted":false,"isImage":false}'
check "suite bash en rojo -> sin marker" "no" \
  "$(run "s6" "{\"session_id\":\"s6\",\"tool_input\":{\"command\":\"bash x_test.sh\"},\"tool_response\":$resp_sh_fail}")"

# 7) DOMAINSERV-175: los runners nativos de domain-admin, el único servicio Python.
#    Sin estos patrones el gate quedaba insatisfacible al tocarlo: los tests pasaban
#    y el marker no se escribía nunca. Mismo modo de falla que DOMAINSERV-111.
resp_py_ok='{"stdout":"....\n----------------------------------------------------------------------\nRan 4 tests in 0.03s\n\nOK","stderr":"","interrupted":false,"isImage":false}'
for c in 'python3 -m unittest config.tests.test_landing' 'python -m unittest discover' \
         'python manage.py test' 'python3 manage.py test config' \
         '/venv/bin/python3 -m unittest config.tests.test_x'; do
  check "runner Python reconocido: $c" "yes" \
    "$(run "s7" "{\"session_id\":\"s7\",\"tool_input\":{\"command\":\"$c\"},\"tool_response\":$resp_py_ok}")"
done

# 8) DOMAINSERV-175 (contra-prueba): unittest en rojo → sin marker.
#    Las tres variantes del resumen de unittest, porque el patrón las distingue por
#    el número: reconocer el runner sin reconocer su rojo sería peor que no reconocerlo.
for resumen in 'FAILED (failures=1)' 'FAILED (errors=1)' 'FAILED (failures=0, errors=2)'; do
  resp_py_fail="{\"stdout\":\"F...\n$resumen\",\"stderr\":\"\",\"interrupted\":false,\"isImage\":false}"
  check "unittest en rojo -> sin marker: $resumen" "no" \
    "$(run "s8" "{\"session_id\":\"s8\",\"tool_input\":{\"command\":\"python3 -m unittest config.tests.test_x\"},\"tool_response\":$resp_py_fail}")"
done

#    Contra-prueba del número: el patrón NO debe disparar con todo en cero.
resp_py_zero='{"stdout":"..\nOK","stderr":"","interrupted":false,"isImage":false}'
check "unittest en verde -> marker escrito" "yes" \
  "$(run "s8b" "{\"session_id\":\"s8b\",\"tool_input\":{\"command\":\"python3 -m unittest config.tests.test_x\"},\"tool_response\":$resp_py_zero}")"

# 9) DOMAINSERV-175: pedir ayuda NO es correr la suite. Sale exit 0 y sin FAIL en el
#    output, así que sin esta exclusión marcaba verde sin haber ejecutado un solo test.
resp_help='{"stdout":"usage: manage.py test [-h] [--keepdb] [--parallel]","stderr":"","interrupted":false,"isImage":false}'
for c in 'python manage.py test --help' 'go test -h' 'pytest --version' 'python3 -m unittest -h'; do
  check "ayuda/versión NO es corrida: $c" "no" \
    "$(run "s9" "{\"session_id\":\"s9\",\"tool_input\":{\"command\":\"$c\"},\"tool_response\":$resp_help}")"
done

# 10) SABOTAJE del 9: la exclusión no debe comerse una corrida real que mencione
#     un path con "-h" adentro. Si esto fallara, el fix del 9 rompió el caso normal.
check "la exclusión no mata una corrida real con -h en un path" "yes" \
  "$(run "s10" "{\"session_id\":\"s10\",\"tool_input\":{\"command\":\"python3 -m unittest config.tests.test_hook-handler\"},\"tool_response\":$resp_py_ok}")"

if [ "$FAILS" -gt 0 ]; then printf '\n%d test(s) FALLARON\n' "$FAILS"; exit 1; fi
printf '\nTodos los tests pasaron\n'
