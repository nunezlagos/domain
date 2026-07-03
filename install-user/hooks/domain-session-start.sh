#!/usr/bin/env bash
# hooks/domain-session-start.sh
#
# Hook SessionStart de Claude Code que ejecuta domain_session_bootstrap
# + domain_code_graph + domain_mem_context ANTES del primer prompt del
# usuario. Devuelve el output como additionalContext (system message
# inyectado por Claude Code, que el LLM no puede ignorar).
#
# El LLM recibe el contexto completo (proyecto, work_summary, recent
# observations, code graph) antes de cualquier mensaje del usuario. Asi
# NO tiene excusa de "olvidar" llamar las tools de domain — la info ya
# esta en su system prompt.
#
# Output format: JSON con hookSpecificOutput.additionalContext.
# Non-zero exit o stderr NO bloquea el inicio de sesion (es best-effort).
# Si la API key falta o el MCP no responde, devolvemos un placeholder
# indicando que el bootstrap fallo — pero dejamos arrancar igual.

set +e

# ---------- 1. detectar cwd + git info ----------
cwd="${CWD:-$PWD}"
git_remote=$(git -C "$cwd" remote get-url origin 2>/dev/null || echo "")
git_branch=$(git -C "$cwd" branch --show-current 2>/dev/null || echo "")
git_head=$(git -C "$cwd" rev-parse HEAD 2>/dev/null || echo "")

# ---------- 2. detectar existing_rules_files ----------
rules=()
[ -f "$cwd/AGENTS.md" ]                              && rules+=("AGENTS.md")
[ -f "$cwd/CLAUDE.md" ]                              && rules+=("CLAUDE.md")
[ -f "$cwd/.claude/CLAUDE.md" ]                       && rules+=(".claude/CLAUDE.md")
[ -f "$cwd/.cursorrules" ]                           && rules+=(".cursorrules")
[ -f "$cwd/.windsurfrules" ]                         && rules+=(".windsurfrules")
[ -f "$cwd/.github/copilot-instructions.md" ]         && rules+=(".github/copilot-instructions.md")
[ -d "$cwd/.claude/rules" ]                          && rules+=(".claude/rules/**")
[ -d "$cwd/openspec" ]                               && rules+=("openspec/")
rules_json=$(printf '"%s",' "${rules[@]}")
rules_json="[${rules_json%,}]"

# ---------- 3. resolver VPS URL + API key ----------
# Prioridad: env > ~/.config/domain/install.env > .env de opencode/claude
vps_url="${DOMAIN_VPS_URL:-}"
api_key="${DOMAIN_API_KEY:-}"

# loadEnv del .env global del installer (formato KEY=VAL, una por linea)
# Acepta keys con o sin prefijo DOMAIN_ (DOMAIN_VPS_URL, DOMAIN_MCP_API_KEY, VPS_URL, API_KEY).
# Tambien strip comillas del valor (los .env suelen tener KEY="VAL").
if [ -z "$vps_url" ] || [ -z "$api_key" ]; then
  for envf in "$HOME/.config/domain/install.env" "$HOME/.claude/.env" "$HOME/.config/opencode/.env"; do
    if [ -r "$envf" ]; then
      while IFS='=' read -r k v; do
        # strip prefix DOMAIN_ para matchear
        kk="${k#DOMAIN_}"
        # strip leading/trailing comillas (soportamos " y ')
        v="${v%\"}"; v="${v#\"}"
        v="${v%\'}"; v="${v#\'}"
        case "$kk" in
          VPS_URL)          [ -z "$vps_url" ] && vps_url="$v" ;;
          MCP_API_KEY|API_KEY) [ -z "$api_key" ] && api_key="$v" ;;
        esac
      done < "$envf"
    fi
  done
fi

# fallback: el JSON del cliente MCP, pero SOLO del entry "domain-mcp" (no de
# otros MCPs como context7). Usamos un parser simple.
if [ -z "$vps_url" ] || [ -z "$api_key" ]; then
  for jsonf in "$HOME/.config/opencode/opencode.json" "$HOME/.claude.json"; do
    [ -r "$jsonf" ] || continue
    # extraer URL del entry domain-mcp (admite "mcp":{...} o "mcpServers":{...})
    if [ -z "$vps_url" ]; then
      vps_url=$(python3 -c "
import json,sys
try:
  d=json.load(open('$jsonf'))
  for cont in ('mcp','mcpServers'):
    e=d.get(cont,{}).get('domain-mcp',{})
    u=e.get('url','')
    if u:
      print(u.rstrip('/mcp').rstrip('/'))
      break
except: pass
" 2>/dev/null)
    fi
    if [ -z "$api_key" ]; then
      api_key=$(python3 -c "
import json,sys
try:
  d=json.load(open('$jsonf'))
  for cont in ('mcp','mcpServers'):
    e=d.get(cont,{}).get('domain-mcp',{})
    h=e.get('headers',{}).get('Authorization','')
    if h.startswith('Bearer '):
      print(h[7:])
      break
except: pass
" 2>/dev/null)
    fi
    [ -n "$vps_url" ] && [ -n "$api_key" ] && break
  done
fi

if [ -z "$vps_url" ] || [ -z "$api_key" ]; then
  cat <<'EOF' 2>/dev/null
{"hookSpecificOutput":{"hookEventName":"SessionStart","additionalContext":"⚠ domain bootstrap: VPS_URL o API_KEY no encontrados en ~/.config/domain/install.env, ~/.claude/.env, ~/.config/opencode/.env, ni en los JSONs de los clientes MCP. Re-corre el installer de domain con --api-key para configurar."}}
EOF
  exit 0
fi

# ---------- 3b. resolver usuario real + HOME (necesario para el auto-index) ----------
# En modo normal (Claude Code local) SUDO_USER="" y whoami() da el user local.
# En sudo, SUDO_USER tiene al user real. getent puede no estar disponible,
# asi que tenemos varios fallbacks.
REAL_USER="${SUDO_USER:-$(whoami)}"
if [ -n "$REAL_USER" ] && [ "$REAL_USER" != "root" ] && command -v getent >/dev/null 2>&1; then
  REAL_HOME=$(getent passwd "$REAL_USER" | cut -d: -f6)
elif [ -n "$HOME" ] && [ "$HOME" != "/" ]; then
  REAL_HOME="$HOME"
else
  REAL_HOME=$(eval echo "~$REAL_USER" 2>/dev/null)
fi
if [ -z "$REAL_HOME" ] || [ "$REAL_HOME" = "/" ]; then
  REAL_HOME="$HOME"
fi

# ---------- 4. helper: call MCP tool via curl (streamable-http, initialize + call) ----------
# El MCP server usa Streamable HTTP (content-type application/json o text/event-stream).
# Para simplicidad, mandamos 1 initialize + 1 tool call por separado.

call_mcp_tool() {
  local tool_name="$1"
  local args_json="$2"

  # initialize (el server puede rechazar sin esto, pero sessionless igual responde)
  curl -fsS -X POST "${vps_url}/mcp" \
    -H "Authorization: Bearer ${api_key}" \
    -H "Content-Type: application/json" \
    -H "Accept: application/json, text/event-stream" \
    -d "{\"jsonrpc\":\"2.0\",\"id\":1,\"method\":\"initialize\",\"params\":{\"protocolVersion\":\"2024-11-05\",\"capabilities\":{},\"clientInfo\":{\"name\":\"domain-session-start\",\"version\":\"0.1\"}}}" \
    >/dev/null 2>&1

  # tool call
  curl -fsS -X POST "${vps_url}/mcp" \
    -H "Authorization: Bearer ${api_key}" \
    -H "Content-Type: application/json" \
    -H "Accept: application/json, text/event-stream" \
    -d "{\"jsonrpc\":\"2.0\",\"id\":2,\"method\":\"tools/call\",\"params\":{\"name\":\"${tool_name}\",\"arguments\":${args_json}}}"
}

# ---------- 5. ejecutar las 3 tools de domain ----------
bootstrap_args=$(printf '{"cwd":"%s","git_remote":"%s","git_branch":"%s","git_head":"%s","existing_rules_files":%s}' \
  "$cwd" "$git_remote" "$git_branch" "$git_head" "$rules_json")

bootstrap_out=$(call_mcp_tool "domain_session_bootstrap" "$bootstrap_args" 2>/dev/null)
if [ -z "$bootstrap_out" ]; then
  bootstrap_out="⚠ domain_session_bootstrap falló (VPS no responde o key inválida)"
fi

# Extraer el project_slug del JSON (el output del MCP es JSON con el body dentro
# de un string escapado tipo {"result":{"content":[{"type":"text","text":"{...\"slug\":\"X\"...}"}]}}.
# El grep con '"slug"' no matchea porque el body está escapado (\"slug\").
# Usamos python para parsear el JSON correctamente.
mem_slug=$(echo "$bootstrap_out" | python3 -c "
import sys, json
try:
    d = json.loads(sys.stdin.read())
    r = d.get('result', {})
    for c in r.get('content', []):
        t = c.get('text', '')
        try:
            inner = json.loads(t)
            p = inner.get('project', {})
            if p.get('slug'):
                print(p['slug'])
                break
        except: pass
except: pass
" 2>/dev/null)
[ -z "$mem_slug" ] && mem_slug=$(basename "$cwd" | tr '[:upper:]' '[:lower:]' | tr -cd '[:alnum:]-')
[ -z "$mem_slug" ] && mem_slug="domain-services"

# code graph: si no está built, sugiere build; si está, lee
# Le pasamos project_slug (del bootstrap) para que funcione en el setup remoto.
# Si built=false y existe el script de code graph local + ast-grep, lo corre
# auto (parsea el cwd y sube via domain_code_upload).
code_graph_args=$(printf '{"project_slug":"%s"}' "$mem_slug")
code_graph_out=$(call_mcp_tool "domain_code_graph" "$code_graph_args" 2>/dev/null)
if [ -z "$code_graph_out" ]; then
  code_graph_out="⚠ domain_code_graph falló (VPS no responde o key inválida)"
fi

# ---------- 4b. AUTO-INDEXAR code graph si no existe ----------
# Si el grafo está vacío o solo tiene los 3 del test e2e, parsear el cwd y subir.
# Solo aplica a Claude Code (este hook corre acá, no en opencode).
SCRIPT_PATH="$REAL_HOME/.local/share/domain/scripts/domain-code-graph.sh"
if [ -x "$SCRIPT_PATH" ] && command -v ast-grep >/dev/null 2>&1; then
  total_nodes=$(echo "$code_graph_out" | python3 -c "
import sys, json
try:
    d = json.loads(sys.stdin.read())
    r = d.get('result', {})
    for c in r.get('content', []):
        t = c.get('text','')
        try: print(json.loads(t).get('total_nodes', 0))
        except: pass
except: pass
" 2>/dev/null)
  total_nodes="${total_nodes:-0}"
  if [ "$total_nodes" -le 3 ]; then
    # Parsear y subir (best-effort, no bloquea si falla)
    if [ -d "$cwd" ] && [ -n "$mem_slug" ]; then
      index_out=$("$SCRIPT_PATH" "$cwd" "$mem_slug" 2>&1)
      index_status=$?
      if [ "$index_status" -eq 0 ]; then
        # Re-leer el grafo para reflejar el upload
        code_graph_out=$(call_mcp_tool "domain_code_graph" "$code_graph_args" 2>/dev/null)
      fi
    fi
  fi
fi

# mem context: pide observaciones recientes del proyecto. mem_slug ya se extrajo
# arriba (linea 148, con python que parsea el JSON correctamente). Si por algun
# motivo quedo vacio, fallback a 'domain-services' como slug global.
[ -z "$mem_slug" ] && mem_slug="domain-services"
mem_args=$(printf '{"project_slug":"%s","limit":10}' "$mem_slug")
mem_out=$(call_mcp_tool "domain_mem_context" "$mem_args" 2>/dev/null)
if [ -z "$mem_out" ]; then
  mem_out="⚠ domain_mem_context falló"
fi

# ---------- 6. emitir additionalContext (JSON construido con python para
#            evitar problemas de escapeo de comillas/bash) ----------
# Pasamos las 3 secciones por env vars a python, que arma el JSON final
export HOOK_VPS_URL="$vps_url"
export HOOK_MEM_SLUG="$mem_slug"
export HOOK_BOOTSTRAP_OUT="$bootstrap_out"
export HOOK_CODE_GRAPH_OUT="$code_graph_out"
export HOOK_MEM_OUT="$mem_out"
# REQ-56 issue-56.1: cap de bytes del additionalContext. Sin tope, el payload
# (bootstrap + code_graph + mem_context + reglas) puede saturar la ventana de
# contexto del agente al arrancar. Configurable via DOMAIN_CTX_MAX_BYTES
# (default 12000). Cada sección se trunca de forma determinista preservando su
# encabezado; el bloque de REGLAS DE ARRANQUE nunca se recorta.
export HOOK_CTX_MAX_BYTES="${DOMAIN_CTX_MAX_BYTES:-12000}"

python3 - <<'PYEOF'
import json, os

def _cap(text, limit):
    """Trunca text a limit bytes (utf-8) preservando líneas completas; deja marca."""
    if limit <= 0:
        return text
    raw = text.encode('utf-8')
    if len(raw) <= limit:
        return text
    # cortar por líneas para no partir una línea a la mitad
    kept, used = [], 0
    marker = "\n… [recortado por DOMAIN_CTX_MAX_BYTES]"
    budget = limit - len(marker.encode('utf-8'))
    for line in text.splitlines(keepends=True):
        b = len(line.encode('utf-8'))
        if used + b > budget:
            break
        kept.append(line); used += b
    return ''.join(kept).rstrip() + marker

try:
    max_bytes = int(os.environ.get('HOOK_CTX_MAX_BYTES', '12000'))
except ValueError:
    max_bytes = 12000

# Las 3 secciones dinámicas comparten el presupuesto (reglas van aparte, íntegras).
# Reparto: bootstrap 45%, code_graph 25%, mem_context 30%.
_bootstrap = _cap(os.environ.get('HOOK_BOOTSTRAP_OUT',''), int(max_bytes * 0.45))
_codegraph = _cap(os.environ.get('HOOK_CODE_GRAPH_OUT',''), int(max_bytes * 0.25))
_memctx    = _cap(os.environ.get('HOOK_MEM_OUT',''),        int(max_bytes * 0.30))

ctx = (
    f"🟢 domain MCP ready (auto-cargado por SessionStart hook, "
    f"vps={os.environ.get('HOOK_VPS_URL','?')}, "
    f"slug={os.environ.get('HOOK_MEM_SLUG','?')})\n\n"
    f"## domain_session_bootstrap\n{_bootstrap}\n\n"
    f"## domain_code_graph\n{_codegraph}\n\n"
    f"## domain_mem_context (ultimas 10 obs)\n{_memctx}\n\n"
    f"---\n"
    f"REGLAS DE ARRANQUE (ejecutar EN ORDEN, SIEMPRE, sin excepcion):\n"
    f"R1. PROHIBIDO volver a llamar domain_session_bootstrap, domain_code_graph, "
    f"domain_mem_context. Sus resultados ya estan arriba.\n"
    f"R2. En tu PRIMER mensaje, ANTES de responder al usuario, llamar estas 4 tools "
    f"(en paralelo, con project_slug={os.environ.get('HOOK_MEM_SLUG','?')}):\n"
    f"    a) domain_project_skill_list(project_slug)\n"
    f"    b) domain_project_policy_list(project_slug)\n"
    f"    c) domain_policy_list()\n"
    f"    d) domain_ticket_list(project_slug, limit=5)\n"
    f"R3. Renderizar el bloque YAML exacto que define la prompt first-response. "
    f"skills y policies van con formato '{{P}} proyecto + {{G}} globales'. "
    f"skills: P=items scope=project, G=items scope=global de (a). "
    f"policies: P=total de (b), G=total de (c).\n"
    f"R4. PROHIBIDO omitir las lineas skills y policies. PROHIBIDO parafrasear el "
    f"contexto en prosa en vez del bloque YAML. PROHIBIDO responder al usuario antes "
    f"de renderizar el bloque.\n"
    f"R5. Aplica IGUAL si el mensaje del usuario es trivial, vacio o basura "
    f"(ej: 'asd', 'hola', 'x'). El bloque va SIEMPRE en el primer mensaje.\n"
    f"R6. Despues del bloque, usar el contexto de arriba + domain_* tools para lo que pida el usuario."
)
print(json.dumps({
    "hookSpecificOutput": {
        "hookEventName": "SessionStart",
        "additionalContext": ctx,
    }
}))
PYEOF

# REQ-55 issue-55.5: rastro auditable de la inyección (additionalContext es
# invisible en la UI de Claude Code). Best-effort, nunca bloquea.
_inj_dir="$REAL_HOME/.local/state/domain"
mkdir -p "$_inj_dir" 2>/dev/null && \
  printf '%s\tSessionStart\t%s\tbootstrap+code_graph+mem_context\n' \
    "$(date -Iseconds 2>/dev/null || echo '?')" "${mem_slug:-?}" \
    >> "$_inj_dir/injections.log" 2>/dev/null
exit 0