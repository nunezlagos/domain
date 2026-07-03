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

# domain_log_injection <hook> <session_id> <resumen> — REQ-55 issue-55.5.
# additionalContext de los hooks es invisible en la UI de Claude Code (sin log
# nativo). Dejamos rastro auditable en ~/.local/state/domain/injections.log:
# timestamp ISO, hook, session, resumen del contexto inyectado. Best-effort:
# nunca falla ni bloquea la sesión (todo redirigido, siempre "return 0").
domain_log_injection() {
  local dir="$HOME/.local/state/domain"
  mkdir -p "$dir" 2>/dev/null || return 0
  local ts summary
  ts=$(date -Iseconds 2>/dev/null || echo "?")
  # resumen en una línea, recortado a 200 chars (el log es índice, no copia)
  summary=$(printf '%s' "$3" | tr '\n' ' ' | cut -c1-200)
  printf '%s\t%s\t%s\t%s\n' "$ts" "$1" "${2:-?}" "$summary" >> "$dir/injections.log" 2>/dev/null
  return 0
}
