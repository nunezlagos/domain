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

# 1) go test que pasa (dict sin exit_code, sin señales de fallo) → marker escrito
check "go test OK -> marker escrito" "yes" \
  "$(run "s1" "{\"session_id\":\"s1\",\"tool_input\":{\"command\":\"go test ./...\"},\"tool_response\":$resp_ok}")"

# 2) go test que falla (FAIL en output) → sin marker
check "go test FAIL -> sin marker" "no" \
  "$(run "s2" "{\"session_id\":\"s2\",\"tool_input\":{\"command\":\"go test ./...\"},\"tool_response\":$resp_fail}")"

# 3) comando que no es test → no-op (sin marker)
check "no-test -> sin marker" "no" \
  "$(run "s3" "{\"session_id\":\"s3\",\"tool_input\":{\"command\":\"ls -la\"},\"tool_response\":$resp_ok}")"

# 4) interrupted=true → sin marker aunque no haya FAIL en output
resp_intr='{"stdout":"running...","stderr":"","interrupted":true,"isImage":false}'
check "interrumpido -> sin marker" "no" \
  "$(run "s4" "{\"session_id\":\"s4\",\"tool_input\":{\"command\":\"go test ./...\"},\"tool_response\":$resp_intr}")"

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

if [ "$FAILS" -gt 0 ]; then printf '\n%d test(s) FALLARON\n' "$FAILS"; exit 1; fi
printf '\nTodos los tests pasaron\n'
