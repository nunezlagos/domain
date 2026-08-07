#!/usr/bin/env bash
# bootstrap.sh — flujo zero-touch para Linux/macOS/WSL2.
#
# Lo que hace:
#   1. Detecta si Go (>= 1.22) está instalado.
#   2. Si NO está, baja Go oficial a ~/.local/go/ (sin sudo, sin tocar /usr).
#   3. Compila el binario domain-install.
#   4. Lo ejecuta pasando los args que recibió este script.
#
# Cero dependencias además de: curl, tar (los traen todas las distros base).
#
# Uso:
#   ./bootstrap.sh                                    # interactive
#   ./bootstrap.sh --url ... --email ... --api-key ...
#   ./bootstrap.sh --uninstall
#   ./bootstrap.sh --dry-run
#
# El binario compilado queda en install-user/domain-install — re-ejecuciones
# saltean la fase de Go install + build si ya existe y es reciente.

set -euo pipefail

GO_VERSION="1.22.6"  # mínimo requerido por go.mod
GO_INSTALL_DIR="$HOME/.local/go"

BOLD=$'\033[1m'; RESET=$'\033[0m'
GREEN=$'\033[32m'; YELLOW=$'\033[33m'; RED=$'\033[31m'; DIM=$'\033[2m'
step() { echo ""; echo "${BOLD}==> $1${RESET}"; }
ok()   { echo "${GREEN}    ✓${RESET} $1"; }
warn() { echo "${YELLOW}    !${RESET} $1"; }
fail() { echo "${RED}    ✗${RESET} $1" >&2; }
info() { echo "${DIM}    ·${RESET} $1"; }

# ---------- detección OS/arch ----------
case "$(uname -s)" in
  Linux)  OS="linux" ;;
  Darwin) OS="darwin" ;;
  *) fail "OS no soportado: $(uname -s). Usá bootstrap.ps1 en Windows nativo."; exit 1 ;;
esac

case "$(uname -m)" in
  x86_64|amd64) ARCH="amd64" ;;
  arm64|aarch64) ARCH="arm64" ;;
  *) fail "arquitectura no soportada: $(uname -m)"; exit 1 ;;
esac

# ---------- detección distro (Linux only; macOS es "darwin") ----------
DISTRO="unknown"
if [ "$OS" = "linux" ] && [ -r /etc/os-release ]; then
  DISTRO=$(. /etc/os-release && printf '%s' "${ID:-unknown}")
elif [ "$OS" = "darwin" ]; then
  DISTRO="darwin"
fi

# ---------- checks de dependencias (curl, tar, git) ----------
# Si alguna falta, abortamos con instrucciones específicas por distro.
# NO instalamos automáticamente con sudo — eso sería agresivo y surprise-y.
missing_deps=()
for dep in curl tar git; do
  if ! command -v "$dep" >/dev/null 2>&1; then
    missing_deps+=("$dep")
  fi
done

step "Entorno: ${OS}/${ARCH} (${DISTRO})"
ok "OS: $OS"
ok "Arch: $ARCH"
ok "Distro: $DISTRO"
if [ ${#missing_deps[@]} -eq 0 ]; then
  ok "deps presentes: curl, tar, git"
else
  fail "deps faltantes: ${missing_deps[*]}"
  echo ""
  echo "  Instalá las deps con el package manager de tu distro:" >&2
  echo "" >&2
  case "$DISTRO" in
    ubuntu|debian|pop|linuxmint|elementary|zorin|kubuntu)
      echo "    sudo apt-get update && sudo apt-get install -y ${missing_deps[*]}" >&2
      ;;
    arch|manjaro|endeavouros|garuda)
      echo "    sudo pacman -S --needed ${missing_deps[*]}" >&2
      ;;
    fedora|rhel|centos|rocky|almalinux|nobara)
      echo "    sudo dnf install -y ${missing_deps[*]}" >&2
      ;;
    opensuse*|suse)
      echo "    sudo zypper install ${missing_deps[*]}" >&2
      ;;
    alpine)
      echo "    sudo apk add ${missing_deps[*]}" >&2
      ;;
    darwin)
      echo "    brew install ${missing_deps[*]}" >&2
      echo "    (o instalá Xcode Command Line Tools: xcode-select --install)" >&2
      ;;
    *)
      echo "    instalá: ${missing_deps[*]} (usando el package manager de tu distro)" >&2
      ;;
  esac
  echo "" >&2
  exit 1
fi

SCRIPT_DIR="$( cd "$( dirname "${BASH_SOURCE[0]}" )" && pwd )"

# ---------- binario publicado (issue-57.1 fase 2) ----------
# Bajarlo evita instalar Go y compilar, pero sobre todo evita que el cliente salga sin versión:
# un binario compilado acá se declara 'dev' y queda FUERA de la comparación por diseño, así que
# nunca recibiría el aviso de actualización. El de la release trae su -X puesto.
#
# La descarga es ANÓNIMA y esa es una restricción dura, no una preferencia: hay usuarios que
# solo clonan la solución y la instalan, sin cuenta de GitHub. curl sin headers de auth.
#
# /releases/latest/download resuelve al último tag publicado, así que el script no hardcodea
# ningún número de versión.
DOMAIN_REPO="${DOMAIN_REPO:-nunezlagos/domain}"
ASSET="domain-install-${OS}-${ARCH}"
RELEASE_URL="https://github.com/${DOMAIN_REPO}/releases/latest/download"

# sha256sum es de coreutils y no existe en macOS; ahí el equivalente es shasum -a 256. Sin las
# dos, la verificación fallaría siempre en mac y el script caería a compilar sin decir por qué.
sha_check() {
  if command -v sha256sum >/dev/null 2>&1; then sha256sum -c --status -
  elif command -v shasum >/dev/null 2>&1; then shasum -a 256 -c --status -
  else return 1
  fi
}

binario_descargado=0
if [ "${DOMAIN_INSTALL_FROM_SOURCE:-0}" != "1" ]; then
  step "Bajando el binario publicado (${ASSET})"
  DL=$(mktemp -d)
  if curl -fsSL --retry 3 -m 120 -o "$DL/$ASSET" "$RELEASE_URL/$ASSET" \
     && curl -fsSL --retry 3 -m 60 -o "$DL/SHA256SUMS.txt" "$RELEASE_URL/SHA256SUMS.txt" \
     && (cd "$DL" && grep " ${ASSET}\$" SHA256SUMS.txt | sha_check); then
    chmod +x "$DL/$ASSET"
    mv "$DL/$ASSET" "$SCRIPT_DIR/domain-install"
    binario_descargado=1
    ok "binario verificado contra SHA256SUMS.txt — sin Go, sin compilar"
  else
    # Que caiga acá es esperable: arquitecturas sin binario publicado, sin red, o un tag cuyo
    # workflow de release fallo. Se informa y se sigue por el camino de siempre.
    warn "no se pudo bajar o verificar el binario publicado — se compila desde el código"
  fi
  rm -rf "$DL"
fi

if [ "$binario_descargado" = "1" ]; then
  step "Ejecutando domain-install $*"
  cd "$SCRIPT_DIR"
  exec ./domain-install "$@"
fi

# ---------- detección Go ----------
go_ok=0
if command -v go >/dev/null 2>&1; then
  current=$(go version 2>/dev/null | awk '{print $3}' | sed 's/^go//')
  # Versión válida si comienza con 1.2x o superior — heurística simple
  major=$(echo "$current" | cut -d. -f1)
  minor=$(echo "$current" | cut -d. -f2)
  if [ "$major" -ge 1 ] && [ "$minor" -ge 22 ]; then
    go_ok=1
    info "Go encontrado: $(command -v go) (version $current)"
  else
    warn "Go $current detectado, pero necesitamos >= 1.22. Voy a bajar uno local."
  fi
fi

# Probar también ~/.local/go/bin si lo bajamos antes
if [ "$go_ok" -eq 0 ] && [ -x "$GO_INSTALL_DIR/bin/go" ]; then
  export PATH="$GO_INSTALL_DIR/bin:$PATH"
  current=$("$GO_INSTALL_DIR/bin/go" version | awk '{print $3}' | sed 's/^go//')
  ok "Reusando Go local previamente bajado: $current"
  go_ok=1
fi

# ---------- instalar Go si falta ----------
if [ "$go_ok" -eq 0 ]; then
  step "Bajando Go ${GO_VERSION} a ${GO_INSTALL_DIR}"
  TAR="go${GO_VERSION}.${OS}-${ARCH}.tar.gz"
  URL="https://go.dev/dl/${TAR}"
  TMP=$(mktemp -d)
  trap 'rm -rf "$TMP"' EXIT

  info "URL: $URL"
  if ! curl -fsSL --retry 3 -m 120 -o "$TMP/$TAR" "$URL"; then
    fail "no se pudo descargar Go desde go.dev"
    exit 1
  fi
  ok "tarball bajado ($(du -h "$TMP/$TAR" | awk '{print $1}'))"

  mkdir -p "$GO_INSTALL_DIR"
  # tar -C extrae al dir; el tarball tiene un dir top-level "go/" — extraer
  # con --strip-components=1 mete su contenido directamente en GO_INSTALL_DIR.
  if ! tar -C "$GO_INSTALL_DIR" -xzf "$TMP/$TAR" --strip-components=1; then
    fail "fallo al extraer el tarball de Go"
    exit 1
  fi
  ok "Go instalado en $GO_INSTALL_DIR (sin tocar /usr ni requerir sudo)"

  export PATH="$GO_INSTALL_DIR/bin:$PATH"
  info "PATH actualizado para esta sesión: $GO_INSTALL_DIR/bin"
  info "Para usarlo en otras shells: agregá 'export PATH=\"\$HOME/.local/go/bin:\$PATH\"' a tu ~/.bashrc o ~/.zshrc"
fi

# ---------- build ----------
cd "$SCRIPT_DIR"

# Se compila SIN -X a propósito: un binario hecho desde el código fuente no es una release, y
# declararse 'dev' es lo que lo deja fuera de la comparación de versiones. Mismo criterio que
# install-user/Makefile, que usa --exact-match.
step "Compilando domain-install"
if go build -ldflags "-s -w" -o domain-install .; then
  ok "binario listo: $SCRIPT_DIR/domain-install ($(du -h domain-install | awk '{print $1}'))"
else
  fail "fallo de build"
  exit 1
fi

# ---------- run ----------
step "Ejecutando domain-install $*"
exec ./domain-install "$@"
