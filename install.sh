#!/bin/bash
# Bootstrap idempotente: sudo ./install.sh [--keep-clone] [--skip-deps] [--skip-compose-up]
set -euo pipefail

INSTALL_DIR="/opt/services"
KEEP_CLONE=0
SKIP_DEPS=0
SKIP_COMPOSE_UP=0

for arg in "$@"; do
  case "$arg" in
    --keep-clone) KEEP_CLONE=1 ;;
    --skip-deps) SKIP_DEPS=1 ;;
    --skip-compose-up) SKIP_COMPOSE_UP=1 ;;
    -h|--help) sed -n '2,3p' "$0"; exit 0 ;;
    *) echo "ERROR: flag desconocida: $arg" >&2; exit 2 ;;
  esac
done

BOLD=$'\033[1m'; RESET=$'\033[0m'
GREEN=$'\033[32m'; YELLOW=$'\033[33m'; RED=$'\033[31m'
step() { echo ""; echo "${BOLD}==> $1${RESET}"; }
ok()   { echo "${GREEN}    ✓${RESET} $1"; }
warn() { echo "${YELLOW}    !${RESET} $1"; }
fail() { echo "${RED}    ✗${RESET} $1" >&2; }

SOURCE_DIR="$( cd "$( dirname "${BASH_SOURCE[0]}" )" && pwd )"

step "1/9  Privilegios"
[[ $EUID -ne 0 ]] && { fail "root requerido"; exit 1; }
ok "root"

step "2/9  Dependencias"
if [[ $SKIP_DEPS -eq 1 ]]; then
  warn "skip deps"
else
  apt-get update -qq
  apt-get install -y -qq ca-certificates curl gnupg lsb-release openssl gpg jq rsync >/dev/null
  ok "base"

  if ! command -v docker &>/dev/null; then
    install -m 0755 -d /etc/apt/keyrings
    curl -fsSL https://download.docker.com/linux/ubuntu/gpg | gpg --dearmor -o /etc/apt/keyrings/docker.gpg
    chmod a+r /etc/apt/keyrings/docker.gpg
    echo "deb [arch=$(dpkg --print-architecture) signed-by=/etc/apt/keyrings/docker.gpg] \
https://download.docker.com/linux/ubuntu $(lsb_release -cs) stable" > /etc/apt/sources.list.d/docker.list
    apt-get update -qq
    apt-get install -y -qq docker-ce docker-ce-cli containerd.io docker-buildx-plugin docker-compose-plugin >/dev/null
    systemctl enable --now docker >/dev/null 2>&1 || true
    ok "docker instalado"
  else
    ok "docker presente"
  fi

  docker compose version &>/dev/null || { fail "docker compose no disponible"; exit 1; }
  ok "compose"
fi

step "3/9  $INSTALL_DIR"
if [[ "$SOURCE_DIR" == "$INSTALL_DIR" ]]; then
  ok "re-install"
  MOVED_FROM=""
else
  mkdir -p "$INSTALL_DIR"
  rsync -a --exclude='.git' "$SOURCE_DIR/" "$INSTALL_DIR/"
  MOVED_FROM="$SOURCE_DIR"
  ok "sincronizado"
fi
chmod +x "$INSTALL_DIR/scripts/"*.sh "$INSTALL_DIR/postgres/init/"*.sh "$INSTALL_DIR/install.sh" 2>/dev/null || true

step "4/9  .env"
cd "$INSTALL_DIR"
if [[ ! -f .env ]]; then
  cp .env.example .env
  chmod 600 .env
  warn "Editá /opt/services/.env y poné passwords reales."
  warn "Generar: openssl rand -base64 48 | tr -d '/+=' | head -c 32"
  read -rp "  ENTER cuando esté listo... "
else
  ok ".env existe"
fi
grep -q "CHANGE_ME" .env && { fail ".env aún tiene CHANGE_ME"; exit 1; }
ok ".env OK"

step "5/9  Certs TLS"
./scripts/gen-certs.sh
ok "certs/"

step "6/9  systemd units"
for unit in systemd/*.service systemd/*.timer; do
  cp "$unit" "/etc/systemd/system/$(basename "$unit")"
done
systemctl daemon-reload
systemctl enable domain-services.service >/dev/null 2>&1
systemctl enable domain-services-backup.timer >/dev/null 2>&1
systemctl enable domain-services-healthcheck.timer >/dev/null 2>&1
ok "habilitados"

step "7/9  Servicios"
if [[ $SKIP_COMPOSE_UP -eq 1 ]]; then
  warn "skip (corré: make up)"
else
  systemctl start domain-services.service
  systemctl start domain-services-backup.timer
  systemctl start domain-services-healthcheck.timer
  ok "iniciados"

  echo "    Esperando healthy..."
  for i in {1..30}; do
    sleep 2
    healthy=$(docker ps --filter health=healthy --format '{{.Names}}' | grep -cE '^domain-(postgres|minio)$' || true)
    [[ "$healthy" -ge 2 ]] && { ok "healthy"; break; }
    [[ $i -eq 30 ]] && warn "timeout esperando healthy"
  done
fi

step "8/9  Cleanup"
if [[ -n "$MOVED_FROM" && $KEEP_CLONE -eq 0 && "$MOVED_FROM" != "$INSTALL_DIR" ]]; then
  rm -rf "$MOVED_FROM"; ok "clone eliminado: $MOVED_FROM"
else
  ok "nada que limpiar"
fi

step "9/9  Resumen"
set -a; source "$INSTALL_DIR/.env"; set +a
VPS_IP="${VPS_PUBLIC_IP:-$(curl -fsS --max-time 3 https://ifconfig.me 2>/dev/null || echo '<ip>')}"

cat <<RESUMEN

${GREEN}${BOLD}domain-services listo${RESET}

  Postgres: $VPS_IP:${PG_PORT:-5432}  db=${POSTGRES_DB} user=${POSTGRES_USER}  sslmode=require
  MinIO:    https://$VPS_IP:${MINIO_API_PORT:-9000}  console=http://$VPS_IP:${MINIO_CONSOLE_PORT:-9001}
  Bucket:   ${MINIO_DEFAULT_BUCKET:-domain-attachments}
  Backups:  diario 02:00 UTC → $INSTALL_DIR/backups/
  Alerts:   ntfy.sh/${NTFY_TOPIC:-<no-configurado>}

  cd $INSTALL_DIR && make {ps,logs,backup,psql,certs-force}

RESUMEN
