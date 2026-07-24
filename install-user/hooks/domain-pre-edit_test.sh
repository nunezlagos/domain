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

# 5) DOMAINSERV-111: Edit/Write sobre archivo NO-código (doc/nota) → sin gate.
#    El gate solo cubre CÓDIGO; hasta ahora la rama Edit/Write no miraba la
#    extensión y bloqueaba un .md, mientras el mismo archivo vía Bash heredoc
#    pasaba (la rama Bash sí filtra por code_ext). Asimetría corregida.
for nc in /tmp/notas.md /tmp/consultas.txt /home/u/.claude/memory/x.md /tmp/salida.log /tmp/data.csv; do
  p="{\"session_id\":\"nc-sess\",\"tool_name\":\"Write\",\"permission_mode\":\"acceptEdits\",\"tool_input\":{\"file_path\":\"$nc\"}}"
  check_not_contains "no-código pasa sin gate: $nc" '"permissionDecision"' "$(run "$p")"
done

# 6) DOMAINSERV-111 (contra-prueba): el código SIGUE gateado en Edit/Write.
for c in /tmp/x.go /repo/svc.py /repo/q.sql /repo/Dockerfile /repo/Makefile /repo/deploy.tf; do
  p="{\"session_id\":\"c-sess\",\"tool_name\":\"Write\",\"permission_mode\":\"acceptEdits\",\"tool_input\":{\"file_path\":\"$c\"}}"
  check_contains "código sigue gateado: $c" '"permissionDecision":"deny"' "$(run "$p")"
done

# 7) DOMAINSERV-111: git-guard NO bloquea subcomandos read-only de stash.
for ro in 'git stash list' 'git stash show -p' 'git stash list --oneline'; do
  p="{\"session_id\":\"ro-sess\",\"tool_name\":\"Bash\",\"permission_mode\":\"acceptEdits\",\"tool_input\":{\"command\":\"$ro\"}}"
  check_not_contains "git-guard permite read-only: $ro" 'git-guard' "$(run "$p")"
done

# 8) DOMAINSERV-111 (contra-prueba): el stash MUTANTE sigue bloqueado.
for mu in 'git stash' 'git stash push -m wip' 'git stash pop' 'git stash drop' 'git -C /repo stash'; do
  p="{\"session_id\":\"mu-sess\",\"tool_name\":\"Bash\",\"permission_mode\":\"acceptEdits\",\"tool_input\":{\"command\":\"$mu\"}}"
  check_contains "git-guard bloquea mutante: $mu" 'git-guard' "$(run "$p")"
done

# 9) DOMAINSERV-111: el git-guard no debe disparar por MENCIONES dentro de un
#    literal (mensaje de commit, heredoc, string). Un mensaje que documenta el
#    guard bloqueaba el propio commit que lo arreglaba.
# payload <session> <command> → JSON bien formado (el escapado a mano corrompe
# el JSON y el hook responde el deny de DOMAINSERV-103, no lo que se testea)
payload() {
  S="$1" C="$2" python3 -c '
import json, os
print(json.dumps({"session_id": os.environ["S"], "tool_name": "Bash",
                  "permission_mode": "acceptEdits",
                  "tool_input": {"command": os.environ["C"]}}))'
}

hd='git commit -F - <<MSG
fix: git stash pop y git reset --hard siguen denegados
MSG'
check_not_contains "git-guard ignora menciones en heredoc" 'git-guard' \
  "$(run "$(payload lit-sess "$hd")")"
check_not_contains "git-guard ignora menciones en -m entrecomillado" 'git-guard' \
  "$(run "$(payload lit-sess 'git commit -m "fix: git reset --hard ya no hace falta"')")"

# 10) DOMAINSERV-111 (contra-prueba): el strip de literales NO debe abrir un
#     bypass. Un intérprete que EJECUTA el literal sigue siendo destructivo.
while IFS= read -r ev; do
  check_contains "bypass por intérprete sigue bloqueado: $ev" 'git-guard' \
    "$(run "$(payload ev-sess "$ev")")"
done <<'EVS'
bash -c "git reset --hard"
sh -c 'git clean -fd'
eval "git restore ."
EVS
check_contains "destructivo real sigue bloqueado" 'git-guard' \
  "$(run "$(payload ev-sess 'git reset --hard HEAD~1')")"
# el literal se reemplaza por un token, NO se borra: vaciarlo dejaría
# `git -C  reset --hard` y el normalizador de opciones se comería "reset"
check_contains "path entrecomillado no abre bypass" 'git-guard' \
  "$(run "$(payload ev-sess 'git -C "/tmp/mi repo" reset --hard')")"

if [[ "$FAILS" -gt 0 ]]; then
  printf '\n%d test(s) FALLARON\n' "$FAILS"; exit 1
fi
printf '\nTodos los tests pasaron\n'
