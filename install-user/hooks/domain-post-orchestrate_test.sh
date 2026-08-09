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

# 4) DOMAINSERV-256: un allowed_paths mal tipado NO puede degradar a "sin scope" callado.
#    run() descarta stderr, así que hace falta un gemelo que lo capture: el aviso ES el fix.
run_err() { # <session_id> <payload> → "marker|stderr"
  local sess="$1" payload="$2" home err
  home="$(mktemp -d)"
  err="$(printf '%s' "$payload" | HOME="$home" bash "$HOOK" 2>&1 >/dev/null)"
  local m="$home/.local/state/domain/flow-$sess" marker=""
  [ -f "$m" ] && marker="$(head -1 "$m")"
  rm -rf "$home"
  printf '%s|%s' "$marker" "$err"
}

# 4a) allowed_paths como STRING (el modo de falla medido: un cliente lo serializa como
#     string JSON) → sin marker, y con la razón por stderr. Sin marker el pre-edit deniega:
#     fail-CLOSED, que para un gate de aislamiento es el lado seguro.
res="$(run_err "sess-ap-str" "{\"session_id\":\"sess-ap-str\",\"tool_name\":\"domain_orchestrate\",\"tool_input\":{\"allowed_paths\":\"install-user/**\"},\"tool_response\":[{\"type\":\"text\",\"text\":\"$TXT\"}]}")"
check_field "allowed_paths string: marker NO escrito" "" "${res%%|*}"
if printf '%s' "${res#*|}" | grep -q "DOMAINSERV-256"; then
  printf 'PASS: allowed_paths string: el aviso dice qué pasó\n'
else
  printf 'FAIL: allowed_paths string: no avisó por stderr (obtuve %q)\n' "${res#*|}"; FAILS=$((FAILS + 1))
fi

# 4b) allowed_paths con un item no-string → mismo trato: descartarlo achicaría el
#     territorio declarado sin avisar.
res="$(run_err "sess-ap-mix" "{\"session_id\":\"sess-ap-mix\",\"tool_name\":\"domain_orchestrate\",\"tool_input\":{\"allowed_paths\":[\"services/**\",42]},\"tool_response\":[{\"type\":\"text\",\"text\":\"$TXT\"}]}")"
check_field "allowed_paths con item no-string: marker NO escrito" "" "${res%%|*}"

# 4c) LA OTRA MITAD, y la que evita volver el gate insatisfacible: ausente sigue siendo el
#     flow normal. Si esto se rompe, todo flow que no declara partición deja de editar.
line="$(run "sess-ap-none" "{\"session_id\":\"sess-ap-none\",\"tool_name\":\"domain_orchestrate\",\"tool_input\":{},\"tool_response\":[{\"type\":\"text\",\"text\":\"$TXT\"}]}")"
check_field "sin allowed_paths: marker SÍ escrito (flow normal intacto)" "$FR" "$(printf '%s' "$line" | cut -f2)"

# 4d) y una allowlist BIEN formada tampoco puede bloquearse
line="$(run "sess-ap-ok" "{\"session_id\":\"sess-ap-ok\",\"tool_name\":\"domain_orchestrate\",\"tool_input\":{\"allowed_paths\":[\"services/**\"]},\"tool_response\":[{\"type\":\"text\",\"text\":\"$TXT\"}]}")"
check_field "allowed_paths bien formado: marker SÍ escrito" "$FR" "$(printf '%s' "$line" | cut -f2)"

if [ "$FAILS" -gt 0 ]; then printf '\n%d test(s) FALLARON\n' "$FAILS"; exit 1; fi
printf '\nTodos los tests pasaron\n'
