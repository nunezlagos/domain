#!/usr/bin/env bash
# hooks/domain-stop.sh — hook Stop de Claude Code.
#
# Cierra el turno en domain (domain_turn_complete) usando el prompt_id que
# guardó domain-user-prompt.sh al capturar el prompt. response_chars se
# aproxima con el largo de last_assistant_message (v1: no incluye tool
# calls — suficiente para métricas de uso).
#
# Guardas:
#   - stop_hook_active=true → salir YA (jamás contribuir a un loop de Stop).
#   - Sin id file (capture falló o turno ya cerrado) → no-op.
#   - Best-effort total: exit 0 siempre, nunca bloquea.
set +e

LIB="$(dirname "$0")/domain-hooks-lib.sh"
[ -r "$LIB" ] || exit 0
. "$LIB"
domain_resolve_env || exit 0

payload=$(cat)
eval "$(printf '%s' "$payload" | python3 -c '
import json, sys, shlex
try:
    d = json.load(sys.stdin)
except Exception:
    sys.exit(0)
print("session_id=%s" % shlex.quote(d.get("session_id", "")))
print("stop_active=%s" % shlex.quote("1" if d.get("stop_hook_active") else "0"))
print("resp_chars=%s" % shlex.quote(str(len(d.get("last_assistant_message") or ""))))
' 2>/dev/null)"

[ "$stop_active" = "1" ] && exit 0
[ -n "$session_id" ] || exit 0

id_file="$HOME/.local/state/domain/turn-$session_id.id"
[ -r "$id_file" ] || exit 0
pid=$(cat "$id_file" 2>/dev/null)
rm -f "$id_file" 2>/dev/null
[ -n "$pid" ] || exit 0

args="{\"prompt_id\":\"$pid\",\"response_chars\":${resp_chars:-0}}"
domain_mcp_init >/dev/null 2>&1
domain_call_tool domain_turn_complete "$args" >/dev/null 2>&1
exit 0
