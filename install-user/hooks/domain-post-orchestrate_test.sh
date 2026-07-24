#!/usr/bin/env bash
# Test del hook PostToolUse (domain-post-orchestrate.sh). Black-box con HOME
# aislado y SIN creds de server → el hook cae al fallback legacy del marker
# (<timestamp>\t<flow_run_id>\t<mode>), lo que permite verificar el parseo del
# tool_response sin depender del VPS.
#
# Regresiones cubiertas (DOMAINSERV-108):
#  - tool_response como LISTA [{type,text}] (shape real de Claude Code) se
#    parsea (antes se asumía SOLO dict{content} → flow_run_id quedaba "").
#  - el modo del flow se persiste como field3 del marker (para el commit-gate).
set -uo pipefail

SCRIPT_DIR="$( cd "$( dirname "${BASH_SOURCE[0]}" )" && pwd )"
HOOK="$SCRIPT_DIR/domain-post-orchestrate.sh"
FR="62fa44a3-f1f5-49a1-97cb-614b10b4cb68"
FAILS=0

# run <session_id> <payload> → escribe el marker en un HOME aislado y lo
# devuelve por stdout (field1<TAB>field2<TAB>field3), o vacío si no se escribió.
run() {
  local sess="$1" payload="$2" home
  home="$(mktemp -d)"
  printf '%s' "$payload" | HOME="$home" bash "$HOOK" >/dev/null 2>&1
  local m="$home/.local/state/domain/flow-$sess"
  [ -f "$m" ] && head -1 "$m"
  rm -rf "$home"
}

check_field() { # descripción, esperado, actual
  if [ "$2" = "$3" ]; then printf 'PASS: %s\n' "$1"
  else printf 'FAIL: %s (esperaba %q, obtuve %q)\n' "$1" "$2" "$3"; FAILS=$((FAILS + 1)); fi
}

TXT="{\\\"flow_run_id\\\": \\\"$FR\\\", \\\"mode\\\": \\\"micro\\\", \\\"status\\\": \\\"pending\\\"}"

# 1) tool_response como LISTA (Claude Code) → parsea flow_run_id y mode
line="$(run "sess-list" "{\"session_id\":\"sess-list\",\"tool_name\":\"domain_flow_status\",\"tool_input\":{},\"tool_response\":[{\"type\":\"text\",\"text\":\"$TXT\"}]}")"
check_field "list-shape: field2=flow_run_id" "$FR" "$(printf '%s' "$line" | cut -f2)"
check_field "list-shape: field3=mode(micro)" "micro" "$(printf '%s' "$line" | cut -f3)"

# 2) tool_response como DICT {content:[...]} → sigue funcionando
line="$(run "sess-dict" "{\"session_id\":\"sess-dict\",\"tool_name\":\"domain_flow_status\",\"tool_input\":{},\"tool_response\":{\"content\":[{\"type\":\"text\",\"text\":\"$TXT\"}]}}")"
check_field "dict-shape: field2=flow_run_id" "$FR" "$(printf '%s' "$line" | cut -f2)"

# 3) sin flow_run_id en el response → no se escribe marker
line="$(run "sess-none" "{\"session_id\":\"sess-none\",\"tool_name\":\"domain_flow_status\",\"tool_input\":{},\"tool_response\":[{\"type\":\"text\",\"text\":\"{}\"}]}")"
check_field "sin flow_run_id: marker NO escrito" "" "$line"

if [ "$FAILS" -gt 0 ]; then printf '\n%d test(s) FALLARON\n' "$FAILS"; exit 1; fi
printf '\nTodos los tests pasaron\n'
