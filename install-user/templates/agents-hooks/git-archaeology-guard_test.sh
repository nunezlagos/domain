#!/usr/bin/env bash
# Test del PreToolUse de git-archaeology. Black-box: alimenta payloads por stdin y
# verifica el permissionDecision.
#
# El guard no tenía ninguna suite, y esa ausencia es la causa de que su hueco viviera
# sin que nadie lo viera: el template declara "cero riesgo de mutación, reforzado por un
# hook PreToolUse propio" y esa garantía nunca se ejercitó. La policy
# delegar-lecturas-multiples pide justo lo contrario — verificar la allowlist con un
# intento fallido REAL, no por omisión en la lista.
#
# Caso que lo motivó: `git log/show --output=<path>` ESCRIBE un archivo sin usar el
# operador de redirección del shell, así que la rama de `[<>]` no lo ve y el allowlist
# positivo lo acepta por empezar con "git log". Es una primitiva de escritura arbitraria
# en el agente de menor riesgo declarado del catálogo: alcanza para sobrescribir
# ~/.claude/settings.json o los .md de los otros agentes y aflojar SUS allowlists.
set -uo pipefail

SCRIPT_DIR="$( cd "$( dirname "${BASH_SOURCE[0]}" )" && pwd )"
GUARD="$SCRIPT_DIR/git-archaeology-guard.sh"

FAILS=0

# run <command> → stdout del guard con ese command en el payload PreToolUse
run() {
  python3 -c '
import json, sys
print(json.dumps({"tool_name": "Bash", "tool_input": {"command": sys.argv[1]}}))
' "$1" | bash "$GUARD" 2>/dev/null
}

# deniega <descripción> <command>
deniega() {
  local desc="$1" cmd="$2" out
  out="$(run "$cmd")"
  if printf '%s' "$out" | grep -q '"permissionDecision": *"deny"'; then
    echo "ok   deny  — $desc"
  else
    echo "FAIL deny  — $desc"
    echo "     comando: $cmd"
    echo "     salida:  ${out:-(vacía = allow)}"
    FAILS=$((FAILS + 1))
  fi
}

# permite <descripción> <command>
permite() {
  local desc="$1" cmd="$2" out
  out="$(run "$cmd")"
  if [ -z "$out" ]; then
    echo "ok   allow — $desc"
  else
    echo "FAIL allow — $desc"
    echo "     comando: $cmd"
    echo "     salida:  $out"
    FAILS=$((FAILS + 1))
  fi
}

echo "== escritura por --output (el hueco de esta suite) =="
deniega "--output= en git log"            'git log -1 -p --output=/tmp/pwned.txt'
deniega "--output= en git show"           'git show HEAD --output=/tmp/pwned.txt'
deniega "--output con espacio"            'git log --output /tmp/pwned.txt'
deniega "--output al final sin valor"     'git log --output='
deniega "--output sobre un .md de agente" 'git show HEAD --output=/home/u/.claude/agents/ticket-triage.md'

echo
echo "== contra-prueba: opciones que EMPIEZAN con --output y NO escriben =="
# sin esto, un regex de "--output" a secas bloquearía lectura legítima y el deny de
# arriba pasaría igual sin discriminar nada
permite "--output-indicator-new" 'git log -p --output-indicator-new=@ --oneline'
permite "--output-indicator-old" 'git show --output-indicator-old=- HEAD'

echo
echo "== lo que el guard ya cubría (regresión) =="
permite "git log simple"        'git log --oneline -5'
permite "git show simple"       'git show HEAD'
permite "git blame simple"      'git blame README.md'
deniega "otro subcomando"       'git commit -m x'
deniega "redirección de shell"  'git log > /tmp/x'
deniega "encadenado"            'git log && rm -rf /tmp/x'
deniega "subshell"              'git log $(whoami)'
deniega "-C a otro repo"        'git -C /otro log'
deniega "GIT_DIR"               'GIT_DIR=/otro/.git git log'
deniega "no es git"             'cat /etc/passwd'

echo
echo "== fail-closed =="
deniega "payload no parseable" "$(printf 'no-json')"

echo
if [ "$FAILS" -eq 0 ]; then
  echo "PASS — todos los casos"
  exit 0
fi
echo "FAIL — $FAILS caso(s)"
exit 1
