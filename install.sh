#!/usr/bin/env bash
# install.sh — instala domain en una sola linea.
#   curl -fsSL https://raw.githubusercontent.com/<org>/domain/main/install.sh | bash
#
# Patrón copiado de personal-tools/install.sh (mismo author).
# Linux + macOS. Windows: no soportado (instalar via WSL o Go manual).
#
# Variables de entorno (opcionales):
#   DOMAIN_INSTALL_SRC   directorio del repo (default: $HOME/.local/share/domain)
#   DOMAIN_INSTALL_DIR   directorio del binario (default: $HOME/go/bin)
#   DOMAIN_REPO_URL      URL del repo (default: github.com/<org>/domain)

set -euo pipefail

REPO_URL="${DOMAIN_REPO_URL:-https://github.com/nunezlagos/domain.git}"
SRC_DIR="${DOMAIN_INSTALL_SRC:-$HOME/.local/share/domain}"
INSTALL_DIR="${DOMAIN_INSTALL_DIR:-$HOME/go/bin}"
BINARY="domain"
MIN_GO_VERSION="1.22"

RED=$'\033[0;31m'; GREEN=$'\033[0;32m'; YELLOW=$'\033[1;33m'; BOLD=$'\033[1m'; RESET=$'\033[0m'

ok()   { echo "  ${GREEN}[ok]${RESET} $*"; }
warn() { echo "  ${YELLOW}[!]${RESET} $*"; }
die()  { echo "  ${RED}[x]${RESET} $*" >&2; exit 1; }
step() { echo -e "\n  ${BOLD}->${RESET} $*"; }

echo ""
echo -e "  ${BOLD}DOMAIN — instalador${RESET}"
echo "  installer end-to-end (HU-01.11)"
echo ""

# === Chequeo de OS ===
case "$(uname -s)" in
    Linux|Darwin) ;;
    *) die "OS no soportado: $(uname -s). Instalar Go manualmente o usar WSL." ;;
esac

# === Chequeo de git ===
command -v git >/dev/null 2>&1 || die "git no esta instalado"

# === Chequeo de Go ===
if ! command -v go >/dev/null 2>&1; then
    die "Go $MIN_GO_VERSION+ no esta instalado.
  Linux: sudo apt install -y golang-go (o version oficial https://go.dev/dl/)
  macOS: brew install go"
fi

GO_VERSION=$(go version | awk '{print $3}' | sed 's/^go//' | sed 's/-.*//')
if [ "$(printf '%s\n%s\n' "$MIN_GO_VERSION" "$GO_VERSION" | sort -V | head -1)" != "$MIN_GO_VERSION" ]; then
    die "Go $MIN_GO_VERSION+ requerido, encontrado $GO_VERSION"
fi
ok "Go $GO_VERSION detectado"

# === Clone o update ===
step "Repositorio"
if [ -d "$SRC_DIR/.git" ]; then
    # NO silenciar el pull: cuando falla y se "continúa con la versión
    # actual", el install compila código viejo y rompe después de forma
    # críptica (e.g. "no migration found for version N" contra una BD
    # más nueva). Mostrar el motivo y FALLAR, salvo opt-in explícito.
    PULL_OUT="$(cd "$SRC_DIR" && git pull --ff-only 2>&1)" || {
        echo "$PULL_OUT" | sed 's/^/    /'
        if [ -n "${DOMAIN_ALLOW_STALE:-}" ]; then
            warn "git pull falló; DOMAIN_ALLOW_STALE=1 → continuando con la versión LOCAL (puede estar desactualizada)"
        else
            die "git pull falló en $SRC_DIR (motivo arriba).
  Arreglalo (red/credenciales/cambios locales) y reintentá,
  o forzá la versión local con: DOMAIN_ALLOW_STALE=1 bash install.sh"
        fi
    }
    ok "Source actualizado en $SRC_DIR ($(cd "$SRC_DIR" && git log -1 --format=%h))"
else
    git clone --depth=1 "$REPO_URL" "$SRC_DIR" || die "clone fallo. Verificar REPO_URL: $REPO_URL"
    ok "Source clonado a $SRC_DIR"
fi

# === Build ===
step "Compilando"
mkdir -p "$INSTALL_DIR"
(cd "$SRC_DIR" && go build -o "$INSTALL_DIR/$BINARY" ./cmd/domain) || die "go build domain fallo"
ok "Binario en $INSTALL_DIR/$BINARY"
# Tambien compilamos domain-mcp (MCP server que el agente opencode usa).
(cd "$SRC_DIR" && go build -o "$INSTALL_DIR/${BINARY}-mcp" ./cmd/domain-mcp) || die "go build domain-mcp fallo"
ok "Binario en $INSTALL_DIR/${BINARY}-mcp"

# Instalaciones legacy: si quedaron binarios viejos en ~/.local/bin
# (instalador anterior), refrescarlos tambien — configs de agentes
# viejos pueden apuntar ahi y un binario desactualizado rompe el MCP.
for legacy in "$HOME/.local/bin/$BINARY" "$HOME/.local/bin/${BINARY}-mcp"; do
    if [ -f "$legacy" ]; then
        pkg="./cmd/domain"; [ "${legacy##*/}" = "${BINARY}-mcp" ] && pkg="./cmd/domain-mcp"
        (cd "$SRC_DIR" && go build -o "$legacy" "$pkg") && ok "Binario legacy refrescado: $legacy"
    fi
done

# === PATH warning ===
case ":$PATH:" in
    *":$INSTALL_DIR:"*) ;;
    *) warn "Agregar a PATH: export PATH=\"\$PATH:$INSTALL_DIR\"" ;;
esac

echo ""
echo -e "  ${GREEN}${BOLD}Compilado.${RESET}"
echo ""

# === Lanzar el instalador interactivo ===
# Si hay terminal disponible, entramos directo al wizard (config primero,
# instalación automática después). /dev/tty permite que funcione incluso
# con `curl | bash` (stdin ocupado por el pipe).
# Escape hatch: DOMAIN_NO_TUI=1 para solo compilar.
if [ -z "${DOMAIN_NO_TUI:-}" ] && [ -e /dev/tty ] && [ -t 1 ]; then
    step "Abriendo el instalador"
    cd "$SRC_DIR"
    exec "$INSTALL_DIR/$BINARY" tui < /dev/tty
fi

echo -e "  Siguiente paso: ${BOLD}cd $SRC_DIR && $BINARY tui${RESET}"
echo -e "  CLI no-interactivo: $BINARY install --mode local --non-interactive"
echo -e "  MCP server (para opencode/claude): $INSTALL_DIR/${BINARY}-mcp"
echo ""
