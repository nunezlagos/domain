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
eval "$(printf '%s' "$payload" | python3 -c '
import json, sys, shlex
try:
    d = json.load(sys.stdin)
except Exception:
    sys.exit(0)
ti = d.get("tool_input") or {}
print("session_id=%s" % shlex.quote(d.get("session_id", "")))
print("tool_name=%s" % shlex.quote(d.get("tool_name", "")))
# extract flow_run_id from orchestrate/phase_result response if present
fr = ""
resp = d.get("tool_response")
if isinstance(resp, dict):
    for c in (resp.get("content") or []):
        if isinstance(c, dict) and c.get("type") == "text":
            try:
                body = json.loads(c["text"])
                fr = body.get("flow_run_id") or body.get("id") or ""
            except (json.JSONDecodeError, TypeError):
                pass
print("flow_run_id=%s" % shlex.quote(fr))
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
if [ -n "$token" ] && [ -n "$expires_at" ]; then
  printf '%s\t%s\n' "$token" "$expires_at" > "$marker" 2>/dev/null
else
  # fallback: legacy format with flow_run_id (no HMAC configured on server)
  printf '%s\t%s\n' "$(date -Iseconds)" "$flow_run_id" > "$marker" 2>/dev/null
fi
exit 0
