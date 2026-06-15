#!/bin/bash
# Bootstrap idempotente VPS — todo en uno.
#
# Uso típico:
#   bash install-vps.sh
#
# Flags opcionales:
#   --keep-clone        no borra /tmp/domain tras instalar (default: borra)
#   --skip-deps         no instala docker ni paquetes apt
#   --skip-compose-up   prepara archivos pero no levanta containers
#   --with-monitoring   levanta también Prometheus + Grafana en :3000
#   --non-interactive   no pregunta; requiere DOMAIN_ADMIN_EMAIL y
#                       DOMAIN_ADMIN_PASSWORD en env (sino aborta)
#
# Env vars opcionales (skipean el prompt si están seteadas):
#   DOMAIN_ORG_SLUG, DOMAIN_ORG_NAME
#   DOMAIN_ADMIN_EMAIL, DOMAIN_ADMIN_NAME, DOMAIN_ADMIN_PASSWORD
#
# Si no se corre como root, se re-ejecuta con sudo.
set -euo pipefail

if [[ $EUID -ne 0 ]]; then
  command -v sudo &>/dev/null || { echo "ERROR: sudo no instalado y no estás corriendo como root" >&2; exit 1; }
  echo "Re-ejecutando con sudo (puede pedir contraseña una vez)..."
  exec sudo -E bash "$0" "$@"
fi

INSTALL_DIR="/opt/services"
KEEP_CLONE=0
SKIP_DEPS=0
SKIP_COMPOSE_UP=0
WITH_MONITORING=0
NON_INTERACTIVE=0

for arg in "$@"; do
  case "$arg" in
    --keep-clone) KEEP_CLONE=1 ;;
    --skip-deps) SKIP_DEPS=1 ;;
    --skip-compose-up) SKIP_COMPOSE_UP=1 ;;
    --with-monitoring) WITH_MONITORING=1 ;;
    --non-interactive) NON_INTERACTIVE=1 ;;
    -h|--help) sed -n '2,17p' "$0"; exit 0 ;;
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

# Helper: contraseña aleatoria (32 chars alfanum URL-safe).
gen_pw() { openssl rand -base64 48 | tr -d '/+=' | head -c 32; }

# Helper: prompt con default. Si NON_INTERACTIVE=1 usa el default sin preguntar.
ask() {
  local prompt="$1" default="${2:-}" var
  if [[ $NON_INTERACTIVE -eq 1 ]]; then
    echo "$default"; return
  fi
  if [[ -n "$default" ]]; then
    read -rp "    $prompt [$default]: " var </dev/tty
  else
    read -rp "    $prompt: " var </dev/tty
  fi
  echo "${var:-$default}"
}

# Helper: prompt de password con confirmación.
ask_password() {
  if [[ $NON_INTERACTIVE -eq 1 ]]; then
    [[ -n "${DOMAIN_ADMIN_PASSWORD:-}" ]] || { fail "DOMAIN_ADMIN_PASSWORD requerida con --non-interactive"; exit 2; }
    echo "$DOMAIN_ADMIN_PASSWORD"; return
  fi
  local pw1 pw2
  while :; do
    read -rsp "    Contraseña admin (≥8 chars): " pw1 </dev/tty; echo
    [[ ${#pw1} -ge 8 ]] || { warn "muy corta, repetir"; continue; }
    read -rsp "    Confirmar contraseña:       " pw2 </dev/tty; echo
    [[ "$pw1" == "$pw2" ]] || { warn "no coinciden, repetir"; continue; }
    echo "$pw1"; return
  done
}

step "1/12  Preflight"
ok "root"

[[ -r /etc/os-release ]] || { fail "/etc/os-release no encontrado — OS no soportado"; exit 1; }
. /etc/os-release
if [[ "${ID:-}" != "ubuntu" ]]; then
  fail "OS = ${PRETTY_NAME:-desconocido}. Solo soportado: Ubuntu."
  exit 1
fi
ok "Ubuntu ${VERSION_ID:-?} (${VERSION_CODENAME:-?})"

command -v systemctl &>/dev/null || { fail "systemd no disponible"; exit 1; }
[[ -d /run/systemd/system ]] || { fail "systemd no es PID 1 (este host no usa systemd)"; exit 1; }
ok "systemd"

ARCH=$(dpkg --print-architecture 2>/dev/null || echo "?")
case "$ARCH" in
  amd64|arm64) ok "arch $ARCH" ;;
  *) fail "arquitectura no soportada: $ARCH"; exit 1 ;;
esac

step "1.5/12  Swap (red de seguridad para OOM)"
SWAP_BYTES=$(free -b | awk '/^Swap:/ {print $2}')
if [[ "$SWAP_BYTES" -ge $((2 * 1024 * 1024 * 1024)) ]]; then
  ok "swap ya configurada ($(free -h | awk '/^Swap:/ {print $2}'))"
elif [[ -f /swapfile ]]; then
  ok "/swapfile existe (no se modifica)"
else
  if [[ -w /etc/fstab ]]; then
    fallocate -l 2G /swapfile && chmod 600 /swapfile
    mkswap /swapfile >/dev/null && swapon /swapfile
    grep -q '^/swapfile' /etc/fstab || echo '/swapfile none swap sw 0 0' >> /etc/fstab
    ok "swap 2G creada y persistente"
  else
    warn "no se pudo crear swap (fs /etc/fstab no escribible)"
  fi
fi

step "2/12  Dependencias"
if [[ $SKIP_DEPS -eq 1 ]]; then
  warn "skip deps"
else
  apt-get update -qq
  apt-get install -y -qq ca-certificates curl gnupg lsb-release openssl gpg jq rsync make >/dev/null
  ok "base"

  if ! command -v docker &>/dev/null; then
    install -m 0755 -d /etc/apt/keyrings
    curl -fsSL https://download.docker.com/linux/ubuntu/gpg | gpg --dearmor -o /etc/apt/keyrings/docker.gpg
    chmod a+r /etc/apt/keyrings/docker.gpg
    echo "deb [arch=$ARCH signed-by=/etc/apt/keyrings/docker.gpg] \
https://download.docker.com/linux/ubuntu $(lsb_release -cs) stable" > /etc/apt/sources.list.d/docker.list
    apt-get update -qq
    apt-get install -y -qq docker-ce docker-ce-cli containerd.io docker-buildx-plugin docker-compose-plugin >/dev/null
    systemctl enable --now docker >/dev/null 2>&1 || true
    ok "docker instalado"
  else
    ok "docker presente ($(docker --version))"
  fi
fi

systemctl is-active --quiet docker || systemctl start docker || { fail "no se pudo iniciar docker daemon"; exit 1; }
docker info >/dev/null 2>&1 || { fail "docker daemon no responde"; exit 1; }
ok "docker daemon"

docker compose version &>/dev/null || { fail "docker compose plugin no disponible"; exit 1; }
ok "compose ($(docker compose version --short 2>/dev/null || echo presente))"

for compose_file in \
  postgres/docker-compose.yml \
  minio/docker-compose.yml \
  domain-backend/docker-compose.yml \
  domain-frontend/docker-compose.yml \
  caddy/docker-compose.yml; do
  (cd "$SOURCE_DIR" && docker compose -f "$compose_file" --env-file .env.example config -q 2>/dev/null) \
    || { fail "compose inválido: $compose_file"; exit 1; }
done
ok "5 composes válidos"

step "3/12  $INSTALL_DIR"
# El user real (no root, aunque corramos con sudo) sirve para los chown.
INVOKER_USER="${SUDO_USER:-${USER:-root}}"
INVOKER_GROUP=$(id -gn "$INVOKER_USER" 2>/dev/null || echo "$INVOKER_USER")

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

# Owner = user que ejecutó sudo, group = docker. Así el user puede
# leer .env y docker buildx sin permission errors.
chown -R "$INVOKER_USER:$INVOKER_GROUP" "$INSTALL_DIR"
if getent group docker >/dev/null 2>&1; then
  chgrp -R docker "$INSTALL_DIR" 2>/dev/null || true
  # .env y backups solo legibles por el dueño + group docker.
  [[ -f "$INSTALL_DIR/.env" ]] && chmod 640 "$INSTALL_DIR/.env"
fi
# El user debe estar en el grupo docker para no necesitar sudo en cada
# comando docker.
if [[ "$INVOKER_USER" != "root" ]] && ! id -nG "$INVOKER_USER" | grep -qw docker; then
  usermod -aG docker "$INVOKER_USER" 2>/dev/null && warn "agregado a grupo docker (re-login para activar)"
fi
# Buildx cache puede quedar root-owned si docker corrió antes con sudo.
[[ -d "/home/$INVOKER_USER/.docker" ]] && chown -R "$INVOKER_USER:$INVOKER_GROUP" "/home/$INVOKER_USER/.docker" 2>/dev/null || true
ok "permisos OK"

step "4/12  .env (auto-genera passwords)"
cd "$INSTALL_DIR"
if [[ ! -f .env ]]; then
  cp .env.example .env
  chmod 600 .env
  ok ".env creado desde example"
else
  ok ".env existe (no se sobrescribe)"
fi

# Reemplazar cada CHANGE_ME por una password aleatoria distinta.
# Idempotente: si ya no hay CHANGE_ME, no hace nada.
while grep -q "CHANGE_ME" .env; do
  PW=$(gen_pw)
  sed -i "0,/CHANGE_ME/{s,CHANGE_ME,${PW},}" .env
done
ok "secretos autogenerados"

grep -q "CHANGE_ME" .env && { fail ".env aún tiene CHANGE_ME"; exit 1; }
ok ".env OK"

step "5/12  Certs TLS"
./scripts/gen-certs.sh
ok "certs/"

step "6/12  systemd units"
for unit in systemd/*.service systemd/*.timer; do
  cp "$unit" "/etc/systemd/system/$(basename "$unit")"
done
systemctl daemon-reload
systemctl enable domain-services.service >/dev/null 2>&1
systemctl enable domain-services-backup.timer >/dev/null 2>&1
systemctl enable domain-services-healthcheck.timer >/dev/null 2>&1
ok "habilitados"

step "7/12  Build imágenes locales"
if [[ $SKIP_COMPOSE_UP -eq 1 ]]; then
  warn "skip build (--skip-compose-up activo)"
else
  make -C "$INSTALL_DIR" build || { fail "make build falló — revisar logs"; exit 1; }
  ok "imágenes buildeadas localmente"
fi

step "8/12  Servicios"
if [[ $SKIP_COMPOSE_UP -eq 1 ]]; then
  warn "skip (corré: make up)"
else
  make -C "$INSTALL_DIR" ensure-network
  make -C "$INSTALL_DIR" up
  ok "5 servicios up"

  systemctl start domain-services-backup.timer
  systemctl start domain-services-healthcheck.timer
  ok "timers iniciados"

  echo "    Esperando healthy..."
  for i in {1..45}; do
    sleep 2
    healthy=$(docker ps --filter health=healthy --format '{{.Names}}' \
              | grep -cE '^domain-(postgres|minio|backend|admin|caddy)$' || true)
    [[ "$healthy" -ge 5 ]] && { ok "los 5 healthy"; break; }
    [[ $i -eq 45 ]] && warn "timeout esperando healthy; revisar con: make ps && make logs SVC=<svc>"
  done
fi

step "9/12  Bootstrap organización + admin"
ADMIN_EMAIL=""
ORG_SLUG=""
if [[ $SKIP_COMPOSE_UP -eq 1 ]]; then
  warn "skip (servicios no arrancados)"
else
  set -a; source "$INSTALL_DIR/.env"; set +a
  PG_USER="${POSTGRES_USER:-domain}"
  PG_DB="${POSTGRES_DB:-domain}"
  pg() { docker exec -i domain-postgres psql -U "$PG_USER" -d "$PG_DB" -tAq "$@"; }

  ORG_SLUG="${DOMAIN_ORG_SLUG:-}"
  [[ -z "$ORG_SLUG" ]] && ORG_SLUG=$(ask "Slug de la organización" "default")
  ORG_NAME="${DOMAIN_ORG_NAME:-}"
  [[ -z "$ORG_NAME" ]] && ORG_NAME=$(ask "Nombre legible de la org" "$ORG_SLUG")

  ADMIN_EMAIL="${DOMAIN_ADMIN_EMAIL:-}"
  [[ -z "$ADMIN_EMAIL" ]] && ADMIN_EMAIL=$(ask "Email del admin" "admin@${ORG_SLUG}.local")
  ADMIN_NAME="${DOMAIN_ADMIN_NAME:-}"
  [[ -z "$ADMIN_NAME" ]] && ADMIN_NAME=$(ask "Nombre del admin" "Admin")
  ADMIN_PW=$(ask_password)

  # Crear org si no existe; devolver id.
  ORG_NAME_SQL=$(printf "%s" "$ORG_NAME" | sed "s/'/''/g")
  ORG_ID=$(pg -c "
    INSERT INTO organizations (slug, name)
    VALUES ('$ORG_SLUG', '$ORG_NAME_SQL')
    ON CONFLICT (slug) DO UPDATE SET name = EXCLUDED.name
    RETURNING id;
  " | tr -d '[:space:]')
  [[ -n "$ORG_ID" ]] || { fail "no se pudo obtener org id"; exit 1; }
  ok "org '$ORG_SLUG' ($ORG_ID)"

  # Crear user si no existe (la password va aparte vía admin-passwd).
  ADMIN_NAME_SQL=$(printf "%s" "$ADMIN_NAME" | sed "s/'/''/g")
  pg -c "
    INSERT INTO users (organization_id, email, name, role)
    VALUES ('$ORG_ID', '$ADMIN_EMAIL', '$ADMIN_NAME_SQL', 'admin')
    ON CONFLICT (organization_id, email) DO NOTHING;
  " >/dev/null
  ok "user $ADMIN_EMAIL"

  # Setear password + asignar rol admin vía CLI.
  DSN="postgres://app_admin:${APP_ADMIN_PASSWORD}@postgres:5432/${PG_DB}?sslmode=disable"
  if echo "$ADMIN_PW" | docker run --rm -i --network domain_internal \
        -e DOMAIN_DATABASE_AUTH_URL="$DSN" \
        domain-backend:local admin-passwd "$ADMIN_EMAIL" --role=admin >/dev/null 2>&1; then
    ok "password seteada + rol admin asignado"
  else
    fail "admin-passwd falló (revisar: docker logs domain-backend)"
    exit 1
  fi
fi

step "10/12  Monitoring opcional"
if [[ $WITH_MONITORING -eq 1 && $SKIP_COMPOSE_UP -eq 0 ]]; then
  MON_COMPOSE="$INSTALL_DIR/domain-backend/deploy/monitoring/docker-compose.yml"
  if [[ -f "$MON_COMPOSE" ]]; then
    docker compose -f "$MON_COMPOSE" --env-file "$INSTALL_DIR/.env" up -d --wait >/dev/null
    ok "prometheus + grafana up (Grafana en :3000)"
  else
    warn "monitoring compose no encontrado en $MON_COMPOSE"
  fi
else
  ok "skip (--with-monitoring no activado)"
fi

step "11/12  Cleanup"
if [[ -n "$MOVED_FROM" && $KEEP_CLONE -eq 0 && "$MOVED_FROM" != "$INSTALL_DIR" ]]; then
  rm -rf "$MOVED_FROM"; ok "clone eliminado: $MOVED_FROM"
else
  ok "nada que limpiar"
fi

# domain-frontend legacy: el dashboard nuevo lo reemplaza. Si todavía
# existe (de un install previo), lo bajamos para liberar RAM/disco.
if docker ps -a --format '{{.Names}}' | grep -qx domain-frontend; then
  docker stop domain-frontend >/dev/null 2>&1 || true
  docker rm domain-frontend   >/dev/null 2>&1 || true
  docker image rm domain-frontend:local >/dev/null 2>&1 || true
  ok "domain-frontend legacy eliminado"
fi

# Build cache acumulado tras build/rebuild. Lo limpiamos automáticamente
# si supera 1GB para no llenar el disco con capas viejas.
CACHE_BYTES=$(docker buildx du 2>/dev/null | awk 'NR>1 {sum+=$2} END {print sum+0}')
if [[ "${CACHE_BYTES:-0}" -gt 1073741824 ]]; then
  docker builder prune -af --filter "unused-for=24h" >/dev/null 2>&1 || true
  ok "build cache limpiado"
fi

step "12/12  Resumen"
set -a; source "$INSTALL_DIR/.env"; set +a
VPS_IP="${VPS_PUBLIC_IP:-$(curl -fsS --max-time 3 https://ifconfig.me 2>/dev/null || echo '<ip>')}"

cat <<RESUMEN

${GREEN}${BOLD}domain-services listo${RESET}

  Dashboard:   http://$VPS_IP/
  API:         http://$VPS_IP/api/v1/...
  MCP HTTP:    http://$VPS_IP/mcp
  Healthz:     http://$VPS_IP/healthz
RESUMEN

if [[ $WITH_MONITORING -eq 1 ]]; then
cat <<MON
  Grafana:     http://$VPS_IP:3000  (admin / ${GRAFANA_ADMIN_PASSWORD:-admin})
MON
fi

if [[ -n "$ADMIN_EMAIL" ]]; then
cat <<CREDS

  ${BOLD}Login del dashboard${RESET}
    Email:     $ADMIN_EMAIL
    Password:  (la que ingresaste durante la instalación)
    Org slug:  $ORG_SLUG

CREDS
fi

cat <<TAIL
  Backups:     diario 02:00 UTC → $INSTALL_DIR/backups/
  Alerts:      ntfy.sh/${NTFY_TOPIC:-<no-configurado>}

  Comandos útiles:
    cd $INSTALL_DIR
    make ps                       # estado de los 5
    make logs SVC=backend         # tail de uno
    make restart SVC=backend      # update sin tocar otros
    make backup                   # backup manual

TAIL
