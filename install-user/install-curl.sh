#!/usr/bin/env bash
# install-curl.sh — entry point del installer de user via curl.
# (version 2: ~/.cache/domain-install)
#
# Pensado para invocarse con sudo (porque puede instalar paquetes del sistema):
#
#   curl -fsSL https://raw.githubusercontent.com/nunezlagos/domain/main/install-user/install-curl.sh | sudo bash
#
# Lo que hace:
#   1. Detecta OS/arch/distro.
#   2. Verifica deps (curl, tar, git). Si falta alguna, la instala con sudo
#      (apt/pacman/dnf/zypper/apk/brew segun distro).
#   3. Verifica Go. Si no esta, lo instala con sudo desde el package manager.
#   4. Clona el repo a $REAL_HOME/.cache/domain-install (cache del user, NO
#      toca el filesystem global). Re-installs reutilizan el clone.
#   5. Compila el binario y lo ejecuta como el usuario real (sudo -u $SUDO_USER)
#      para que ~/.claude/ y ~/.config/opencode/ queden con owner correcto.
#
# Se puede ejecutar desde CUALQUIER path: no toca el cwd ni /opt ni /tmp.
# Para limpiar todo: rm -rf ~/.cache/domain-install
#
# Re-ejecutable. Si Go, deps y clone ya estan, solo compila y ejecuta.
#
# Flags (todos opcionales, se pasan al binario final):
#   --url http://1.2.3.4          URL del VPS
#   --email u@x.cl                Email admin
#   --api-key domk_live_xxx       API key
#   --target opencode             Solo configura 1 cliente
#   --uninstall                   Desinstalar
#   --dry-run                     Solo detectar
#   --yes                         No prompt

set -euo pipefail

BOLD=$'\033[1m'; RESET=$'\033[0m'
GREEN=$'\033[32m'; YELLOW=$'\033[33m'; RED=$'\033[31m'; DIM=$'\033[2m'
step() { echo ""; echo "${BOLD}==> $1${RESET}"; }
ok()   { echo "${GREEN}    ✓${RESET} $1"; }
warn() { echo "${YELLOW}    !${RESET} $1"; }
fail() { echo "${RED}    ✗${RESET} $1" >&2; }
info() { echo "${DIM}    ·${RESET} $1"; }

# ---------- usuario real (cuando se corre con sudo) ----------
REAL_USER="${SUDO_USER:-$(whoami)}"
if command -v getent >/dev/null 2>&1; then
  REAL_HOME=$(getent passwd "$REAL_USER" | cut -d: -f6)
else
  REAL_HOME=$(eval echo "~$REAL_USER")
fi
if [ -z "$REAL_HOME" ] || [ "$REAL_HOME" = "/" ]; then
  fail "no pude resolver HOME del usuario real ($REAL_USER)"
  exit 1
fi

# ---------- detección OS/arch ----------
case "$(uname -s)" in
  Linux)  OS="linux" ;;
  Darwin) OS="darwin" ;;
  *) fail "OS no soportado: $(uname -s)"; exit 1 ;;
esac

case "$(uname -m)" in
  x86_64|amd64) ARCH="amd64" ;;
  arm64|aarch64) ARCH="arm64" ;;
  *) fail "arquitectura no soportada: $(uname -m)"; exit 1 ;;
esac

# ---------- detección distro ----------
DISTRO="unknown"
PKG_MGR="unknown"
if [ "$OS" = "linux" ] && [ -r /etc/os-release ]; then
  DISTRO=$(. /etc/os-release && printf '%s' "${ID:-unknown}")
elif [ "$OS" = "darwin" ]; then
  DISTRO="darwin"
fi

case "$DISTRO" in
  ubuntu|debian|pop|linuxmint|elementary|zorin|kubuntu)
    PKG_MGR="apt" ;;
  arch|manjaro|endeavouros|garuda)
    PKG_MGR="pacman" ;;
  fedora|rhel|centos|rocky|almalinux|nobara)
    PKG_MGR="dnf" ;;
  opensuse*|suse)
    PKG_MGR="zypper" ;;
  alpine)
    PKG_MGR="apk" ;;
  darwin)
    PKG_MGR="brew" ;;
  *) warn "distro no reconocida ($DISTRO); no instalaré paquetes automáticamente" ;;
esac

# ---------- verificar que soy root ----------
if [ "$(id -u)" -ne 0 ]; then
  fail "este script necesita ejecutarse con sudo (para instalar paquetes si faltan)"
  echo "  Uso: curl -fsSL https://raw.githubusercontent.com/nunezlagos/domain/main/install-user/install-curl.sh | sudo bash" >&2
  exit 1
fi

step "Entorno: ${OS}/${ARCH} (${DISTRO}, pkg=${PKG_MGR})"
ok "OS: $OS"
ok "Arch: $ARCH"
ok "Distro: $DISTRO"
ok "Usuario real: $REAL_USER (HOME=$REAL_HOME)"

# ---------- instalar deps faltantes ----------
missing=()
for dep in curl tar git; do
  command -v "$dep" >/dev/null 2>&1 || missing+=("$dep")
done

if [ ${#missing[@]} -gt 0 ]; then
  if [ "$PKG_MGR" = "unknown" ]; then
    fail "faltan deps (${missing[*]}) y no reconozco tu distro. Instalalas manualmente."
    exit 1
  fi
  step "Instalando deps faltantes: ${missing[*]}"
  case "$PKG_MGR" in
    apt) apt-get update -qq && apt-get install -y -qq "${missing[@]}" ;;
    pacman) pacman -S --needed --noconfirm "${missing[@]}" ;;
    dnf) dnf install -y "${missing[@]}" ;;
    zypper) zypper install -y "${missing[@]}" ;;
    apk) apk add "${missing[@]}" ;;
    brew) brew install "${missing[@]}" ;;
  esac
  ok "deps instaladas"
else
  ok "deps presentes: curl, tar, git"
fi

# ---------- verificar Go ----------
need_go_install=0
if ! command -v go >/dev/null 2>&1; then
  need_go_install=1
else
  current=$(go version 2>/dev/null | awk '{print $3}' | sed 's/^go//')
  major=$(echo "$current" | cut -d. -f1)
  minor=$(echo "$current" | cut -d. -f2)
  if [ "$major" -lt 1 ] || [ "$minor" -lt 22 ]; then
    warn "Go $current detectado, pero necesitamos >= 1.22. Lo actualizo."
    need_go_install=1
  else
    ok "Go $current OK"
  fi
fi

if [ "$need_go_install" -eq 1 ]; then
  if [ "$PKG_MGR" = "unknown" ]; then
    fail "Go >= 1.22 requerido y no puedo instalarlo en distro desconocida"
    exit 1
  fi
  step "Instalando Go via $PKG_MGR"
  case "$PKG_MGR" in
    apt) apt-get install -y -qq golang-go ;;
    pacman) pacman -S --needed --noconfirm go ;;
    dnf) dnf install -y golang ;;
    zypper) zypper install -y go ;;
    apk) apk add go ;;
    brew) brew install go ;;
  esac
  ok "Go instalado"
fi

# ---------- clone a cache del user (no toca /opt, ni cwd, ni /tmp) ----------
CACHE_DIR="$REAL_HOME/.cache/domain-install"
REPO_DIR="$CACHE_DIR/repo"
mkdir -p "$CACHE_DIR"

if [ ! -d "$REPO_DIR/.git" ]; then
  step "Clonando repo a $REPO_DIR (cache del user)"
  # Como root para crear la estructura, despues chown al usuario real
  git clone --depth 1 https://github.com/nunezlagos/domain.git "$REPO_DIR"
  chown -R "$REAL_USER" "$CACHE_DIR"
  ok "repo clonado"
else
  step "Actualizando repo en $REPO_DIR"
  # git pull como el usuario real (para no cambiar ownership)
  sudo -u "$REAL_USER" git -C "$REPO_DIR" pull --depth 1 origin main
  ok "repo actualizado"
fi

# ---------- compilar y ejecutar como el usuario real ----------
step "Compilando y ejecutando domain-install como $REAL_USER"
# Compilar como el usuario real (asi el binario queda con owner correcto)
sudo -u "$REAL_USER" bash -c "cd '$REPO_DIR/install-user' && go build -ldflags '-s -w' -o domain-install ."
ok "binario compilado"

echo ""
echo "${BOLD}==> Ejecutando como $REAL_USER (HOME=$REAL_HOME)${RESET}"
exec sudo -u "$REAL_USER" HOME="$REAL_HOME" --preserve-env=PATH "$REPO_DIR/install-user/domain-install" "$@"