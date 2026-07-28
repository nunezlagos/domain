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

domain_state="$HOME/.local/state/domain"
# La limpieza va ANTES de domain_resolve_env: es higiene de disco y no necesita
# credenciales. Estaba detrás del early-exit por creds, así que en una máquina
# sin resolver nunca corría, al revés de lo que decía este comentario.

# tests-ok muere con el turno: el commit-gate exige una corrida que cubra el
# estado ACTUAL del código (DOMAINSERV-74).
rm -f "$domain_state/tests-ok-$session_id" 2>/dev/null

# DOMAINSERV-181: el marker de flow NO se borra por turno. Un flow SDD dura
# varios turnos por diseño —el modo hybrid pausa para el humano, o sea que
# GARANTIZA cruzar de turno— y borrarlo dejaba al agente sin poder editar con el
# flow todavía running. No es un agujero de seguridad: el marker local no es la
# autoridad, el gate revalida el token contra el server en cada edición.
# Los huérfanos que DOMAINSERV-78 quería evitar se podan por antigüedad: ninguna
# sesión sigue viva 24h después, y el token HMAC ya venció mucho antes.
find "$domain_state" -maxdepth 1 -name 'flow-*' -type f -mmin +1440 -delete 2>/dev/null

# el turn_complete solo procede si hay turn-id (prompt capturado)
id_file="$domain_state/turn-$session_id.id"
[ -r "$id_file" ] || exit 0
pid=$(cat "$id_file" 2>/dev/null)
rm -f "$id_file" 2>/dev/null
[ -n "$pid" ] || exit 0

# las credenciales recién hacen falta acá: el turn_complete es lo único que sale
# a la red
domain_resolve_env || exit 0

args="{\"prompt_id\":\"$pid\",\"response_chars\":${resp_chars:-0}}"
domain_mcp_init >/dev/null 2>&1
# REQ-56 issue-56.2: no silenciar el fallo. Capturamos la respuesta/errores y, si
# el turn_complete no salió bien, dejamos rastro en hook-errors.log. Sigue siendo
# best-effort (exit 0), pero ahora el fallo es auditable en vez de invisible.
resp=$(domain_call_tool domain_turn_complete "$args" 2>&1)
rc=$?
if [ "$rc" -ne 0 ] || printf '%s' "$resp" | grep -q '"error"'; then
  domain_log_hook_error "Stop" "$session_id" "domain_turn_complete" "rc=$rc resp=$resp"
fi
exit 0
