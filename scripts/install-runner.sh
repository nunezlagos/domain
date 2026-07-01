#!/usr/bin/env bash
# scripts/install-runner.sh
#
# Registra el GitHub Actions self-hosted runner en el VPS y lo deja como
# SERVICIO systemd de USUARIO persistente (Restart=always + linger = arranca al
# boot sin login) para que el deploy on push a main funcione SIEMPRE, sin dejar
# terminales abiertas. (REQ-53 / HU 38.12; runner de usuario, sin root.)
#
# Correr como el usuario no-root del deploy (ej. sysadmin) EN EL VPS.
#
# Uso:
#   ./scripts/install-runner.sh --check
#       Muestra qué haría, sin actuar.
#   ./scripts/install-runner.sh --register TOKEN [LABEL]
#       Descarga runner y lo registra (token: GitHub → repo Settings → Actions →
#       Runners → New self-hosted runner → el token de ./config.sh --token XXX).
#   ./scripts/install-runner.sh --install-service
#       Setea DEPLOY_LOG_DIR + linger + unit systemd --user + enable --now.
#   ./scripts/install-runner.sh --all TOKEN [LABEL]
#       register + install-service de una.
#
# Vars (override por env): RUNNER_VERSION, RUNNER_USER, RUNNER_HOME, RUNNER_LABEL,
#   REPO_URL, DEPLOY_LOG_DIR.

set -euo pipefail

RUNNER_VERSION="${RUNNER_VERSION:-2.319.1}"
RUNNER_USER="${RUNNER_USER:-$(whoami)}"
RUNNER_HOME="${RUNNER_HOME:-$HOME/actions-runner}"
RUNNER_LABEL="${RUNNER_LABEL:-deploy-vps}"
REPO_URL="${REPO_URL:-https://github.com/nunezlagos/domain}"
# deploy.yml escribe el log del run en $DEPLOY_LOG_DIR (tee). Si no está seteado
# en el env del runner, el step Deploy falla. Lo dejamos en .env del runner.
DEPLOY_LOG_DIR="${DEPLOY_LOG_DIR:-$HOME/deploy-logs}"

cmd_check() {
  echo "=== install-runner --check ==="
  echo "runner_version: $RUNNER_VERSION"
  echo "runner_user:    $RUNNER_USER"
  echo "runner_home:    $RUNNER_HOME"
  echo "runner_label:   $RUNNER_LABEL  (deploy.yml exige el label 'deploy-vps')"
  echo "repo_url:       $REPO_URL"
  echo "deploy_log_dir: $DEPLOY_LOG_DIR"
  echo
  echo "--register: descarga runner, config.sh --unattended --labels $RUNNER_LABEL --replace"
  echo "--install-service:"
  echo "  1. mkdir $DEPLOY_LOG_DIR + escribirlo en $RUNNER_HOME/.env"
  echo "  2. sudo loginctl enable-linger $RUNNER_USER  (arranca sin login)"
  echo "  3. unit systemd --user actions-runner.service (ExecStart=run.sh, Restart=always)"
  echo "  4. systemctl --user enable --now actions-runner.service"
}

cmd_register() {
  local token="${1:-}"
  local label="${2:-$RUNNER_LABEL}"
  [[ -n "$token" ]] || { echo "ERROR: token requerido (--register TOKEN [LABEL])" >&2; exit 1; }

  mkdir -p "$RUNNER_HOME"
  if [[ ! -f "$RUNNER_HOME/run.sh" ]]; then
    echo "[register] Descargando runner $RUNNER_VERSION..."
    local arch="linux-x64"
    case "$(uname -m)" in aarch64|arm64) arch="linux-arm64" ;; esac
    local tarball="actions-runner-${arch}-${RUNNER_VERSION}.tar.gz"
    local url="https://github.com/actions/runner/releases/download/v${RUNNER_VERSION}/${tarball}"
    local tmp; tmp="$(mktemp -d)"
    curl -fsSL -o "$tmp/$tarball" "$url"
    # Checksum: comparar contra el SHA256 publicado por GitHub en el release
    # (ver docs/auto-deploy.md). Si tenés ~/runner-checksums.txt oficial:
    #   grep "$tarball" ~/runner-checksums.txt | (cd "$tmp" && sha256sum -c -)
    tar xzf "$tmp/$tarball" -C "$RUNNER_HOME"
    rm -rf "$tmp"
  else
    echo "[register] Runner ya presente en $RUNNER_HOME, skip download"
  fi

  echo "[register] Configurando con label '$label' (idempotente, --replace)..."
  ( cd "$RUNNER_HOME" && ./config.sh --unattended --url "$REPO_URL" --token "$token" --labels "$label" --replace )
  echo "[register] OK. Seguí con: ./scripts/install-runner.sh --install-service"
}

cmd_install_service() {
  [[ -f "$RUNNER_HOME/run.sh" ]] || { echo "ERROR: $RUNNER_HOME sin run.sh (registrá primero con --register)" >&2; exit 1; }

  echo "[service] DEPLOY_LOG_DIR=$DEPLOY_LOG_DIR (para el log del deploy.yml)"
  mkdir -p "$DEPLOY_LOG_DIR"
  if grep -q '^DEPLOY_LOG_DIR=' "$RUNNER_HOME/.env" 2>/dev/null; then
    sed -i "s#^DEPLOY_LOG_DIR=.*#DEPLOY_LOG_DIR=$DEPLOY_LOG_DIR#" "$RUNNER_HOME/.env"
  else
    echo "DEPLOY_LOG_DIR=$DEPLOY_LOG_DIR" >> "$RUNNER_HOME/.env"
  fi

  echo "[service] Habilitando linger para $RUNNER_USER (servicio user arranca al boot sin login)..."
  sudo loginctl enable-linger "$RUNNER_USER"

  echo "[service] Escribiendo unit systemd --user..."
  local unit_dir="$HOME/.config/systemd/user"
  mkdir -p "$unit_dir"
  cat > "$unit_dir/actions-runner.service" <<EOF
[Unit]
Description=GitHub Actions self-hosted runner (domain deploy-vps)
After=network-online.target
Wants=network-online.target

[Service]
ExecStart=$RUNNER_HOME/run.sh
WorkingDirectory=$RUNNER_HOME
Restart=always
RestartSec=5
KillMode=process

[Install]
WantedBy=default.target
EOF

  systemctl --user daemon-reload
  systemctl --user enable --now actions-runner.service
  echo "[service] Estado:"
  systemctl --user --no-pager status actions-runner.service || true
  echo "[service] Listo: Restart=always + linger => el runner corre SIEMPRE (boot + auto-restart)."
  echo "[service] Verificá en GitHub → repo Settings → Actions → Runners que figure 'Idle'."
}

case "${1:-}" in
  --check) cmd_check ;;
  --register) shift; cmd_register "${1:-}" "${2:-}" ;;
  --install-service) cmd_install_service ;;
  --all) shift; cmd_register "${1:-}" "${2:-}" && cmd_install_service ;;
  *) echo "Uso: $0 {--check|--register TOKEN [LABEL]|--install-service|--all TOKEN [LABEL]}" >&2; exit 1 ;;
esac
