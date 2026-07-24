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
REPO_BRANCH="${REPO_BRANCH:-main}"

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

# === STEP 0: Cleanup orphan containers ===
# Borra containers del install que pueden tener name conflicts. Lista
# explicita para NO tocar grafana/prometheus (otro deploy, fuera del install).
log "0/9  Limpiando contenedores del install (excluye grafana/prometheus)..."
CLEANED=0
for name in domain-postgres domain-minio domain-minio-bootstrap domain-mcp domain-admin domain-caddy domain-migrate domain-seed; do
  c=$(docker ps -a -q -f "name=^${name}$" 2>/dev/null || true)
  [[ -n "$c" ]] && docker rm -f "$c" >/dev/null 2>&1 && CLEANED=$((CLEANED + 1))
done
ok "Limpiados: $CLEANED"

# === STEP 1: Validate OS ===
log "1/9  Validating OS..."
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
log "2/9  Checking Docker..."
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
log "3/9  Setting up repo at $INSTALL_DIR..."
# El repo suele ser de otro usuario (sysadmin) mientras este script corre como
# root (sudo / cron de auto-update). Git rechaza operar sobre un repo con owner
# distinto ("detected dubious ownership") y aborta con set -e, DESPUÉS del
# make down → deja el stack caído. Marcamos el dir como seguro ANTES de tocar
# git. Idempotente: el grep evita duplicar la entrada en re-corridas del cron.
if [[ -d "$INSTALL_DIR" ]]; then
  git config --global --get-all safe.directory 2>/dev/null | grep -qxF "$INSTALL_DIR" \
    || git config --global --add safe.directory "$INSTALL_DIR" 2>/dev/null || true
fi
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

# Limpieza de leftovers: archivos untracked de layouts viejos. Un install
# pre-restructura dejaba copias FLAT de services/* en la raíz de $INSTALL_DIR
# (Makefile, domain-admin/, domain-mcp/, scripts/backup.sh, systemd/, ...).
# Esas copias hacían que `make -C /opt/services` y los units systemd apuntaran
# a CÓDIGO VIEJO.
#
# Usamos -ffd (doble -f): -fd solo no borra directorios con un .git anidado
# (ej: domain-mcp/ que tenía un repo embebido) y los dejaba como residual.
# -ff sí los remueve. SIN -x → respeta .gitignore, así que .env, certs/ y
# backups/ (ignorados) se PRESERVAN; solo borra los duplicados obsoletos.
if [[ -d "$INSTALL_DIR/.git" ]]; then
  STALE=$(cd "$INSTALL_DIR" && git clean -nffd 2>/dev/null | wc -l)
  if [[ "$STALE" -gt 0 ]]; then
    log "Limpiando $STALE leftovers untracked (layouts viejos, incl. repos anidados)..."
    (cd "$INSTALL_DIR" && git clean -ffd >/dev/null 2>&1) || true
  fi
  # Algunos leftovers flat tienen artefactos GITIGNOREADOS adentro (ej:
  # domain-mcp/.../.ai/, .mcp.json) que git clean no remueve sin -x — y -x
  # borraría .env/certs/backups. Los limpiamos de forma FUTURE-PROOF (sin
  # hardcodear nombres): una entrada en la raíz es duplicado del layout viejo
  # si su nombre también existe bajo services/ Y no está tracked en la raíz.
  # Así, cualquier servicio nuevo que sumes bajo services/ queda cubierto
  # automáticamente, sin tocar contenido tracked (README.md, scripts/...) ni
  # los ignorados legítimos (.env, certs/, backups/).
  for svc in "$INSTALL_DIR/services"/*; do
    name=$(basename "$svc")
    [[ "$name" == "certs" ]] && continue          # symlink, lo recrea STEP 5
    [[ -e "$INSTALL_DIR/$name" ]] || continue       # no hay dup en la raíz
    # Si algún archivo de ese path está tracked en la raíz → contenido legítimo.
    git -C "$INSTALL_DIR" ls-files --error-unmatch "$name" >/dev/null 2>&1 && continue
    rm -rf "${INSTALL_DIR:?}/$name"
  done
  ok "Working tree limpio (sin duplicados flat)"
fi

# === STEP 4: Generate or preserve .env ===
log "4/9  Configurando credenciales..."
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
  [DOMAIN_FLOW_TOKEN_SECRET]=DOMAIN_FLOW_TOKEN_SECRET
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
log "5/9  Generando certs autofirmados..."
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
log "6/9  Building + starting services (esto puede tardar 1-3 min)..."
cd "$INSTALL_DIR/services"
# Makefile usa --env-file .env (relativo al CWD). El .env real está en
# $INSTALL_DIR/.env (parent). Symlink para que make lo encuentre.
# Si por error alguien reemplazo el symlink con un archivo regular (ej.
# editandolo a mano), ln -sf lo sobreescribe silenciosamente → perdemos
# la sincronia con $INSTALL_DIR/.env y futuros updates no se propagan.
# Detectamos el caso, respaldamos el archivo y dejamos el symlink.
if [[ -e .env ]] && [[ ! -L .env ]]; then
  warn "services/.env era un archivo regular (no symlink) — respaldando y recreando como symlink a ../.env"
  mv .env ".env.broken-symlink.$(date +%Y%m%d-%H%M%S)"
fi
ln -sfn ../.env .env
# docker-compose.yml de cada servicio busca .env en su propio directorio.
# Sin esto, variables como APP_USER_PASSWORD quedan vacías → DB connection fail.
# Misma proteccion que arriba: si .env en el subdir es un archivo regular,
# lo respaldamos antes de recrear como symlink.
for _svc in "$INSTALL_DIR/services"/*/; do
  _name=$(basename "$_svc")
  [[ "$_name" == "certs" || "$_name" == "systemd" ]] && continue
  [[ -f "$_svc/docker-compose.yml" ]] || continue
  if [[ -e "$_svc/.env" ]] && [[ ! -L "$_svc/.env" ]]; then
    warn "$_svc/.env era un archivo regular (no symlink) — respaldando y recreando"
    mv "$_svc/.env" "$_svc/.env.broken-symlink.$(date +%Y%m%d-%H%M%S)"
  fi
  ln -sfn ../../.env "$_svc/.env"
done
# Backup pre-migración (DOMAINSERV-26/37): respaldamos la BD ANTES de make down +
# el init-container domain-migrate (migrate up). El guard decide por el VOLUMEN de
# datos, no por el container: STEP 0 ya borró domain-postgres, así que chequear
# `docker ps` omitía el backup en todo redeploy. backup.sh necesita el container
# corriendo → si STEP 0 lo bajó, lo levantamos temporal (make up lo baja el make
# down siguiente). Si el backup falla, ABORTAMOS: no migrar sin red de seguridad.
source "$INSTALL_DIR/services/scripts/pg-backup-guard.sh"
if should_run_pre_migration_backup; then
  log "Backup pre-migración (redeploy con datos)..."
  if ! postgres_running; then
    log "  postgres no corre (STEP 0 lo bajó) — levantando temporal para el dump..."
    make up SVC=postgres >/dev/null 2>&1 || fail "no se pudo levantar postgres temporal para el backup"
    # si el deploy aborta entre acá y el backup, no dejar el postgres temporal huérfano
    TEMP_PG_UP=1
    trap '[[ "${TEMP_PG_UP:-0}" == "1" ]] && docker rm -f domain-postgres >/dev/null 2>&1 || true' EXIT
    pg_wait_ready 60 || fail "postgres no quedó listo en 60s — abortando deploy sin backup"
  fi
  "$INSTALL_DIR/services/scripts/backup.sh" || fail "backup pre-migración falló — abortando deploy para no migrar sin respaldo"
  ok "backup pre-migración OK"
  # backup ok: el make down siguiente maneja el ciclo normal, desarmar el trap
  TEMP_PG_UP=0
  # marca para el guard de migrate (DOMAINSERV-39): ya hay pg_dump, migrar es seguro
  export DOMAIN_BACKUP_DONE=1
else
  log "Install fresh (sin volumen de datos) — se omite backup pre-migración"
fi
make down 2>/dev/null || true
# Re-check huerfanos despues del down (incluyendo running que no estan en el compose).
CLEANED=0
for name in domain-postgres domain-minio domain-minio-bootstrap domain-mcp domain-admin domain-caddy domain-migrate domain-seed; do
  c=$(docker ps -a -q -f "name=^${name}$" 2>/dev/null || true)
  [[ -n "$c" ]] && docker rm -f "$c" >/dev/null 2>&1 && CLEANED=$((CLEANED + 1))
done
[[ "$CLEANED" -gt 0 ]] && log "Limpiados $CLEANED post-down"
make build

# Embeddings opt-in (DOMAINSERV-80 H2): si el .env pide ollama, el servicio se
# levanta ANTES que domain-mcp. El server mide la dimensión real del modelo al
# arrancar; si ollama no responde todavía degrada a noop y la búsqueda semántica
# queda apagada hasta el próximo restart.
EMB_PROVIDER=$(env_get DOMAIN_EMBEDDING_PROVIDER "$ENV_FILE")
EMB_MODEL=$(env_get DOMAIN_OLLAMA_EMBED_MODEL "$ENV_FILE"); EMB_MODEL="${EMB_MODEL:-bge-m3}"
if [[ "$EMB_PROVIDER" == "ollama" ]]; then
  log "Embeddings: provider=ollama modelo=$EMB_MODEL — levantando servicio..."
  make up SVC=ollama
  # el bootstrap hace `ollama pull`; la primera vez baja ~1.2 GB. Esperamos a que
  # el modelo esté listo antes de arrancar el server, con tope de 15 min.
  OLLAMA_READY=""
  for _ in $(seq 1 180); do
    if docker exec domain-ollama ollama list 2>/dev/null | grep -q "$EMB_MODEL"; then
      OLLAMA_READY=1; break
    fi
    sleep 5
  done
  if [[ -n "$OLLAMA_READY" ]]; then
    ok "modelo $EMB_MODEL disponible en ollama"
  else
    warn "timeout (15m) esperando el modelo $EMB_MODEL — el server arrancará con búsqueda semántica apagada"
    warn "  revisá 'docker logs domain-ollama-bootstrap' y re-corré el instalador"
  fi
fi

make up
make wait-healthy
ok "servicios healthy"

# Self-check HTTP fail-loud (DOMAINSERV-84): wait-healthy solo verifica health de
# contenedores; esto confirma que el stack SIRVE de verdad a traves de Caddy.
log "6/9  Self-check HTTP (stack sirviendo via Caddy en :80)..."
SELFCHECK_OK=""
for _ in $(seq 1 30); do
  code=$(curl -fsS -o /dev/null -w '%{http_code}' -m 5 http://localhost/healthz 2>/dev/null || true)
  if [[ "$code" == "200" ]]; then SELFCHECK_OK=1; break; fi
  sleep 2
done
[[ -n "$SELFCHECK_OK" ]] || fail "self-check: http://localhost/healthz no respondio 200 tras 60s — el stack no esta sirviendo. Revisá 'docker ps' y 'docker logs domain-caddy domain-mcp'."
ok "Self-check HTTP OK (/healthz 200)"

# Backfill de embeddings (DOMAINSERV-80 H2). BEST-EFFORT a propósito: un fallo
# acá no debe tumbar un deploy que ya está sirviendo. Es idempotente porque el
# backfill solo toma filas con embedding NULL — en el cron diario, con todo
# poblado, son 0 filas y sale enseguida. --pause-ms=0 porque ollama es local y
# no tiene rate-limit que respetar.
if [[ "$EMB_PROVIDER" == "ollama" ]]; then
  log "Backfill de embeddings pendientes (idempotente, best-effort)..."
  if docker exec domain-mcp domain embed-backfill --all --pause-ms=0 2>&1 | tail -20; then
    ok "backfill completado"
  else
    warn "el backfill falló o quedó incompleto — el stack sigue operativo"
    warn "  reintentá con: docker exec domain-mcp domain embed-backfill --all --pause-ms=0"
  fi
fi

# === STEP 7: Systemd units + timers ===
log "7/9  Configurando systemd units + timers..."
if [[ -d "$INSTALL_DIR/services/systemd" ]]; then
  sudo_run cp "$INSTALL_DIR/services/systemd/"*.service /etc/systemd/system/ 2>/dev/null || true
  sudo_run cp "$INSTALL_DIR/services/systemd/"*.timer /etc/systemd/system/ 2>/dev/null || true
  sudo_run systemctl daemon-reload
  # Persistencia al boot: el stack ya está arriba (STEP 6 via make up); enable
  # sin --now solo lo registra para arrancar en el próximo boot desde la ruta
  # correcta ($INSTALL_DIR/services). Evita el doble make up redundante.
  sudo_run systemctl enable domain-services.service 2>/dev/null || true
  sudo_run systemctl enable --now domain-services-backup.timer domain-services-healthcheck.timer 2>/dev/null || true
  ok "Units + timers systemd activos"
else
  warn "no se encontró $INSTALL_DIR/services/systemd/, saltando systemd"
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

# === STEP 9: Daily-update cron ===
# El comando canonico `curl ... | sudo bash` corre este mismo script desde
# el cron de root una vez por dia. Es el MISMO entrypoint para install fresco
# y para update diario (idempotente). Si el cron ya esta, no duplica la linea.
log "9/9  Configurando cron de auto-update diario (03:00)..."

CRON_LINE="0 3 * * * /opt/services/scripts/daily-update.sh >> /var/log/domain-update.log 2>&1"
WRAPPER="/opt/services/scripts/daily-update.sh"

# Crea el wrapper si el repo no lo bajo aun (caso fresh clone antes del git pull).
if [[ ! -f "$WRAPPER" ]]; then
  cat > "$WRAPPER" <<'WRAPPER_EOF'
#!/usr/bin/env bash
exec /opt/services/services/install.sh
WRAPPER_EOF
  chmod +x "$WRAPPER"
  ok "Wrapper $WRAPPER creado"
fi

CURRENT_CRON=$(sudo_run crontab -u root -l 2>/dev/null || true)
if echo "$CURRENT_CRON" | grep -qF "$WRAPPER"; then
  ok "Cron ya estaba instalado (no duplico)"
else
  # Temp file en vez de pipe: bash no propaga el stdin del pipe a la funcion
  # sudo_run, entonces `printf | sudo_run crontab -u root -` leeria stdin vacio.
  TMP_CRON=$(mktemp)
  printf '%s\n%s\n' "$CURRENT_CRON" "$CRON_LINE" > "$TMP_CRON"
  sudo_run crontab -u root "$TMP_CRON"
  rm -f "$TMP_CRON"
  ok "Cron instalado: corre $WRAPPER todos los dias a las 03:00"
  ok "Log en /var/log/domain-update.log"
fi

ok "DONE"