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
# Slug del proyecto: dentro de un WORKTREE, basename(cwd) es el nombre del
# worktree y atribuiría la captura a un proyecto fantasma. Resolver el repo
# PRINCIPAL vía git-common-dir (REQ-54 compat worktrees); fallback basename.
proj_dir="${hook_cwd:-$PWD}"
common=$(git -C "$proj_dir" rev-parse --path-format=absolute --git-common-dir 2>/dev/null)
if [ -n "$common" ] && [ "$common" != ".git" ]; then
  proj_dir=$(dirname "$common")
fi
slug=$(basename "$proj_dir" | tr '[:upper:]' '[:lower:]' | tr -cd '[:alnum:]-')

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
' "$slug" 2>/dev/null) || exit 0

domain_mcp_init >/dev/null 2>&1
resp=$(domain_call_tool domain_prompt_capture "$prompt_json" 2>&1)
_cap_rc=$?
# REQ-56 issue-56.2: si la captura falló (curl error o el server devolvió "error"),
# dejar rastro auditable en hook-errors.log en vez de descartarlo en silencio.
if [ "$_cap_rc" -ne 0 ] || printf '%s' "$resp" | grep -q '"error"'; then
  domain_log_hook_error "UserPromptSubmit" "$session_id" "domain_prompt_capture" "rc=$_cap_rc resp=$resp"
fi

# Parsear id (para el hook Stop) + classification (REQ-54 issue-54.4: señal
# de orquestación). El python imprime: línea 1 = prompt_id, línea 2 = JSON de
# additionalContext (o vacía si no corresponde señal).
parsed=$(printf '%s' "$resp" | python3 -c '
import json, sys
pid, ctx = "", ""
try:
    d = json.loads(sys.stdin.read())
    for c in d.get("result", {}).get("content", []):
        t = c.get("text", "")
        try:
            body = json.loads(t)
        except Exception:
            continue
        pid = body.get("id", "")
        cls = body.get("classification") or {}
        action = cls.get("suggested_action", "none")
        if action == "orchestrate":
            mode = cls.get("suggested_mode", "")
            msg = ("domain: este prompt clasifica complexity=%s — es un REQUERIMIENTO. "
                   "PROHIBIDO tocar código sin flow SDD activo (hay gate en PreToolUse). "
                   "Ejecutá domain_orchestrate (mode sugerido: %s) ANTES de implementar. "
                   "En la fase sdd-spec, CONSULTÁ al usuario las dudas/ambigüedades ANTES "
                   "de redactar el spec. Si el usuario ordena explícitamente saltear el "
                   "SDD, obedecé y pedile que apruebe las ediciones que el gate detenga."
                   ) % (cls.get("complexity", "?"), mode or "auto")
            ctx = json.dumps({"hookSpecificOutput": {
                "hookEventName": "UserPromptSubmit", "additionalContext": msg}})
        elif action == "resume":
            msg = ("domain: el proyecto tiene un flow SDD ACTIVO (%s). "
                   "Retomalo con domain_flow_status / domain_orchestrate_status — "
                   "NUNCA re-orquestes un flow nuevo para el mismo trabajo.") % cls.get("active_flow_run_id", "?")
            ctx = json.dumps({"hookSpecificOutput": {
                "hookEventName": "UserPromptSubmit", "additionalContext": msg}})
        break
except Exception:
    pass
print(pid)
print(ctx)
' 2>/dev/null)

pid=$(printf '%s\n' "$parsed" | /usr/bin/head -1)
ctx=$(printf '%s\n' "$parsed" | /usr/bin/tail -n +2)

if [ -n "$pid" ] && [ -n "$session_id" ]; then
  state_dir="$HOME/.local/state/domain"
  mkdir -p "$state_dir" 2>/dev/null
  printf '%s' "$pid" > "$state_dir/turn-$session_id.id" 2>/dev/null
fi

# La señal (si hay) va por stdout: Claude Code la inyecta como additionalContext.
if [ -n "$ctx" ]; then
  domain_log_injection "UserPromptSubmit" "$session_id" "$ctx"
  printf '%s\n' "$ctx"
fi
exit 0
