#!/usr/bin/env bash
# install-user.sh — configura clientes MCP de la laptop para apuntar al VPS de Domain.
#
# Filosofía: cero archivos de instrucciones/memoria/rules sueltos. El
# protocolo de uso vive en BD como policy `agent-protocol` (editable,
# versionada) y el MCP server lo inyecta a cada cliente en el `initialize`
# handshake vía el campo estándar `instructions`. En disco solo quedan:
#
#   1. ~/.claude/skills/domain/SKILL.md   ← bootstrap on-demand
#   2. ~/.claude/agents/domain-memory.md   ← subagent read-only
#
# Más el config del MCP server por cliente (mcp_servers.json o equivalente)
# — eso es transport config, indispensable, no es "memoria".
#
# Para opencode, los skill/agent globales se exponen vía symlink (1 sola
# copia en disco). Cursor/Cline/Continue/Claude Desktop reciben SOLO el
# config del server; el protocolo les llega por el handshake MCP.
#
# Uso:
#   ./install-user.sh                          # interactive
#   ./install-user.sh --url http://1.2.3.4 \   # no-interactive
#                     --email u@x.cl \
#                     --api-key domk_live_...
#   ./install-user.sh --uninstall              # restaura backups y borra skill/agent
#   ./install-user.sh --dry-run                # solo detecta

set -euo pipefail

# ----------------------------------------------------------------------------
# Estilo
# ----------------------------------------------------------------------------
BOLD=$'\033[1m'; RESET=$'\033[0m'
GREEN=$'\033[32m'; YELLOW=$'\033[33m'; RED=$'\033[31m'; DIM=$'\033[2m'
step() { echo ""; echo "${BOLD}==> $1${RESET}"; }
ok()   { echo "${GREEN}    ✓${RESET} $1"; }
warn() { echo "${YELLOW}    !${RESET} $1"; }
fail() { echo "${RED}    ✗${RESET} $1" >&2; }
info() { echo "${DIM}    ·${RESET} $1"; }

# ----------------------------------------------------------------------------
# Args
# ----------------------------------------------------------------------------
VPS_URL=""
USER_EMAIL=""
API_KEY=""
UNINSTALL=0
DRY_RUN=0

while [[ $# -gt 0 ]]; do
  case "$1" in
    --url)        VPS_URL="$2"; shift 2 ;;
    --email)      USER_EMAIL="$2"; shift 2 ;;
    --api-key)    API_KEY="$2"; shift 2 ;;
    --uninstall)  UNINSTALL=1; shift ;;
    --dry-run)    DRY_RUN=1; shift ;;
    -h|--help)    sed -n '2,25p' "$0"; exit 0 ;;
    *) fail "flag desconocida: $1"; exit 2 ;;
  esac
done

# ----------------------------------------------------------------------------
# Helpers
# ----------------------------------------------------------------------------
SCRIPT_DIR="$( cd "$( dirname "${BASH_SOURCE[0]}" )" && pwd )"
TEMPLATES_DIR="$SCRIPT_DIR/templates"

# .env global: persiste VPS_URL/EMAIL entre re-ejecuciones para que el
# usuario no tenga que tipearlos cada vez. API_KEY NO se persiste — solo
# vive en los configs de cada cliente (encriptados o no según cliente).
GLOBAL_ENV="$HOME/.config/domain/install.env"

load_global_env() {
  [[ -f "$GLOBAL_ENV" ]] || return 0
  # shellcheck disable=SC1090
  source "$GLOBAL_ENV"
  [[ -z "$VPS_URL"    && -n "${DOMAIN_VPS_URL:-}" ]]     && VPS_URL="$DOMAIN_VPS_URL"
  [[ -z "$USER_EMAIL" && -n "${DOMAIN_USER_EMAIL:-}" ]]  && USER_EMAIL="$DOMAIN_USER_EMAIL"
}

save_global_env() {
  mkdir -p "$(dirname "$GLOBAL_ENV")"
  cat > "$GLOBAL_ENV" <<ENV
# domain install-user — generado por $0
# API_KEY no se guarda acá por seguridad.
DOMAIN_VPS_URL="$VPS_URL"
DOMAIN_USER_EMAIL="$USER_EMAIL"
ENV
  chmod 600 "$GLOBAL_ENV"
}

case "$(uname -s)" in
  Darwin) OS="macos" ;;
  Linux)  OS="linux" ;;
  *)      fail "OS no soportado: $(uname -s)"; exit 1 ;;
esac

require_cmd() {
  command -v "$1" >/dev/null 2>&1 || { fail "comando requerido no encontrado: $1"; exit 1; }
}
require_cmd curl
HAS_JQ=0
command -v jq >/dev/null 2>&1 && HAS_JQ=1

# Paths globales (independientes del cliente)
GLOBAL_SKILL_PATH="$HOME/.claude/skills/domain/SKILL.md"
GLOBAL_AGENT_PATH="$HOME/.claude/agents/domain-memory.md"

# Paths de configs por cliente
case "$OS" in
  macos)
    CLAUDE_CODE_DIR="$HOME/.claude"
    CURSOR_DIR="$HOME/.cursor"
    CLINE_VSCODE="$HOME/Library/Application Support/Code/User/globalStorage/saoudrizwan.claude-dev/settings"
    CONTINUE_DIR="$HOME/.continue"
    CLAUDE_DESKTOP_DIR="$HOME/Library/Application Support/Claude"
    OPENCODE_DIR="$HOME/.config/opencode"
    ;;
  linux)
    CLAUDE_CODE_DIR="$HOME/.claude"
    CURSOR_DIR="$HOME/.cursor"
    CLINE_VSCODE="$HOME/.config/Code/User/globalStorage/saoudrizwan.claude-dev/settings"
    CONTINUE_DIR="$HOME/.continue"
    CLAUDE_DESKTOP_DIR="$HOME/.config/Claude"
    OPENCODE_DIR="$HOME/.config/opencode"
    ;;
esac

TIMESTAMP="$(date -u +"%Y%m%dT%H%M%SZ")"

# ----------------------------------------------------------------------------
# Detección
# ----------------------------------------------------------------------------
detect_clients() {
  DETECTED=()
  [[ -d "$CLAUDE_CODE_DIR" ]]    && DETECTED+=("claude-code")
  [[ -d "$CURSOR_DIR" ]]         && DETECTED+=("cursor")
  [[ -d "$CLINE_VSCODE" ]]       && DETECTED+=("cline")
  [[ -d "$CONTINUE_DIR" ]]       && DETECTED+=("continue")
  [[ -d "$CLAUDE_DESKTOP_DIR" ]] && DETECTED+=("claude-desktop")
  if command -v opencode >/dev/null 2>&1 || [[ -d "$OPENCODE_DIR" ]]; then
    DETECTED+=("opencode")
  fi
}

# ----------------------------------------------------------------------------
# Skill + Agent global (una sola copia en disco)
# ----------------------------------------------------------------------------
install_global_skill_and_agent() {
  mkdir -p "$(dirname "$GLOBAL_SKILL_PATH")" "$(dirname "$GLOBAL_AGENT_PATH")"
  cp "$TEMPLATES_DIR/skill-domain/SKILL.md" "$GLOBAL_SKILL_PATH"
  cp "$TEMPLATES_DIR/agents/domain-memory.md" "$GLOBAL_AGENT_PATH"
  ok "skill: $GLOBAL_SKILL_PATH"
  ok "agent: $GLOBAL_AGENT_PATH"
}

# Symlink (idempotente) — usar para clientes que tienen su propia path
# pero comparten contenido con los archivos globales.
link_to_global() {
  local link="$1"; local target="$2"
  mkdir -p "$(dirname "$link")"
  # Si ya es symlink al mismo target, no hacer nada
  if [[ -L "$link" ]] && [[ "$(readlink "$link")" = "$target" ]]; then
    return
  fi
  # Si existe como archivo real, backup y reemplazo por symlink
  if [[ -e "$link" && ! -L "$link" ]]; then
    cp "$link" "$link.backup-$TIMESTAMP"
  fi
  ln -sf "$target" "$link"
}

# ----------------------------------------------------------------------------
# Config del MCP server por cliente (transport config — NO rules)
# ----------------------------------------------------------------------------

config_claude_code() {
  local target="$CLAUDE_CODE_DIR/mcp_servers.json"
  mkdir -p "$(dirname "$target")"
  [[ -f "$target" ]] && cp "$target" "$target.backup-$TIMESTAMP"

  if [[ $HAS_JQ -eq 1 && -f "$target" ]]; then
    # Migración: borrar entry legacy "domain" antes de plantar "domain-mcp"
    # (evita duplicación si el usuario instaló con versión anterior).
    jq --arg url "$VPS_URL/mcp" --arg key "$API_KEY" '
      (.mcpServers // {}) |= del(.["domain"])
      | .mcpServers["domain-mcp"] = {url: $url, headers: {Authorization: ("Bearer " + $key)}}
    ' "$target" > "$target.tmp" && mv "$target.tmp" "$target"
  else
    cat > "$target" <<JSON
{
  "mcpServers": {
    "domain-mcp": {
      "url": "$VPS_URL/mcp",
      "headers": {
        "Authorization": "Bearer $API_KEY"
      }
    }
  }
}
JSON
  fi
  ok "claude-code: $target"
  # Skill + agent: ya plantados globalmente (los lee desde $CLAUDE_CODE_DIR)
}

config_opencode() {
  local target="$OPENCODE_DIR/opencode.json"
  mkdir -p "$OPENCODE_DIR"
  [[ -f "$target" ]] && cp "$target" "$target.backup-$TIMESTAMP"

  if [[ $HAS_JQ -eq 1 && -f "$target" ]]; then
    jq --arg url "$VPS_URL/mcp" --arg key "$API_KEY" '
      (.mcp // {}) |= del(.["domain"])
      | .mcp["domain-mcp"] = {type: "remote", url: $url, headers: {Authorization: ("Bearer " + $key)}, enabled: true}
    ' "$target" > "$target.tmp" && mv "$target.tmp" "$target"
  else
    cat > "$target" <<JSON
{
  "mcp": {
    "domain-mcp": {
      "type": "remote",
      "url": "$VPS_URL/mcp",
      "headers": {
        "Authorization": "Bearer $API_KEY"
      },
      "enabled": true
    }
  }
}
JSON
  fi
  ok "opencode: $target"

  # Symlinks al skill y agent globales (una sola copia en disco)
  link_to_global "$OPENCODE_DIR/skills/domain/SKILL.md" "$GLOBAL_SKILL_PATH"
  link_to_global "$OPENCODE_DIR/agents/domain-memory.md" "$GLOBAL_AGENT_PATH"
  ok "opencode skill/agent: symlinks a globales"
}

config_cursor() {
  local target="$CURSOR_DIR/mcp.json"
  [[ -f "$target" ]] && cp "$target" "$target.backup-$TIMESTAMP"
  if [[ $HAS_JQ -eq 1 && -f "$target" ]]; then
    jq --arg url "$VPS_URL/mcp" --arg key "$API_KEY" '
      (.mcpServers // {}) |= del(.["domain"])
      | .mcpServers["domain-mcp"] = {url: $url, headers: {Authorization: ("Bearer " + $key)}}
    ' "$target" > "$target.tmp" && mv "$target.tmp" "$target"
  else
    cat > "$target" <<JSON
{
  "mcpServers": {
    "domain-mcp": {
      "url": "$VPS_URL/mcp",
      "headers": {
        "Authorization": "Bearer $API_KEY"
      }
    }
  }
}
JSON
  fi
  ok "cursor: $target"
  info "cursor recibe el protocolo via MCP handshake instructions; no se planta rule file."
}

config_cline() {
  local target="$CLINE_VSCODE/cline_mcp_settings.json"
  mkdir -p "$(dirname "$target")"
  [[ -f "$target" ]] && cp "$target" "$target.backup-$TIMESTAMP"
  cat > "$target" <<JSON
{
  "mcpServers": {
    "domain-mcp": {
      "url": "$VPS_URL/mcp",
      "headers": {
        "Authorization": "Bearer $API_KEY"
      },
      "alwaysAllow": []
    }
  }
}
JSON
  ok "cline: $target"
  info "cline recibe el protocolo via MCP handshake instructions; no se planta rule file."
}

config_continue() {
  local target="$CONTINUE_DIR/config.json"
  mkdir -p "$CONTINUE_DIR"
  [[ -f "$target" ]] && cp "$target" "$target.backup-$TIMESTAMP"
  if [[ $HAS_JQ -eq 1 && -f "$target" ]]; then
    jq --arg url "$VPS_URL/mcp" --arg key "$API_KEY" \
      '.experimental.modelContextProtocolServers = [{transport: {type: "http", url: $url, headers: {Authorization: ("Bearer " + $key)}}}]' \
      "$target" > "$target.tmp" && mv "$target.tmp" "$target"
    ok "continue: $target"
  else
    warn "Continue requiere jq para merge — config NO modificada"
  fi
}

config_claude_desktop() {
  local target="$CLAUDE_DESKTOP_DIR/claude_desktop_config.json"
  mkdir -p "$CLAUDE_DESKTOP_DIR"
  [[ -f "$target" ]] && cp "$target" "$target.backup-$TIMESTAMP"
  if [[ $HAS_JQ -eq 1 && -f "$target" ]]; then
    jq --arg url "$VPS_URL/mcp" --arg key "$API_KEY" '
      (.mcpServers // {}) |= del(.["domain"])
      | .mcpServers["domain-mcp"] = {url: $url, headers: {Authorization: ("Bearer " + $key)}}
    ' "$target" > "$target.tmp" && mv "$target.tmp" "$target"
  else
    cat > "$target" <<JSON
{
  "mcpServers": {
    "domain-mcp": {
      "url": "$VPS_URL/mcp",
      "headers": {
        "Authorization": "Bearer $API_KEY"
      }
    }
  }
}
JSON
  fi
  ok "claude-desktop: $target"
}

# ----------------------------------------------------------------------------
# Uninstall: restaura backups
# ----------------------------------------------------------------------------
uninstall_client_cfg() {
  local config_path="$1"
  local client_name="$2"
  if [[ ! -f "$config_path" ]]; then
    info "$client_name: no config existente, skip"
    return
  fi
  # Estrategia determinista: SIEMPRE eliminar solo la entry domain con jq.
  # Restaurar el backup completo perdería ediciones que el usuario haya hecho
  # entre install y uninstall — eso era peor.
  # Borramos tanto "domain-mcp" (nombre actual) como "domain" (legacy de
  # instalaciones previas) para que el unisntall sea idempotente y robusto.
  if [[ $HAS_JQ -eq 1 ]]; then
    # del() de un path inexistente es no-op (no crea la key intermedia),
    # así que es seguro encadenar las 4 sin chequear shape del JSON.
    if jq '
      del(.mcpServers["domain-mcp"])
      | del(.mcpServers["domain"])
      | del(.mcp["domain-mcp"])
      | del(.mcp["domain"])
    ' "$config_path" > "$config_path.tmp" 2>/dev/null; then
      mv "$config_path.tmp" "$config_path"
      ok "$client_name: entries domain/domain-mcp removidas (resto del archivo intacto)"
    else
      rm -f "$config_path.tmp" 2>/dev/null
      warn "$client_name: el archivo no es JSON parseable, no se modificó"
    fi
  else
    warn "$client_name: requiere jq para uninstall determinista; o borrá la entry manualmente"
  fi
  # El backup más reciente queda en disco como red de seguridad — no se
  # restaura automáticamente. Si el usuario quiere, puede revertir manualmente.
  local latest_backup
  latest_backup=$(ls -t "$config_path".backup-* 2>/dev/null | head -1 || true)
  if [[ -n "$latest_backup" ]]; then
    info "$client_name: backup disponible si necesitás revertir: $latest_backup"
  fi
}

uninstall_all() {
  step "Desinstalando Domain MCP"
  detect_clients
  for client in "${DETECTED[@]}"; do
    case "$client" in
      claude-code)    uninstall_client_cfg "$CLAUDE_CODE_DIR/mcp_servers.json" "claude-code" ;;
      opencode)       uninstall_client_cfg "$OPENCODE_DIR/opencode.json" "opencode"
                      rm -f "$OPENCODE_DIR/skills/domain/SKILL.md" 2>/dev/null
                      rm -f "$OPENCODE_DIR/agents/domain-memory.md" 2>/dev/null
                      ok "opencode: symlinks limpiados" ;;
      cursor)         uninstall_client_cfg "$CURSOR_DIR/mcp.json" "cursor" ;;
      cline)          uninstall_client_cfg "$CLINE_VSCODE/cline_mcp_settings.json" "cline" ;;
      continue)       warn "continue: revisión manual recomendada en ~/.continue/config.json" ;;
      claude-desktop) uninstall_client_cfg "$CLAUDE_DESKTOP_DIR/claude_desktop_config.json" "claude-desktop" ;;
    esac
  done
  # Skill + agent globales
  rm -f "$GLOBAL_SKILL_PATH" "$GLOBAL_AGENT_PATH" 2>/dev/null
  rmdir "$(dirname "$GLOBAL_SKILL_PATH")" 2>/dev/null || true
  ok "skill + agent globales removidos"
  # .env global
  rm -f "$GLOBAL_ENV" 2>/dev/null
  rmdir "$(dirname "$GLOBAL_ENV")" 2>/dev/null || true
  ok ".env global removido"
  step "Listo"
  echo "  Reiniciá tus clientes MCP."
}

# ----------------------------------------------------------------------------
# Install flow
# ----------------------------------------------------------------------------
install_all() {
  step "Domain MCP — install user"

  load_global_env
  if [[ -n "$VPS_URL" ]]; then
    ok "URL del VPS (desde $GLOBAL_ENV): $VPS_URL"
  fi
  if [[ -n "$USER_EMAIL" ]]; then
    ok "Email (desde $GLOBAL_ENV): $USER_EMAIL"
  fi

  [[ -z "$VPS_URL" ]]    && read -rp "  URL del VPS (ej. http://1.2.3.4): " VPS_URL
  [[ -z "$USER_EMAIL" ]] && read -rp "  Email: " USER_EMAIL
  [[ -z "$API_KEY" ]]    && { read -rsp "  API key: " API_KEY; echo; }

  [[ -z "$VPS_URL" ]]    && { fail "URL del VPS requerida"; exit 1; }
  [[ -z "$USER_EMAIL" ]] && { fail "Email requerido"; exit 1; }
  [[ -z "$API_KEY" ]]    && { fail "API key requerida"; exit 1; }
  VPS_URL="${VPS_URL%/}"

  # Persistir VPS_URL/EMAIL para re-ejecuciones futuras.
  save_global_env
  ok "guardado en $GLOBAL_ENV (modo 0600)"

  step "Verificando conexión al VPS"
  if curl -fsS --max-time 5 "$VPS_URL/healthz" >/dev/null 2>&1; then
    ok "VPS responde en $VPS_URL"
  else
    warn "VPS no responde en $VPS_URL/healthz (continuando igual)"
  fi

  step "Detectando clientes MCP"
  detect_clients
  if [[ ${#DETECTED[@]} -eq 0 ]]; then
    fail "Ningún cliente MCP detectado. Clientes: claude-code, opencode, cursor, cline, continue, claude-desktop"
    exit 1
  fi
  for c in "${DETECTED[@]}"; do ok "$c"; done
  [[ $DRY_RUN -eq 1 ]] && { warn "DRY-RUN: terminando sin tocar configs"; exit 0; }

  step "Plantando skill + subagent globales"
  install_global_skill_and_agent

  step "Configurando clientes (MCP transport)"
  for client in "${DETECTED[@]}"; do
    case "$client" in
      claude-code)    config_claude_code ;;
      opencode)       config_opencode ;;
      cursor)         config_cursor ;;
      cline)          config_cline ;;
      continue)       config_continue ;;
      claude-desktop) config_claude_desktop ;;
    esac
  done

  step "Listo"
  cat <<RESUMEN

  ${GREEN}${BOLD}Domain MCP configurado${RESET}

  VPS:    $VPS_URL
  Email:  $USER_EMAIL

  Archivos en disco (totales en este sistema):
    · $GLOBAL_SKILL_PATH
    · $GLOBAL_AGENT_PATH
    · 1 archivo de config MCP por cliente detectado (transport-only)

  Protocolo de uso: vive en BD como policy 'agent-protocol' (editable
  con domain_policy_update). El MCP server lo inyecta en cada
  initialize via instructions; no hay archivos rules sueltos.

  Próximos pasos:
    1. Reiniciá tus clientes MCP.
    2. Mandá un mensaje al LLM → debe llamar domain_policy_get('agent-protocol')
       o seguir el handshake instructions y usar tools domain_*.

  Para desinstalar: $0 --uninstall

RESUMEN
}

# ----------------------------------------------------------------------------
# Main
# ----------------------------------------------------------------------------
if [[ $UNINSTALL -eq 1 ]]; then
  uninstall_all
else
  install_all
fi
