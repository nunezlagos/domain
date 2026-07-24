#!/usr/bin/env bash
# hooks/domain-post-orchestrate.sh — hook PostToolUse de Claude Code.
#
# REQ-54 issue-54.7: cuando el agente interactúa con el orquestador SDD
# (orchestrate / phase_result / confirm), obtiene un token HMAC firmado
# del server que autoriza ediciones de código. El gate (domain-pre-edit.sh)
# valida este token contra el server en cada edición.
# Al cancelar un flow (flow_cancel), el marcador se limpia.
#
# Marker format (v2 — HMAC token):
#   <base64_token>\t<expires_at_iso8601>
# Si el server no tiene DOMAIN_FLOW_TOKEN_SECRET configurado, degrada al
# formato legacy: <timestamp>\t<flow_run_id> (validación vía flow_status).
#
# Best-effort: exit 0 siempre.
set +e

payload=$(cat)

# DOMAINSERV-71: si python3 no está disponible, no podemos parsear → skip
[ -n "$payload" ] && command -v python3 >/dev/null 2>&1 || exit 0

eval "$(printf '%s' "$payload" | python3 -c '
import json, sys, shlex
try:
    d = json.load(sys.stdin)
except Exception:
    sys.exit(0)
ti = d.get("tool_input") or {}
print("session_id=%s" % shlex.quote(d.get("session_id", "")))
print("tool_name=%s" % shlex.quote(d.get("tool_name", "")))
# extract flow_run_id from orchestrate/phase_result response if present.
# DOMAINSERV-108: Claude Code entrega tool_response como LISTA [{type,text}]
# para tools MCP; otros clientes (o payloads anidados) pueden darlo como dict
# {content:[...]} o como string. Normalizamos a una lista de items antes de
# parsear — el bug previo asumía SOLO dict.get("content") y con la lista real
# de Claude Code fr quedaba "" → nunca se minteaba el token del gate.
fr = ""
mode = ""
resp = d.get("tool_response")
if isinstance(resp, dict):
    items = resp.get("content") or []
elif isinstance(resp, list):
    items = resp
elif isinstance(resp, str):
    items = [{"type": "text", "text": resp}]
else:
    items = []
for c in items:
    if isinstance(c, dict) and c.get("type") == "text":
        try:
            body = json.loads(c["text"])
            fr = body.get("flow_run_id") or body.get("id") or ""
            # DOMAINSERV-108: el modo del flow se persiste como 3er campo del
            # marker para que el commit-gate exente los flows micro de tests.
            mode = body.get("mode", "") or ""
            if fr:
                break
        except (json.JSONDecodeError, TypeError):
            pass
print("flow_run_id=%s" % shlex.quote(fr))
print("flow_mode=%s" % shlex.quote(mode))
' 2>/dev/null)"

[ -n "$session_id" ] || exit 0

state_dir="$HOME/.local/state/domain"
mkdir -p "$state_dir" 2>/dev/null

# flow_cancel → delete marker, exit
case "$tool_name" in
  *flow_cancel*)
    rm -f "$state_dir/flow-$session_id" 2>/dev/null
    exit 0
    ;;
esac

[ -n "$flow_run_id" ] || exit 0

# try to get HMAC token from server
LIB="$(dirname "$0")/domain-hooks-lib.sh"
token=""
expires_at=""
if [ -r "$LIB" ]; then
  . "$LIB"
  domain_resolve_env 2>/dev/null || true
  if [ -n "$vps_url" ] && [ -n "$api_key" ]; then
    domain_mcp_init >/dev/null 2>&1
    resp=$(domain_call_tool domain_flow_grant_token \
      "{\"flow_run_id\":\"$flow_run_id\",\"session_id\":\"$session_id\"}" 2>/dev/null)
    token=$(printf '%s' "$resp" | python3 -c '
import json, sys
try:
    d = json.load(sys.stdin)
    for c in d.get("result",{}).get("content",[]):
        if isinstance(c, dict) and c.get("type") == "text":
            body = json.loads(c["text"])
            print(body.get("token",""))
            break
except Exception:
    pass
' 2>/dev/null)
    expires_at=$(printf '%s' "$resp" | python3 -c '
import json, sys
try:
    d = json.load(sys.stdin)
    for c in d.get("result",{}).get("content",[]):
        if isinstance(c, dict) and c.get("type") == "text":
            body = json.loads(c["text"])
            expires_in = body.get("expires_in", 1800)
            from datetime import datetime, timezone, timedelta
            print((datetime.now(timezone.utc) + timedelta(seconds=expires_in)).isoformat())
            break
except Exception:
    pass
' 2>/dev/null)
  fi
fi

marker="$state_dir/flow-$session_id"
# DOMAINSERV-98: el marker guarda el token HMAC — protegerlo de otros usuarios
# del host (state_dir 700, marker 600) para que no sea legible ni replayable.
mkdir -p "$state_dir" 2>/dev/null
chmod 700 "$state_dir" 2>/dev/null
# DOMAINSERV-108: field3 = modo del flow (ej. "micro"). El commit-gate del
# pre-edit lo lee para eximir los flows micro del requisito de tests. Vacío
# para modos normales (express/lite/full) → sin cambio de comportamiento.
if [ -n "$token" ] && [ -n "$expires_at" ]; then
  printf '%s\t%s\t%s\n' "$token" "$expires_at" "$flow_mode" > "$marker" 2>/dev/null
else
  # fallback: legacy format with flow_run_id (no HMAC configured on server)
  printf '%s\t%s\t%s\n' "$(date -Iseconds)" "$flow_run_id" "$flow_mode" > "$marker" 2>/dev/null
fi
chmod 600 "$marker" 2>/dev/null
exit 0
