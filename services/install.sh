#!/usr/bin/env bash
# services/install.sh — único bootstrap del stack domain-services.
#
# USO:
#   sudo bash services/install.sh                  # install fresco o reinstall (preserva credenciales)
#
# REQUISITOS:
#   - Ubuntu 22.04+ (amd64 o arm64)
#   - systemd como PID 1
#   - Acceso a internet (para clonar el repo, descargar imágenes)
#
# COMPORTAMIENTO:
#   - En install fresco: genera UUID v4 para cada credencial y la imprime al final
#   - En reinstall: lee /opt/services/.env existente y preserva credenciales
#   - Idempotente: correlo N veces, el estado final es consistente
#
# NO TOCA:
#   - /opt/services/.env (lo lee, lo regenera solo si no existe o falta algo)
#   - /opt/services/certs/ (los preserva)
#   - /opt/services/backups/ (los preserva)

set -euo pipefail

# === Config ===
INSTALL_DIR="${INSTALL_DIR:-/opt/services}"
REPO_URL="${REPO_URL:-https://github.com/nunezlagos/domain.git}"
REPO_BRANCH="${REPO_BRANCH:-services}"

# Si el script no se corre como root pero SUDO_PASSWORD está seteada,
# usamos sudo -S (lee password de stdin). Útil para VPS donde el user
# no tiene NOPASSWD. Si corrés como root, este helper es no-op.
sudo_run() {
  if [[ $EUID -eq 0 ]]; then
    "$@"
  elif [[ -n "${SUDO_PASSWORD:-}" ]]; then
    echo "$SUDO_PASSWORD" | sudo -S -p '' "$@"
  else
    # Último intento: sudo sin password (NOPASSWD configurado)
    sudo "$@"
  fi
}

# === Logging ===
log()  { printf '\033[36m[install]\033[0m %s\n' "$*" >&2; }
ok()   { log "✓ $*"; }
fail() { log "✗ $*"; exit 1; }
warn() { log "! $*"; }

# === Helpers ===

# UUID v4 desde /dev/urandom (sin dependencia de uuidgen o python).
# Formato: xxxxxxxx-xxxx-4xxx-yxxx-xxxxxxxxxxxx (RFC 4122 v4)
# 122 bits de entropía. Posición 13 es siempre '4' (versión).
# Posición 17 es 8/9/a/b (variant 10xx).
gen_uuid() {
  local hex
  hex=$(head -c 16 /dev/urandom | od -An -tx1 | tr -d ' \n')
  # hex = 32 chars (e.g. "7bcb2e6d62fb4b53a31428d4143f090b")
  # Set version (4) at position 12
  # Set variant (10xx) at position 16 → 8/9/a/b
  local var_hex
  var_hex=$(printf '%x' $(( 0x${hex:16:1} & 0x3 | 0x8 )))
  printf '%s-%s-4%s-%s%s-%s\n' \
    "${hex:0:8}"   \
    "${hex:8:4}"   \
    "${hex:13:3}"  \
    "$var_hex"     \
    "${hex:17:3}"  \
    "${hex:20:12}"
}

# Lee un valor de un .env existente (sin importar quoting).
# Siempre retorna 0 (echo vacío si no encuentra) — necesario para set -e.
env_get() {
  local key="$1" file="$2"
  [[ -f "$file" ]] || { echo ""; return 0; }
  grep -E "^${key}=" "$file" 2>/dev/null | head -1 | cut -d= -f2- | sed -E "s/^['\"]//; s/['\"]$//" || echo ""
}

# Aplica un valor al .env (preserva quoting)
env_set() {
  local key="$1" value="$2" file="$3"
  if grep -qE "^${key}=" "$file" 2>/dev/null; then
    sed -i.bak -E "s|^${key}=.*|${key}=${value}|" "$file" && rm -f "${file}.bak"
  else
    printf '%s=%s\n' "$key" "$value" >> "$file"
  fi
}

# === STEP 1: Validate OS ===
log "1/8  Validating OS..."
. /etc/os-release 2>/dev/null || fail "/etc/os-release no encontrado"
[[ "${ID:-}" == "ubuntu" ]] || fail "OS no soportada. Solo Ubuntu. Detectado: ${PRETTY_NAME:-desconocido}"
command -v systemctl &>/dev/null || fail "systemd no disponible"
[[ -d /run/systemd/system ]] || fail "systemd no es PID 1"
ARCH="$(uname -m)"
# Linux reporta x86_64 o amd64 según distro; aceptamos ambos
case "$ARCH" in
  amd64|x86_64) ARCH=amd64 ;;
  arm64|aarch64) ARCH=arm64 ;;
  *) fail "arquitectura no soportada: $ARCH" ;;
esac
ok "Ubuntu ${VERSION_ID} (${VERSION_CODENAME}) — ${ARCH}"

# === STEP 2: Docker ===
log "2/8  Checking Docker..."
if ! command -v docker &>/dev/null; then
  warn "Docker no instalado, instalando..."
  apt-get update -qq
  apt-get install -y -qq docker.io docker-compose-plugin
  ok "Docker instalado"
else
  ok "Docker presente ($(docker --version | head -1))"
fi
systemctl is-active --quiet docker || systemctl start docker
docker info >/dev/null 2>&1 || fail "Docker daemon no responde"
ok "Docker daemon OK"

# === STEP 3: Clone or pull repo ===
log "3/8  Setting up repo at $INSTALL_DIR..."
if [[ -d "$INSTALL_DIR/.git" ]]; then
  log "Repo ya clonado, git pull..."
  (cd "$INSTALL_DIR" && git fetch origin "$REPO_BRANCH" && git reset --hard "origin/$REPO_BRANCH")
  ok "Repo actualizado a origin/$REPO_BRANCH"
elif [[ -d "$INSTALL_DIR" ]] && [[ -n "$(ls -A "$INSTALL_DIR" 2>/dev/null)" ]]; then
  # Existe pero no es git (ej: archivos copiados por rsync o install viejo).
  # Inicializamos git apuntando al repo oficial y sincronizamos.
  log "$INSTALL_DIR existe sin .git, inicializando + pulling..."
  (cd "$INSTALL_DIR" && git init -q && git remote add origin "$REPO_URL" && \
    git fetch origin "$REPO_BRANCH" && git reset --hard "origin/$REPO_BRANCH")
  ok "Git inicializado y sincronizado con origin/$REPO_BRANCH"
else
  mkdir -p "$INSTALL_DIR"
  git clone -b "$REPO_BRANCH" "$REPO_URL" "$INSTALL_DIR"
  ok "Repo clonado"
fi

# === STEP 4: Generate or preserve .env ===
log "4/8  Configurando credenciales..."
ENV_FILE="$INSTALL_DIR/.env"
ENV_EXAMPLE="$INSTALL_DIR/services/.env.example"
[[ -f "$ENV_EXAMPLE" ]] || fail ".env.example no encontrado en $INSTALL_DIR/services/"

# Map: variable name → .env.example key (suelen ser iguales)
declare -A CREDS=(
  [POSTGRES_PASSWORD]=POSTGRES_PASSWORD
  [APP_USER_PASSWORD]=APP_USER_PASSWORD
  [APP_ADMIN_PASSWORD]=APP_ADMIN_PASSWORD
  [MINIO_ROOT_PASSWORD]=MINIO_ROOT_PASSWORD
  [BACKUP_GPG_PASSPHRASE]=BACKUP_GPG_PASSPHRASE
)

if [[ ! -f "$ENV_FILE" ]]; then
  log ".env no existe — generando credenciales nuevas"
  cp "$ENV_EXAMPLE" "$ENV_FILE"
  for key in "${!CREDS[@]}"; do
    NEW=$(gen_uuid)
    env_set "$key" "$NEW" "$ENV_FILE"
  done
  ok ".env generado con UUIDs nuevos"
else
  log ".env existe — preservando credenciales"
  for key in "${!CREDS[@]}"; do
    EXISTING=$(env_get "$key" "$ENV_FILE")
    if [[ -z "$EXISTING" ]] || [[ "$EXISTING" == "CHANGE_ME" ]]; then
      NEW=$(gen_uuid)
      env_set "$key" "$NEW" "$ENV_FILE"
      log "  $key: regenerada (estaba vacía o CHANGE_ME)"
    else
      log "  $key: preservada"
    fi
  done
  ok ".env preservado + solo lo faltante regenerado"
fi
chmod 600 "$ENV_FILE"

# === STEP 5: Generate certs ===
log "5/8  Generando certs autofirmados..."
mkdir -p "$INSTALL_DIR/certs/postgres" "$INSTALL_DIR/certs/minio"
# Los compose files usan paths relativos tipo ../certs/minio que resuelven
# a /opt/services/services/certs/minio. Symlink para que apunte a los certs
# reales en /opt/services/certs/. Si existe un directorio viejo (de deploys
# previos, posiblemente root-owned), lo borramos con sudo.
if [[ -d "$INSTALL_DIR/services/certs" && ! -L "$INSTALL_DIR/services/certs" ]]; then
  sudo_run rm -rf "$INSTALL_DIR/services/certs"
fi
ln -sfn ../certs "$INSTALL_DIR/services/certs"
if [[ ! -f "$INSTALL_DIR/certs/postgres/server.crt" ]]; then
  openssl req -x509 -newkey rsa:2048 -nodes -days 365 \
    -keyout "$INSTALL_DIR/certs/postgres/server.key" \
    -out "$INSTALL_DIR/certs/postgres/server.crt" \
    -subj "/CN=postgres" 2>/dev/null
  ok "Cert postgres generado"
else
  ok "Cert postgres preservado"
fi
if [[ ! -f "$INSTALL_DIR/certs/minio/public.crt" ]]; then
  openssl req -x509 -newkey rsa:2048 -nodes -days 365 \
    -keyout "$INSTALL_DIR/certs/minio/private.key" \
    -out "$INSTALL_DIR/certs/minio/public.crt" \
    -subj "/CN=minio" 2>/dev/null
  ok "Cert minio generado"
else
  ok "Cert minio preservado"
fi

# === STEP 6: Build + Up ===
log "6/8  Building + starting services (esto puede tardar 1-3 min)..."
cd "$INSTALL_DIR/services"
# Makefile usa --env-file .env (relativo al CWD). El .env real está en
# $INSTALL_DIR/.env (parent). Symlink para que make lo encuentre.
[[ -L .env ]] || ln -sf ../.env .env
make down 2>/dev/null || true
make build
make up
make wait-healthy
ok "5 servicios healthy"

# === STEP 7: Systemd timers ===
log "7/8  Configurando systemd timers..."
if [[ -d "$INSTALL_DIR/services/systemd" ]]; then
  sudo_run cp "$INSTALL_DIR/services/systemd/"*.service /etc/systemd/system/ 2>/dev/null || true
  sudo_run cp "$INSTALL_DIR/services/systemd/"*.timer /etc/systemd/system/ 2>/dev/null || true
  sudo_run systemctl daemon-reload
  sudo_run systemctl enable --now domain-services-backup.timer domain-services-healthcheck.timer 2>/dev/null || true
  ok "Timers systemd activos"
else
  warn "no se encontró $INSTALL_DIR/services/systemd/, saltando timers"
fi

# === STEP 8: Print credenciales ===
VPS_IP=$(hostname -I 2>/dev/null | awk '{print $1}')
[[ -z "$VPS_IP" ]] && VPS_IP="<IP-de-tu-VPS>"

ADMIN_EMAIL=$(env_get ADMIN_EMAIL "$ENV_FILE")
[[ -z "$ADMIN_EMAIL" || "$ADMIN_EMAIL" == "CHANGE_ME" ]] && ADMIN_EMAIL="(no configurada)"

cat <<EOF

══════════════════════════════════════════════════════════════════════
  ✓ Stack instalado en $INSTALL_DIR
══════════════════════════════════════════════════════════════════════

  CREDENCIALES — guardalas en lugar seguro (1Password, Bitwarden, etc.)
  Las mismas están en: $ENV_FILE (chmod 600)

  ──────────────────────────────────────────────────────────────────
  ADMIN_EMAIL:           $ADMIN_EMAIL
  ──────────────────────────────────────────────────────────────────

  POSTGRES_PASSWORD:     $(env_get POSTGRES_PASSWORD "$ENV_FILE")
  APP_USER_PASSWORD:     $(env_get APP_USER_PASSWORD "$ENV_FILE")
  APP_ADMIN_PASSWORD:    $(env_get APP_ADMIN_PASSWORD "$ENV_FILE")
  MINIO_ROOT_PASSWORD:   $(env_get MINIO_ROOT_PASSWORD "$ENV_FILE")
  BACKUP_GPG_PASSPHRASE: $(env_get BACKUP_GPG_PASSPHRASE "$ENV_FILE")
  ──────────────────────────────────────────────────────────────────

  URLS (HTTP plano por IP):
    Dashboard:  http://$VPS_IP/
    API:        http://$VPS_IP/api/v1/...
    MCP HTTP:   http://$VPS_IP/mcp
    Health:     http://$VPS_IP/healthz

  ──────────────────────────────────────────────────────────────────
  PRÓXIMOS PASOS:
    - Verificá que el dashboard carga: curl http://$VPS_IP/
    - Para HTTPS con cert válido, armar una HU aparte (no incluido).
    - Si rotás credenciales manualmente en .env y re-corrés install,
      se preservan tus valores.

══════════════════════════════════════════════════════════════════════

EOF
ok "DONE"