#!/usr/bin/env bash
# hooks/domain-hooks-lib.sh — helpers compartidos por los hooks de domain
# (domain-user-prompt.sh, domain-stop.sh). Mismo orden de resolución de
# credenciales que domain-session-start.sh: env > install.env > .env de
# clientes. Todo best-effort: los hooks NUNCA deben bloquear la sesión.

# domain_resolve_env setea $vps_url y $api_key. Retorna 1 si faltan.
domain_resolve_env() {
  vps_url="${DOMAIN_VPS_URL:-}"
  api_key="${DOMAIN_API_KEY:-}"
  if [ -z "$vps_url" ] || [ -z "$api_key" ]; then
    for envf in "$HOME/.config/domain/install.env" "$HOME/.claude/.env" "$HOME/.config/opencode/.env"; do
      [ -r "$envf" ] || continue
      while IFS='=' read -r k v; do
        kk="${k#DOMAIN_}"
        v="${v%\"}"; v="${v#\"}"
        v="${v%\'}"; v="${v#\'}"
        case "$kk" in
          VPS_URL)             [ -z "$vps_url" ] && vps_url="$v" ;;
          MCP_API_KEY|API_KEY) [ -z "$api_key" ] && api_key="$v" ;;
        esac
      done < "$envf"
    done
  fi
  [ -n "$vps_url" ] && [ -n "$api_key" ]
}

# domain_mcp_init manda el initialize (el server sessionless responde igual,
# pero algunos setups lo exigen antes del tools/call).
domain_mcp_init() {
  curl -fsS -m 4 -X POST "${vps_url}/mcp" \
    -H "Authorization: Bearer ${api_key}" \
    -H "Content-Type: application/json" \
    -H "Accept: application/json, text/event-stream" \
    -d '{"jsonrpc":"2.0","id":1,"method":"initialize","params":{"protocolVersion":"2024-11-05","capabilities":{},"clientInfo":{"name":"domain-lifecycle-hook","version":"0.1"}}}'
}

# domain_call_tool <tool_name> <args_json> — imprime la respuesta JSON-RPC.
domain_call_tool() {
  curl -fsS -m 6 -X POST "${vps_url}/mcp" \
    -H "Authorization: Bearer ${api_key}" \
    -H "Content-Type: application/json" \
    -H "Accept: application/json, text/event-stream" \
    -d "{\"jsonrpc\":\"2.0\",\"id\":2,\"method\":\"tools/call\",\"params\":{\"name\":\"$1\",\"arguments\":$2}}"
}
