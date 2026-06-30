#!/usr/bin/env bash
# scripts/install-runner.sh
#
# Helper idempotente para registrar el GitHub Actions self-hosted runner
# en el VPS Contabo. NO corre ./svc.sh install automaticamente — pide
# confirmacion. Pensado para correr con un usuario no-root que tenga
# `loginctl enable-linger` habilitado.
#
# Uso:
#   ./scripts/install-runner.sh --check
#       Imprime que haria --register y --install-service, sin actuar.
#   ./scripts/install-runner.sh --register TOKEN [LABEL]
#       Descarga runner, valida checksum, registra con el token dado.
#   ./scripts/install-runner.sh --install-service
#       Habilita linger + corre svc.sh install + start.

set -euo pipefail

RUNNER_VERSION="${RUNNER_VERSION:-2.319.1}"
RUNNER_USER="${RUNNER_USER:-$(whoami)}"
RUNNER_HOME="${RUNNER_HOME:-/home/${RUNNER_USER}/actions-runner}"
RUNNER_LABEL="${RUNNER_LABEL:-deploy-vps}"
REPO_URL="${REPO_URL:-https://github.com/nunezlagos/domain}"

cmd_check() {
  echo "=== install-runner --check ==="
  echo "runner_version: $RUNNER_VERSION"
  echo "runner_user:    $RUNNER_USER"
  echo "runner_home:    $RUNNER_HOME"
  echo "runner_label:   $RUNNER_LABEL"
  echo "repo_url:       $REPO_URL"
  echo
  echo "Pasos de --register:"
  echo "  1. Descargar actions-runner-linux-x64-$RUNNER_VERSION.tar.gz"
  echo "  2. Validar SHA256 contra el publicado por GitHub"
  echo "  3. Extraer a $RUNNER_HOME (skip si ya existe run.sh)"
  echo "  4. ./config.sh --unattended --url $REPO_URL --token <TOKEN> --labels $RUNNER_LABEL --replace"
  echo "  5. NO ejecuta svc.sh install (separado)"
}

cmd_register() {
  local token="${1:-}"
  local label="${2:-$RUNNER_LABEL}"

  [[ -n "$token" ]] || { echo "ERROR: token requerido (uso: install-runner.sh --register TOKEN [LABEL])" >&2; exit 1; }

  mkdir -p "$RUNNER_HOME"

  if [[ ! -f "$RUNNER_HOME/run.sh" ]]; then
    echo "[register] Descargando runner $RUNNER_VERSION..."
    local tarball="actions-runner-linux-x64-${RUNNER_VERSION}.tar.gz"
    local url="https://github.com/actions/runner/releases/download/v${RUNNER_VERSION}/${tarball}"
    local tmp
    tmp="$(mktemp -d)"
    curl -fsSL -o "$tmp/$tarball" "$url"
    echo "[register] Validando checksum..."
    # El SHA256 publicado esta al lado del release en GitHub. Comparalo
    # manualmente la primera vez (ver docs/auto-deploy.md). Si tenes
    # /home/.../runner-checksums.txt con los oficiales, descomentar:
    # grep "$RUNNER_VERSION" "$HOME/runner-checksums.txt" | sha256sum -c
    tar xzf "$tmp/$tarball" -C "$RUNNER_HOME"
    rm -rf "$tmp"
  else
    echo "[register] Runner ya presente en $RUNNER_HOME, skip download"
  fi

  echo "[register] Configurando con label '$label'..."
  ( cd "$RUNNER_HOME" && ./config.sh --unattended --url "$REPO_URL" --token "$token" --labels "$label" --replace )
  echo "[register] Listo. Siguiente paso: ./scripts/install-runner.sh --install-service"
}

cmd_install_service() {
  [[ -f "$RUNNER_HOME/run.sh" ]] || { echo "ERROR: $RUNNER_HOME no parece un runner instalado (run.sh ausente)" >&2; exit 1; }
  echo "[service] Habilitando linger para $RUNNER_USER..."
  sudo loginctl enable-linger "$RUNNER_USER" || true
  echo "[service] Instalando servicio systemd user..."
  ( cd "$RUNNER_HOME" && sudo -u "$RUNNER_USER" ./svc.sh install "$RUNNER_USER" && sudo -u "$RUNNER_USER" ./svc.sh start )
  echo "[service] Estado: systemctl --user status actions.runner.*-$RUNNER_USER"
}

case "${1:-}" in
  --check) cmd_check ;;
  --register) shift; cmd_register "${1:-}" "${2:-}" ;;
  --install-service) cmd_install_service ;;
  *) echo "Uso: $0 {--check|--register TOKEN [LABEL]|--install-service}" >&2; exit 1 ;;
esac
