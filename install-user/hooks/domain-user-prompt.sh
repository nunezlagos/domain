#!/usr/bin/env bash
# hooks/domain-user-prompt.sh — hook UserPromptSubmit de Claude Code.
#
# Captura CADA prompt del usuario en domain (domain_prompt_capture) de forma
# DETERMINISTA (no depende de que el LLM se acuerde) y guarda el prompt_id
# devuelto en ~/.local/state/domain/turn-<session_id>.id para que el hook
# Stop (domain-stop.sh) cierre el turno con domain_turn_complete.
#
# Best-effort TOTAL: cualquier fallo (sin credenciales, VPS caído, JSON raro)
# sale con exit 0 y NO bloquea el prompt del usuario.
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
print("hook_cwd=%s" % shlex.quote(d.get("cwd", "")))
' 2>/dev/null)"

# El prompt puede ser grande y contener cualquier cosa: lo pasamos por stdin,
# nunca por argv ni eval.
prompt_json=$(printf '%s' "$payload" | python3 -c '
import json, sys
try:
    d = json.load(sys.stdin)
except Exception:
    sys.exit(1)
p = d.get("prompt", "")
if not p.strip():
    sys.exit(1)
slug = sys.argv[1] if len(sys.argv) > 1 else ""
print(json.dumps({"content": p, "project_slug": slug, "client_kind": "claude-code"}))
' "$(basename "${hook_cwd:-$PWD}" | tr '[:upper:]' '[:lower:]' | tr -cd '[:alnum:]-')" 2>/dev/null) || exit 0

domain_mcp_init >/dev/null 2>&1
resp=$(domain_call_tool domain_prompt_capture "$prompt_json" 2>/dev/null)
pid=$(printf '%s' "$resp" | python3 -c '
import json, sys
try:
    d = json.loads(sys.stdin.read())
    for c in d.get("result", {}).get("content", []):
        t = c.get("text", "")
        try:
            print(json.loads(t)["id"])
            break
        except Exception:
            pass
except Exception:
    pass
' 2>/dev/null)

if [ -n "$pid" ] && [ -n "$session_id" ]; then
  state_dir="$HOME/.local/state/domain"
  mkdir -p "$state_dir" 2>/dev/null
  printf '%s' "$pid" > "$state_dir/turn-$session_id.id" 2>/dev/null
fi
exit 0
