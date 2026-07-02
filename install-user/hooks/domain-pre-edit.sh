#!/usr/bin/env bash
# hooks/domain-pre-edit.sh — hook PreToolUse de Claude Code (Edit|Write|
# NotebookEdit|Bash).
#
# REQ-54 issue-54.7: gate determinista SDD-para-código. TODO código pasa por
# SDD (decisión del usuario, sin exención trivial):
#
#   - Si la sesión tiene flow SDD activo (marca de domain-post-orchestrate.sh)
#     → la edición pasa.
#   - Sin flow: en modo normal (default/plan) → permissionDecision "ask" (el
#     HUMANO decide en el diálogo); en modos automáticos (acceptEdits/
#     bypassPermissions/auto) → "deny" con razón: el agente es FORZADO a
#     orquestar primero.
#   - Bash solo se gatea si el comando PARECE edición de código (sed -i, tee,
#     patch, git apply, redirect a archivo de código). Limitación conocida:
#     heurística con falsos negativos posibles — la policy explícita cubre el
#     resto (auditable).
#
# Best-effort en fallos de parseo: permitir (exit 0) antes que romper la sesión.
set +e

payload=$(cat)
eval "$(printf '%s' "$payload" | python3 -c '
import json, sys, shlex
try:
    d = json.load(sys.stdin)
except Exception:
    sys.exit(0)
print("session_id=%s" % shlex.quote(d.get("session_id", "")))
print("tool_name=%s" % shlex.quote(d.get("tool_name", "")))
print("perm_mode=%s" % shlex.quote(d.get("permission_mode", "default")))
' 2>/dev/null)"
[ -n "$session_id" ] || exit 0

# Flow SDD activo en la sesión → pasa todo.
[ -r "$HOME/.local/state/domain/flow-$session_id" ] && exit 0

# Bash: solo gatear si el comando parece edición de código.
if [ "$tool_name" = "Bash" ]; then
  is_edit=$(printf '%s' "$payload" | python3 -c '
import json, re, sys
try:
    cmd = json.load(sys.stdin).get("tool_input", {}).get("command", "")
except Exception:
    sys.exit(0)
code_ext = r"\.(go|py|ts|tsx|js|jsx|sql|sh|bash|rs|java|kt|php|rb|c|cc|cpp|h|hpp|vue|svelte)\b"
patterns = [
    r"\bsed\s+(-\w*\s+)*-i",             # sed -i (in-place)
    r"\btee\s+(-a\s+)?\S*" + code_ext,   # tee a archivo de código
    r"\bpatch\b",
    r"\bgit\s+apply\b",
    r">>?\s*\S*" + code_ext,             # redirect a archivo de código
]
if any(re.search(p, cmd) for p in patterns):
    print("yes")
' 2>/dev/null)
  [ "$is_edit" = "yes" ] || exit 0
fi

# Sin flow y tocando código → decidir según el modo de permisos.
case "$perm_mode" in
  default|plan) decision="ask" ;;
  *)            decision="deny" ;;
esac

reason="domain (issue-54.7): edición de código SIN flow SDD activo. TODO código pasa por SDD: ejecutá domain_orchestrate (el spec se arma en la fase sdd-spec — consultá dudas al usuario ANTES de redactarlo). Si el usuario ordenó explícitamente saltear el SDD, pedile que apruebe esta edición."

python3 -c '
import json, sys
print(json.dumps({"hookSpecificOutput": {
    "hookEventName": "PreToolUse",
    "permissionDecision": sys.argv[1],
    "permissionDecisionReason": sys.argv[2],
}}))
' "$decision" "$reason" 2>/dev/null
exit 0
