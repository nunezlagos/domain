#!/bin/bash
# ============================================================================
# install.sh — bootstrap domain-services en el VPS.
#
# Idempotente: re-correr no rompe nada.
#
# Pasos:
#   1. Verifica que corre como root (necesario para apt install + /opt + systemd)
#   2. Instala deps: docker, docker compose plugin, openssl, gpg, curl, jq
#   3. mkdir /opt/services y mueve los archivos (si todavía no estamos ahí)
#   4. Genera certs TLS self-signed (scripts/gen-certs.sh)
#   5. Copia .env.example → .env si no existe; alerta si vars sin completar
#   6. Instala unidades systemd (start + backup timer + healthcheck timer)
#   7. docker compose up -d (perfil core)
#   8. Elimina el clone original SI install.sh fue corrido desde ahí
#   9. Print resumen con IPs/puertos/próximos pasos
#
# Uso:
#   sudo ./install.sh                    # bootstrap completo
#   sudo ./install.sh --keep-clone       # no elimina el clone original al final
#   sudo ./install.sh --skip-deps        # asume docker/openssl/gpg ya instalados
#   sudo ./install.sh --skip-compose-up  # configura todo pero no levanta containers
# ============================================================================
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
    -h|--help)
      sed -n '2,30p' "$0"
      exit 0
      ;;
    *)
      echo "ERROR: flag desconocida: $arg" >&2
      exit 2
      ;;
  esac
done

# Colores para output legible
BOLD=$'\033[1m'
RESET=$'\033[0m'
GREEN=$'\033[32m'
YELLOW=$'\033[33m'
RED=$'\033[31m'

step()  { echo ""; echo "${BOLD}==> $1${RESET}"; }
ok()    { echo "${GREEN}    ✓${RESET} $1"; }
warn()  { echo "${YELLOW}    !${RESET} $1"; }
fail()  { echo "${RED}    ✗${RESET} $1" >&2; }

SOURCE_DIR="$( cd "$( dirname "${BASH_SOURCE[0]}" )" && pwd )"

# ----------------------------------------------------------------------------
# 1. Root check
# ----------------------------------------------------------------------------
step "1/9  Verificando privilegios"
if [[ $EUID -ne 0 ]]; then
  fail "Hay que correr como root (sudo)."
  exit 1
fi
ok "Ejecutando como root"

# ----------------------------------------------------------------------------
# 2. Install deps
# ----------------------------------------------------------------------------
step "2/9  Instalando dependencias del host"
if [[ $SKIP_DEPS -eq 1 ]]; then
  warn "Skip --skip-deps; asumiendo deps presentes"
else
  apt-get update -qq
  apt-get install -y -qq \
    ca-certificates curl gnupg lsb-release \
    openssl gpg jq \
    >/dev/null
  ok "ca-certificates curl gnupg openssl gpg jq"

  # Docker (oficial repo)
  if ! command -v docker &>/dev/null; then
    install -m 0755 -d /etc/apt/keyrings
    curl -fsSL https://download.docker.com/linux/ubuntu/gpg \
      | gpg --dearmor -o /etc/apt/keyrings/docker.gpg
    chmod a+r /etc/apt/keyrings/docker.gpg
    echo "deb [arch=$(dpkg --print-architecture) signed-by=/etc/apt/keyrings/docker.gpg] \
https://download.docker.com/linux/ubuntu $(lsb_release -cs) stable" \
      > /etc/apt/sources.list.d/docker.list
    apt-get update -qq
    apt-get install -y -qq docker-ce docker-ce-cli containerd.io docker-buildx-plugin docker-compose-plugin >/dev/null
    systemctl enable --now docker >/dev/null 2>&1 || true
    ok "Docker instalado"
  else
    ok "Docker ya presente ($(docker --version | head -1))"
  fi

  if ! docker compose version &>/dev/null; then
    fail "docker compose plugin no disponible"
    exit 1
  fi
  ok "docker compose plugin OK"
fi

# ----------------------------------------------------------------------------
# 3. Move a /opt/services
# ----------------------------------------------------------------------------
step "3/9  Configurando $INSTALL_DIR"
if [[ "$SOURCE_DIR" == "$INSTALL_DIR" ]]; then
  ok "Ya estamos en $INSTALL_DIR (re-install)"
  MOVED_FROM=""
elif [[ -e "$INSTALL_DIR" && "$(ls -A "$INSTALL_DIR" 2>/dev/null | head -1)" ]]; then
  warn "$INSTALL_DIR ya existe y no está vacío"
  warn "Vamos a fusionar (los archivos del clone sobreescriben los existentes)."
  # rsync los archivos del clone sobre /opt/services
  rsync -a --exclude='.git' "$SOURCE_DIR/" "$INSTALL_DIR/"
  MOVED_FROM="$SOURCE_DIR"
  ok "Archivos sincronizados a $INSTALL_DIR"
else
  mkdir -p "$INSTALL_DIR"
  # Copiamos en vez de mover para que --keep-clone funcione consistente
  rsync -a --exclude='.git' "$SOURCE_DIR/" "$INSTALL_DIR/"
  MOVED_FROM="$SOURCE_DIR"
  ok "Archivos copiados a $INSTALL_DIR"
fi

# Hacer ejecutables los scripts después del rsync
chmod +x "$INSTALL_DIR/scripts/"*.sh 2>/dev/null || true
chmod +x "$INSTALL_DIR/postgres/init/"*.sh 2>/dev/null || true
chmod +x "$INSTALL_DIR/install.sh" 2>/dev/null || true

# ----------------------------------------------------------------------------
# 4. .env
# ----------------------------------------------------------------------------
step "4/9  Configurando .env"
cd "$INSTALL_DIR"
if [[ ! -f .env ]]; then
  cp .env.example .env
  chmod 600 .env
  warn "Creado .env desde .env.example"
  warn "  ↪ Tenés que editar /opt/services/.env y poner passwords reales antes del paso siguiente."
  warn "  ↪ Generá passwords fuertes con: openssl rand -base64 48 | tr -d '/+=' | head -c 32"
  echo ""
  read -rp "  Presioná ENTER cuando hayas terminado de editar .env, o Ctrl-C para abortar... "
else
  ok ".env ya existe (no sobreescribo)"
fi

# Validar que no quedaron placeholders CHANGE_ME
if grep -q "CHANGE_ME" .env; then
  fail ".env todavía contiene placeholders CHANGE_ME. Editalo y volvé a correr."
  exit 1
fi
ok ".env parece tener todos los valores rellenados"

# ----------------------------------------------------------------------------
# 5. Certs TLS
# ----------------------------------------------------------------------------
step "5/9  Generando certs TLS self-signed"
./scripts/gen-certs.sh
ok "Certs generados en $INSTALL_DIR/certs/"

# ----------------------------------------------------------------------------
# 6. systemd units
# ----------------------------------------------------------------------------
step "6/9  Instalando units systemd"
for unit in systemd/*.service systemd/*.timer; do
  target="/etc/systemd/system/$(basename "$unit")"
  cp "$unit" "$target"
  ok "$target"
done
systemctl daemon-reload
systemctl enable domain-services.service >/dev/null 2>&1
systemctl enable domain-services-backup.timer >/dev/null 2>&1
systemctl enable domain-services-healthcheck.timer >/dev/null 2>&1
ok "Units habilitados (auto-start en boot)"

# ----------------------------------------------------------------------------
# 7. docker compose up
# ----------------------------------------------------------------------------
step "7/9  Levantando servicios"
if [[ $SKIP_COMPOSE_UP -eq 1 ]]; then
  warn "Skip --skip-compose-up; levantá manual con: cd $INSTALL_DIR && make up"
else
  systemctl start domain-services.service
  systemctl start domain-services-backup.timer
  systemctl start domain-services-healthcheck.timer
  ok "domain-services.service iniciado (compose up -d ejecutado)"

  # Esperar healthy hasta 60s
  echo "    Esperando que los containers estén healthy..."
  for i in {1..30}; do
    sleep 2
    if docker compose ps --format json 2>/dev/null | grep -q '"Health":"healthy"'; then
      ok "Servicios reportan healthy"
      break
    fi
    if [[ $i -eq 30 ]]; then
      warn "Timeout esperando healthy (revisar con make logs)"
    fi
  done
fi

# ----------------------------------------------------------------------------
# 8. Cleanup clone original
# ----------------------------------------------------------------------------
step "8/9  Cleanup"
if [[ -n "$MOVED_FROM" && $KEEP_CLONE -eq 0 ]]; then
  if [[ "$MOVED_FROM" != "$INSTALL_DIR" ]]; then
    rm -rf "$MOVED_FROM"
    ok "Clone original eliminado: $MOVED_FROM"
  fi
else
  ok "No hay clone para limpiar (o --keep-clone)"
fi

# ----------------------------------------------------------------------------
# 9. Resumen
# ----------------------------------------------------------------------------
step "9/9  Resumen"
set -a
# shellcheck disable=SC1091
source "$INSTALL_DIR/.env"
set +a

VPS_IP="${VPS_PUBLIC_IP:-$(curl -fsS --max-time 3 https://ifconfig.me 2>/dev/null || echo '<ip-no-detectada>')}"

cat <<RESUMEN

${BOLD}=========================================================${RESET}
${GREEN}${BOLD}  domain-services LISTO${RESET}
${BOLD}=========================================================${RESET}

  ${BOLD}Postgres${RESET}
    Host:     $VPS_IP
    Puerto:   ${PG_PORT:-5432}
    DB:       ${POSTGRES_DB:-domain}
    User:     ${POSTGRES_USER:-domain}  (también: app_user, app_admin, app_migrator)
    SSL:      requerido (sslmode=require)
    DSN:      postgres://app_user:***@$VPS_IP:${PG_PORT:-5432}/${POSTGRES_DB:-domain}?sslmode=require

  ${BOLD}MinIO${RESET}
    API:      https://$VPS_IP:${MINIO_API_PORT:-9000}
    Console:  http://$VPS_IP:${MINIO_CONSOLE_PORT:-9001}  (UI web)
    Bucket:   ${MINIO_DEFAULT_BUCKET:-domain-attachments}
    Root:     ${MINIO_ROOT_USER:-domain-admin}

  ${BOLD}Backups${RESET}
    Schedule:  diario 02:00 UTC (systemd timer)
    Destino:   $INSTALL_DIR/backups/
    Estado:    systemctl status domain-services-backup.timer

  ${BOLD}Healthcheck${RESET}
    Cada 5min, notifica a ntfy si algo falla.
    Topic ntfy: ${NTFY_TOPIC:-<no configurado>}

${BOLD}Próximos pasos:${RESET}
  1. Desde tu laptop, configurá domain con el DSN de arriba (sslmode=require)
  2. Probá conexión: psql "<DSN>" -c "SELECT version();"
  3. Corré las migrations de domain con app_migrator:
       DOMAIN_DATABASE_URL="postgres://app_migrator:***@$VPS_IP:5432/domain?sslmode=require" \\
       domain migrate up
  4. Verificá MinIO en https://$VPS_IP:${MINIO_API_PORT:-9000}
       (browser dará warning por cert self-signed; aceptalo)
  5. Suscribite al ntfy topic: https://ntfy.sh/${NTFY_TOPIC:-<sin-topic>}

${BOLD}Comandos útiles:${RESET}
  cd $INSTALL_DIR
  make ps              # estado containers
  make logs            # tail logs
  make logs SVC=postgres
  make backup          # backup manual
  make psql            # shell SQL
  make certs-force     # renovar certs TLS

RESUMEN
