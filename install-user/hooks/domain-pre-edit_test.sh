#!/usr/bin/env bash
# Test del gate PreToolUse (domain-pre-edit.sh). Black-box: alimenta payloads por
# stdin con un HOME aislado (sin markers de flow) y verifica el permissionDecision.
# Regresión DOMAINSERV-103: payload no-parseable con python3 presente → fail-closed
# (deny), no fail-open (exit 0 sin decisión).
set -uo pipefail

SCRIPT_DIR="$( cd "$( dirname "${BASH_SOURCE[0]}" )" && pwd )"
HOOK="$SCRIPT_DIR/domain-pre-edit.sh"
FAKE_HOME="$(mktemp -d)"
trap 'rm -rf "$FAKE_HOME"' EXIT

FAILS=0
# run <payload> → imprime stdout del hook con HOME aislado
run() { printf '%s' "$1" | HOME="$FAKE_HOME" bash "$HOOK" 2>/dev/null; }

# normaliza espacios: json.dumps (emit_decision) mete "k": "v"; los echo compactos
# usan "k":"v". Comparamos sin espacios para tolerar ambos formatos.
nospace() { printf '%s' "$1" | tr -d ' '; }

check_contains() { # descripción, esperado-substring, actual
  if nospace "$3" | grep -qF -- "$(nospace "$2")"; then
    printf 'PASS: %s\n' "$1"
  else
    printf 'FAIL: %s (esperaba contener %q, obtuve %q)\n' "$1" "$2" "$3"; FAILS=$((FAILS + 1))
  fi
}
check_not_contains() { # descripción, prohibido-substring, actual
  if nospace "$3" | grep -qF -- "$(nospace "$2")"; then
    printf 'FAIL: %s (NO debía contener %q, obtuve %q)\n' "$1" "$2" "$3"; FAILS=$((FAILS + 1))
  else
    printf 'PASS: %s\n' "$1"
  fi
}

# 1) payload corrupto (no JSON) con python3 presente → fail-closed deny (DOMAINSERV-103)
out="$(run 'esto no es json {{{')"
check_contains "payload corrupto -> deny" '"permissionDecision":"deny"' "$out"
check_contains "payload corrupto -> razón DOMAINSERV-103" 'DOMAINSERV-103' "$out"

# 2) payload JSON válido, Edit sin flow, modo default → ask del gate normal,
#    NO el deny de payload corrupto
valid='{"session_id":"test-sess-103","tool_name":"Edit","permission_mode":"default","tool_input":{"file_path":"/tmp/x.go"}}'
out="$(run "$valid")"
check_contains "JSON válido sin flow (default) -> ask" '"permissionDecision":"ask"' "$out"
check_not_contains "JSON válido no dispara el deny de payload corrupto" 'DOMAINSERV-103' "$out"

# 3) COMMIT-GATE micro (DOMAINSERV-108): flow marker con field3=micro → el
#    commit se exime del requisito de tests (no debe emitir el deny del gate).
mkdir -p "$FAKE_HOME/.local/state/domain"
printf 'faketoken\t2099-01-01T00:00:00+00:00\tmicro\n' > "$FAKE_HOME/.local/state/domain/flow-micro-sess"
commit_micro='{"session_id":"micro-sess","tool_name":"Bash","permission_mode":"acceptEdits","tool_input":{"command":"git commit -m \"fix: texto\""}}'
out="$(run "$commit_micro")"
check_not_contains "commit-gate: micro exento (no exige tests)" 'commit-gate' "$out"

# 4) COMMIT-GATE no-micro (DOMAINSERV-74 intacto): flow marker con field3=express
#    y sin marker tests-ok → deny en modo automático.
printf 'faketoken\t2099-01-01T00:00:00+00:00\texpress\n' > "$FAKE_HOME/.local/state/domain/flow-exp-sess"
commit_exp='{"session_id":"exp-sess","tool_name":"Bash","permission_mode":"acceptEdits","tool_input":{"command":"git commit -m \"feat: x\""}}'
out="$(run "$commit_exp")"
check_contains "commit-gate: express NO exento -> deny sin tests-ok" 'commit-gate' "$out"

if [[ "$FAILS" -gt 0 ]]; then
  printf '\n%d test(s) FALLARON\n' "$FAILS"; exit 1
fi
printf '\nTodos los tests pasaron\n'
