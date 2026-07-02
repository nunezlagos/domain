#!/usr/bin/env bash
# hooks/domain-post-orchestrate.sh — hook PostToolUse de Claude Code.
#
# REQ-54 issue-54.7: cuando el agente interactúa con el orquestador SDD
# (domain_orchestrate / flow_status / phase_result / confirm), marca
# "flow activo" para la sesión. El gate de código (domain-pre-edit.sh) deja
# pasar las ediciones mientras exista esta marca.
#
# Best-effort: exit 0 siempre.
set +e

payload=$(cat)
session_id=$(printf '%s' "$payload" | python3 -c '
import json, sys
try:
    print(json.load(sys.stdin).get("session_id", ""))
except Exception:
    pass
' 2>/dev/null)
[ -n "$session_id" ] || exit 0

state_dir="$HOME/.local/state/domain"
mkdir -p "$state_dir" 2>/dev/null
printf '%s' "$(date -Iseconds)" > "$state_dir/flow-$session_id" 2>/dev/null
exit 0
