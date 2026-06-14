#!/usr/bin/env bash
# install-user.sh — configura clientes MCP de la laptop para apuntar al VPS de Domain.
#
# Uso:
#   ./install-user.sh                          # interactive (pide URL, email, API key)
#   ./install-user.sh --url http://1.2.3.4 \   # no-interactive (flags)
#                     --email u@x.cl \
#                     --api-key domk_live_...
#   ./install-user.sh --uninstall              # restaura configs originales
#
# Clientes soportados:
#   claude-code, Cursor, Cline (VS Code ext), Continue (VS Code ext), Claude Desktop
#
# Idempotente: re-correr no rompe ni duplica config.

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
    -h|--help)    sed -n '2,15p' "$0"; exit 0 ;;
    *) fail "flag desconocida: $1"; exit 2 ;;
  esac
done

# ----------------------------------------------------------------------------
# Helpers
# ----------------------------------------------------------------------------
SCRIPT_DIR="$( cd "$( dirname "${BASH_SOURCE[0]}" )" && pwd )"
TEMPLATES_DIR="$SCRIPT_DIR/templates"

# Detectar OS
case "$(uname -s)" in
  Darwin) OS="macos" ;;
  Linux)  OS="linux" ;;
  *)      fail "OS no soportado: $(uname -s) (solo macOS/Linux por ahora)"; exit 1 ;;
esac

# Verificar deps
require_cmd() {
  command -v "$1" >/dev/null 2>&1 || { fail "comando requerido no encontrado: $1"; exit 1; }
}
require_cmd curl
# jq es preferible para JSON merge pero hay fallback sed
HAS_JQ=0
command -v jq >/dev/null 2>&1 && HAS_JQ=1

# Paths de configs por cliente (según OS)
case "$OS" in
  macos)
    CLAUDE_CODE_DIR="$HOME/.claude"
    CURSOR_DIR="$HOME/.cursor"
    CLINE_VSCODE="$HOME/Library/Application Support/Code/User/globalStorage/saoudrizwan.claude-dev/settings"
    CONTINUE_DIR="$HOME/.continue"
    CLAUDE_DESKTOP_DIR="$HOME/Library/Application Support/Claude"
    ;;
  linux)
    CLAUDE_CODE_DIR="$HOME/.claude"
    CURSOR_DIR="$HOME/.cursor"
    CLINE_VSCODE="$HOME/.config/Code/User/globalStorage/saoudrizwan.claude-dev/settings"
    CONTINUE_DIR="$HOME/.continue"
    CLAUDE_DESKTOP_DIR="$HOME/.config/Claude"
    ;;
esac

TIMESTAMP="$(date -u +"%Y%m%dT%H%M%SZ")"

# ----------------------------------------------------------------------------
# Detección de clientes instalados
# ----------------------------------------------------------------------------
detect_clients() {
  DETECTED=()
  [[ -d "$CLAUDE_CODE_DIR" ]]    && DETECTED+=("claude-code")
  [[ -d "$CURSOR_DIR" ]]         && DETECTED+=("cursor")
  [[ -d "$CLINE_VSCODE" ]]       && DETECTED+=("cline")
  [[ -d "$CONTINUE_DIR" ]]       && DETECTED+=("continue")
  [[ -d "$CLAUDE_DESKTOP_DIR" ]] && DETECTED+=("claude-desktop")
}

# ----------------------------------------------------------------------------
# Configuración del MCP server por cliente
# ----------------------------------------------------------------------------

# claude-code: ~/.claude/mcp_servers.json
config_claude_code() {
  local target="$CLAUDE_CODE_DIR/mcp_servers.json"
  mkdir -p "$(dirname "$target")"
  [[ -f "$target" ]] && cp "$target" "$target.backup-$TIMESTAMP"

  if [[ $HAS_JQ -eq 1 && -f "$target" ]]; then
    jq --arg url "$VPS_URL/mcp" --arg key "$API_KEY" \
      '.mcpServers.domain = {url: $url, headers: {Authorization: ("Bearer " + $key)}}' \
      "$target" > "$target.tmp" && mv "$target.tmp" "$target"
  else
    cat > "$target" <<JSON
{
  "mcpServers": {
    "domain": {
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

  # Rules
  local rules_dir="$CLAUDE_CODE_DIR/instructions"
  mkdir -p "$rules_dir"
  cp "$TEMPLATES_DIR/claude-code-rules.md" "$rules_dir/domain.md"
  ok "claude-code rules: $rules_dir/domain.md"
}

# Cursor: ~/.cursor/mcp.json + ~/.cursor/rules/domain.mdc
config_cursor() {
  local target="$CURSOR_DIR/mcp.json"
  [[ -f "$target" ]] && cp "$target" "$target.backup-$TIMESTAMP"

  if [[ $HAS_JQ -eq 1 && -f "$target" ]]; then
    jq --arg url "$VPS_URL/mcp" --arg key "$API_KEY" \
      '.mcpServers.domain = {url: $url, headers: {Authorization: ("Bearer " + $key)}}' \
      "$target" > "$target.tmp" && mv "$target.tmp" "$target"
  else
    cat > "$target" <<JSON
{
  "mcpServers": {
    "domain": {
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

  local rules_dir="$CURSOR_DIR/rules"
  mkdir -p "$rules_dir"
  cp "$TEMPLATES_DIR/cursor-rules.mdc" "$rules_dir/domain.mdc"
  ok "cursor rules: $rules_dir/domain.mdc"
}

# Cline (VS Code ext): settings.json
config_cline() {
  local target="$CLINE_VSCODE/cline_mcp_settings.json"
  mkdir -p "$(dirname "$target")"
  [[ -f "$target" ]] && cp "$target" "$target.backup-$TIMESTAMP"

  cat > "$target" <<JSON
{
  "mcpServers": {
    "domain": {
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

  # Cline rules: en .clinerules global o en cada workspace
  local rules_target="$HOME/.clinerules-domain"
  cp "$TEMPLATES_DIR/cline-rules.md" "$rules_target"
  ok "cline rules: $rules_target"
}

# Continue (VS Code ext): ~/.continue/config.json
config_continue() {
  local target="$CONTINUE_DIR/config.json"
  mkdir -p "$CONTINUE_DIR"
  [[ -f "$target" ]] && cp "$target" "$target.backup-$TIMESTAMP"

  if [[ $HAS_JQ -eq 1 && -f "$target" ]]; then
    jq --arg url "$VPS_URL/mcp" --arg key "$API_KEY" \
      '.experimental.modelContextProtocolServers = [{transport: {type: "http", url: $url, headers: {Authorization: ("Bearer " + $key)}}}]' \
      "$target" > "$target.tmp" && mv "$target.tmp" "$target"
  else
    warn "Continue requiere jq para merge — config NO modificada"
    warn "Instalá jq o configurá manualmente desde $TEMPLATES_DIR/continue-rules.md"
    return
  fi
  ok "continue: $target"
}

# Claude Desktop: claude_desktop_config.json
config_claude_desktop() {
  local target="$CLAUDE_DESKTOP_DIR/claude_desktop_config.json"
  mkdir -p "$CLAUDE_DESKTOP_DIR"
  [[ -f "$target" ]] && cp "$target" "$target.backup-$TIMESTAMP"

  if [[ $HAS_JQ -eq 1 && -f "$target" ]]; then
    jq --arg url "$VPS_URL/mcp" --arg key "$API_KEY" \
      '.mcpServers.domain = {url: $url, headers: {Authorization: ("Bearer " + $key)}}' \
      "$target" > "$target.tmp" && mv "$target.tmp" "$target"
  else
    cat > "$target" <<JSON
{
  "mcpServers": {
    "domain": {
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
uninstall_client() {
  local config_path="$1"
  local client_name="$2"

  if [[ ! -f "$config_path" ]]; then
    info "$client_name: no config existente, skip"
    return
  fi

  # Buscar backup más reciente
  local latest_backup
  latest_backup=$(ls -t "$config_path".backup-* 2>/dev/null | head -1 || true)

  if [[ -n "$latest_backup" ]]; then
    cp "$latest_backup" "$config_path"
    ok "$client_name: restaurado desde $latest_backup"
  else
    # Sin backup: el archivo lo creó este script, lo borramos
    if grep -q '"domain"' "$config_path" 2>/dev/null; then
      if [[ $HAS_JQ -eq 1 ]]; then
        jq 'del(.mcpServers.domain)' "$config_path" > "$config_path.tmp" && mv "$config_path.tmp" "$config_path"
        ok "$client_name: entry 'domain' removida"
      else
        warn "$client_name: requiere jq para remover entry; o borrá manualmente"
      fi
    fi
  fi
}

uninstall_all() {
  step "Desinstalando configuración Domain MCP"
  detect_clients

  for client in "${DETECTED[@]}"; do
    case "$client" in
      claude-code)
        uninstall_client "$CLAUDE_CODE_DIR/mcp_servers.json" "claude-code"
        rm -f "$CLAUDE_CODE_DIR/instructions/domain.md" && ok "claude-code rules removidos"
        ;;
      cursor)
        uninstall_client "$CURSOR_DIR/mcp.json" "cursor"
        rm -f "$CURSOR_DIR/rules/domain.mdc" && ok "cursor rules removidos"
        ;;
      cline)
        uninstall_client "$CLINE_VSCODE/cline_mcp_settings.json" "cline"
        rm -f "$HOME/.clinerules-domain" && ok "cline rules removidos"
        ;;
      continue)
        warn "continue: revisión manual recomendada en ~/.continue/config.json"
        ;;
      claude-desktop)
        uninstall_client "$CLAUDE_DESKTOP_DIR/claude_desktop_config.json" "claude-desktop"
        ;;
    esac
  done

  step "Listo"
  echo "  Reiniciá tus clientes MCP."
}

# ----------------------------------------------------------------------------
# Install flow
# ----------------------------------------------------------------------------
install_all() {
  step "Domain MCP — install user"

  # Pedir datos si faltan
  if [[ -z "$VPS_URL" ]]; then
    read -rp "  URL del VPS (ej. http://1.2.3.4): " VPS_URL
  fi
  if [[ -z "$USER_EMAIL" ]]; then
    read -rp "  Email: " USER_EMAIL
  fi
  if [[ -z "$API_KEY" ]]; then
    read -rsp "  API key: " API_KEY; echo
  fi

  # Validar
  [[ -z "$VPS_URL" ]]    && { fail "URL del VPS requerida"; exit 1; }
  [[ -z "$USER_EMAIL" ]] && { fail "Email requerido"; exit 1; }
  [[ -z "$API_KEY" ]]    && { fail "API key requerida"; exit 1; }

  # Sanitizar URL: quitar trailing slash
  VPS_URL="${VPS_URL%/}"

  step "Verificando conexión al VPS"
  if curl -fsS --max-time 5 "$VPS_URL/healthz" >/dev/null 2>&1; then
    ok "VPS responde en $VPS_URL"
  else
    warn "VPS no responde en $VPS_URL/healthz (continuando igual; config se aplicará)"
  fi

  step "Detectando clientes MCP"
  detect_clients
  if [[ ${#DETECTED[@]} -eq 0 ]]; then
    fail "Ningún cliente MCP detectado en este sistema."
    fail "Clientes soportados: claude-code, Cursor, Cline (VS Code), Continue (VS Code), Claude Desktop"
    exit 1
  fi
  for c in "${DETECTED[@]}"; do ok "$c"; done

  [[ $DRY_RUN -eq 1 ]] && { warn "DRY-RUN: terminando sin tocar configs"; exit 0; }

  step "Configurando clientes"
  for client in "${DETECTED[@]}"; do
    case "$client" in
      claude-code)    config_claude_code ;;
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
  Tools:  domain_* disponibles tras reiniciar clientes

  Próximos pasos:
    1. Reiniciá tus clientes MCP (claude-code, Cursor, Cline, etc.)
    2. Probá una tool: en claude-code escribí "lista mis observations"
       → debería invocar domain_observations_list

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
