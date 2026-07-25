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
#
# TODO SECRETO VA ACA. Lo que no esté en esta lista se despliega con el valor
# literal del .env.example, y eso ya pasó en producción: GRAFANA_ADMIN_PASSWORD y
# DOMAIN_FIELD_ENC_KEY faltaban, así que el VPS quedó con "CHANGE_ME" — un valor
# publicado en un repo público, junto con la IP del servidor. Grafana estuvo
# alcanzable en 0.0.0.0:3000 con esa password (incidente del 2026-07-24).
declare -A CREDS=(
  [POSTGRES_PASSWORD]=POSTGRES_PASSWORD
  [APP_USER_PASSWORD]=APP_USER_PASSWORD
  [APP_ADMIN_PASSWORD]=APP_ADMIN_PASSWORD
  [MINIO_ROOT_PASSWORD]=MINIO_ROOT_PASSWORD
  [BACKUP_GPG_PASSPHRASE]=BACKUP_GPG_PASSPHRASE
  [DOMAIN_FLOW_TOKEN_SECRET]=DOMAIN_FLOW_TOKEN_SECRET
  [GRAFANA_ADMIN_PASSWORD]=GRAFANA_ADMIN_PASSWORD
  # cifra auth_api_keys.key_ciphertext. Se regenera SOLO si está vacía o en
  # CHANGE_ME, nunca si ya tiene valor real: rotarla dejaría ilegible todo
  # ciphertext previo y esas API keys habría que rotarlas a mano
  [DOMAIN_FIELD_ENC_KEY]=DOMAIN_FIELD_ENC_KEY
  [CROWDSEC_BOUNCER_API_KEY]=CROWDSEC_BOUNCER_API_KEY
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
    # comparar contra el .env.example es la deteccion que de verdad sirve:
    # buscar solo la cadena "CHANGE_ME" deja pasar cualquier otro placeholder
    # que el example use ahora o en el futuro
    PLACEHOLDER=$(env_get "$key" "$ENV_EXAMPLE")
    if [[ -z "$EXISTING" ]] || [[ "$EXISTING" == "CHANGE_ME" ]] \
       || { [[ -n "$PLACEHOLDER" ]] && [[ "$EXISTING" == "$PLACEHOLDER" ]]; }; then
      NEW=$(gen_uuid)
      env_set "$key" "$NEW" "$ENV_FILE"
      log "  $key: regenerada (estaba vacía o con el placeholder del .env.example)"
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
# DOMAINSERV-124: pre-flight del borde HTTP, ANTES de bajar nada.
#
# Caddy dejo de usar la imagen oficial: lleva el bouncer de CrowdSec compilado con
# xcaddy. Eso agrega un modo de falla nuevo — si el Caddyfile no adapta contra ESE
# binario, Caddy no arranca y se cae el unico borde HTTP del host, que tambien
# sirve quiensabe.cl.
#
# Por eso se valida ACA y no despues del `make down`: si algo esta mal, el deploy
# aborta con el stack anterior INTACTO y sirviendo. Validar despues del down
# significaria diagnosticar con el sitio ya caido.
#
# El self-check HTTP de mas abajo sigue siendo la red final; esto es la que evita
# llegar hasta ahi.
log "Pre-flight del borde HTTP (imagen con plugins + validacion del Caddyfile)..."
# el CWD ya es $INSTALL_DIR/services y .env es el symlink a ../.env (ver arriba)
if ! docker compose -f caddy/docker-compose.yml --env-file .env build; then
  fail "no se pudo buildear la imagen de Caddy con plugins — abortando con el stack anterior intacto"
fi
if ! docker run --rm \
      -v "$INSTALL_DIR/services/caddy/Caddyfile:/etc/caddy/Caddyfile:ro" \
      -e CROWDSEC_BOUNCER_API_KEY="$(env_get CROWDSEC_BOUNCER_API_KEY "$ENV_FILE")" \
      domain-caddy:plugins caddy validate --config /etc/caddy/Caddyfile >/dev/null 2>&1; then
  fail "el Caddyfile NO valida contra la imagen con plugins — abortando con el stack anterior intacto.
  Reproducilo con:  docker run --rm -v $INSTALL_DIR/services/caddy/Caddyfile:/etc/caddy/Caddyfile:ro -e CROWDSEC_BOUNCER_API_KEY=x domain-caddy:plugins caddy validate --config /etc/caddy/Caddyfile"
fi
ok "pre-flight OK: imagen con plugins construida y Caddyfile valido"

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
# arrancar, reintentando hasta 90s: `ollama list` reporta el modelo apenas está en
# disco, pero el primer embed todavía tiene que cargarlo a memoria (~18s para
# bge-m3 en CPU). Esperar solo el pull dejaba la búsqueda semántica apagada.
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
for _ in $(seq 1 150); do
  code=$(curl -fsS -o /dev/null -w '%{http_code}' -m 5 http://localhost/healthz 2>/dev/null || true)
  if [[ "$code" == "200" ]]; then SELFCHECK_OK=1; break; fi
  sleep 2
done
[[ -n "$SELFCHECK_OK" ]] || fail "self-check: http://localhost/healthz no respondio 200 tras 300s — el stack no esta sirviendo. Revisá 'docker ps' y 'docker logs domain-caddy domain-mcp'."
ok "Self-check HTTP OK (/healthz 200)"

# === Postura de seguridad fail-loud (DOMAINSERV-120) ===
# El instalador no solo levanta servicios: valida que el stack no quede expuesto.
# Mismo criterio que el self-check HTTP de arriba — si la postura es insegura, el
# deploy NO reporta exito.
log "6b/9  Endureciendo red y revisando puertos expuestos..."

# ufw cubre los puertos del HOST (SSH y servicios no-docker). NO cubre los
# puertos publicados por Docker: Docker inserta sus reglas en la tabla nat y en
# la cadena DOCKER-USER, que se evaluan ANTES de las cadenas INPUT/FORWARD que
# usa ufw. Por eso la revision de puertos de mas abajo es la defensa real para
# los containers, y es fail-loud.
SSH_PORT="$(ss -tlnp 2>/dev/null | awk '/sshd/ {n=split($4,a,":"); print a[n]; exit}')"
SSH_PORT="${SSH_PORT:-22}"

if ! command -v ufw >/dev/null 2>&1; then
  log "  ufw no esta instalado, instalando..."
  sudo_run apt-get update -qq >/dev/null 2>&1 || true
  sudo_run apt-get install -y -qq ufw >/dev/null 2>&1 || warn "no se pudo instalar ufw"
fi

if command -v ufw >/dev/null 2>&1; then
  # el allow de SSH va PRIMERO: habilitar ufw sin esa regla deja el VPS
  # inaccesible y no hay forma de volver atras remotamente
  sudo_run ufw allow "$SSH_PORT"/tcp >/dev/null 2>&1 || fail "ufw: no se pudo permitir SSH en $SSH_PORT — abortando antes de habilitar el firewall"
  sudo_run ufw allow 80/tcp  >/dev/null 2>&1 || true
  sudo_run ufw allow 443/tcp >/dev/null 2>&1 || true

  # DOMAINSERV-130: los señuelos del honeypot TIENEN que quedar alcanzables — son
  # carnada. Y hay que abrirlos explicitamente porque el honeypot usa
  # network_mode:host, asi que NO pasa por las cadenas de Docker y ufw si lo
  # filtra. Verificado en el deploy del 2026-07-25: con ufw activo los 14 puertos
  # quedaron bloqueados y el honeypot no detectaba nada, mientras que las bases en
  # 6300-6303 (publicadas por Docker) seguian abiertas. El mismo hallazgo de que
  # ufw no cubre Docker, visto desde el otro lado.
  for hp in 23 445 1433 3389 6379 9200 11211 27017 2375 4444 31337 5555 1099 50050; do
    sudo_run ufw allow "$hp"/tcp >/dev/null 2>&1 || true
  done
  sudo_run ufw default deny incoming  >/dev/null 2>&1 || true
  sudo_run ufw default allow outgoing >/dev/null 2>&1 || true
  sudo_run ufw --force enable >/dev/null 2>&1 || warn "ufw: no se pudo habilitar"

  if sudo_run ufw status 2>/dev/null | grep -q "^Status: active"; then
    if sudo_run ufw status 2>/dev/null | grep -qE "(^| )${SSH_PORT}/tcp"; then
      ok "ufw activo (deny incoming; permitidos $SSH_PORT/tcp, 80/tcp, 443/tcp)"
    else
      fail "ufw quedo activo SIN regla para SSH en $SSH_PORT — riesgo de perder acceso. Revisar 'ufw status' YA."
    fi
  else
    warn "ufw no quedo activo — los puertos del host siguen sin filtrar"
  fi
else
  warn "ufw no disponible: los puertos del host quedan sin filtrar"
fi

# Revision de puertos publicados por Docker en 0.0.0.0. Allowlist: solo el borde
# HTTP/HTTPS de Caddy. Cualquier otra cosa (bases de datos, dashboards, APIs
# internas) alcanzable desde internet aborta el deploy.
# Se distingue por dueño: un container domain-* expuesto es culpa de este
# instalador y aborta el deploy. Un container de otro proyecto en el mismo host
# no lo podemos arreglar desde aca (su compose no viaja en este bundle), asi que
# se reporta fuerte en cada deploy pero no bloquea.
# SECURITY_ALLOWED_PUBLIC_PORTS (opcional, en .env) declara puertos expuestos A
# PROPOSITO — el RDS propio al que el dueño se conecta en remoto. La excepcion
# tiene que ser DECLARADA para no avisar: asi queda auditable y, sobre todo, un
# puerto NUEVO que aparezca sin declarar SI se reporta. Un warning recurrente que
# se ignora no protege de nada.
#   SECURITY_ALLOWED_PUBLIC_PORTS=6300,6301,6302,6303
#
# Estos puertos no quedan sin defensa: la estrategia elegida es lista negra en vez
# de lista blanca, porque una allowlist de IP no sirve con IP dinamica. El baneo
# por comportamiento vive en DOMAINSERV-124 y DOMAINSERV-130.
log "  revisando puertos publicados a internet..."
DECLARED_PORTS=",${SECURITY_ALLOWED_PUBLIC_PORTS:-},"
EXPOSED_OWN=""
EXPOSED_FOREIGN=""
EXPOSED_DECLARED=""
while read -r cname cports; do
  [[ -z "$cports" ]] && continue
  for hostport in $(printf '%s' "$cports" | grep -oE '0\.0\.0\.0:[0-9]+' | cut -d: -f2 | sort -u); do
    case "$hostport" in
      80|443) continue ;;
    esac
    if [[ "$DECLARED_PORTS" == *",${hostport},"* ]]; then
      EXPOSED_DECLARED="${EXPOSED_DECLARED}    - ${cname}: puerto ${hostport}"$'\n'
      continue
    fi
    entry="    - ${cname}: puerto ${hostport} alcanzable desde internet"$'\n'
    case "$cname" in
      domain-*) EXPOSED_OWN="${EXPOSED_OWN}${entry}" ;;
      *)        EXPOSED_FOREIGN="${EXPOSED_FOREIGN}${entry}" ;;
    esac
  done
done < <(docker ps --format '{{.Names}} {{.Ports}}' 2>/dev/null)

if [[ -n "$EXPOSED_DECLARED" ]]; then
  log "  expuestos DECLARADOS en SECURITY_ALLOWED_PUBLIC_PORTS (excepcion consciente):"
  printf '%s' "$EXPOSED_DECLARED" >&2
fi

fix_hint() {
  log "  ufw NO los bloquea: Docker publica via nat/DOCKER-USER y evita sus cadenas."
  log "  Fix: bindear a loopback en el compose del servicio, por ejemplo"
  log "      ports: [\"127.0.0.1:6300:5432\"]   en vez de   [\"6300:5432\"]"
  log "  y acceder por tunel SSH: ssh -L 6300:localhost:6300 <host>"
}

if [[ -n "$EXPOSED_FOREIGN" ]]; then
  log ""
  warn "Containers AJENOS a domain expuestos a internet (este instalador no los gestiona):"
  printf '%s' "$EXPOSED_FOREIGN" >&2
  fix_hint
  log ""
fi

if [[ -n "$EXPOSED_OWN" ]]; then
  log ""
  log "  Containers de domain publicados a internet fuera de la allowlist (80, 443):"
  printf '%s' "$EXPOSED_OWN" >&2
  fix_hint
  fail "postura insegura: domain expone puertos a internet — deploy abortado (DOMAINSERV-121)"
fi
ok "Puertos: domain solo publica 80/443 a internet"

# Segunda pasada, a nivel de KERNEL. El chequeo de arriba usa `docker ps`, que
# tiene un punto ciego verificado: para un container con network_mode:host el
# campo .Ports viene VACIO, asi que sus puertos no se ven. El honeypot usa
# justamente host networking (lo necesita para ver la IP real del atacante), y
# cualquier servicio del host fuera de Docker tampoco aparecia.
#
# `ss` no miente: enumera lo que realmente escucha, venga de donde venga. No
# requiere sudo para listar puertos (si para ver el proceso dueño, que no hace
# falta aca).
log "  revisando puertos a nivel de kernel (cubre host-networking y no-docker)..."
KERNEL_PORTS=""
while read -r kport; do
  [[ -z "$kport" ]] && continue
  case "$kport" in
    80|443|"$SSH_PORT") continue ;;
  esac
  [[ "$DECLARED_PORTS" == *",${kport},"* ]] && continue
  # los señuelos del honeypot son carnada deliberada, no un descuido
  case "$kport" in
    23|445|1433|3389|6379|9200|11211|27017|2375|4444|31337|5555|1099|50050) continue ;;
  esac
  KERNEL_PORTS="${KERNEL_PORTS} ${kport}"
done < <(ss -tlnH 2>/dev/null | awk '{print $4}' | grep -E '^(0\.0\.0\.0|\[::\]|\*):' | sed 's/.*://' | sort -un)

if [[ -n "$KERNEL_PORTS" ]]; then
  warn "puertos escuchando en todas las interfaces, sin declarar:${KERNEL_PORTS}"
  warn "  si son deliberados, declaralos en SECURITY_ALLOWED_PUBLIC_PORTS del .env"
else
  ok "Kernel: no hay puertos publicos sin declarar"
fi

# Backfill de embeddings (DOMAINSERV-80 H2). BEST-EFFORT a propósito: un fallo
# acá no debe tumbar un deploy que ya está sirviendo. Es idempotente porque el
# backfill solo toma filas con embedding NULL — en el cron diario, con todo
# poblado, son 0 filas y sale enseguida. --pause-ms=0 porque ollama es local y
# no tiene rate-limit que respetar.
if [[ "$EMB_PROVIDER" == "ollama" ]]; then
  # Verificación previa al backfill: si el guard de dimensión degradó a noop, el
  # backfill corre contra un embedder que no escribe nada y reporta éxito igual.
  # Así quedó el deploy del 2026-07-24: 0 de 2065 observaciones con embedding.
  if docker logs domain-mcp 2>&1 | grep -q "degradando a noop"; then
    warn "el embedder degradó a noop pese a DOMAIN_EMBEDDING_PROVIDER=ollama"
    warn "  la búsqueda semántica queda apagada (cae a full-text) y el backfill no escribiría nada"
    warn "  causa en el log: docker logs domain-mcp | grep 'degradando a noop'"
    warn "  tras corregirla: docker restart domain-mcp && docker exec domain-mcp domain embed-backfill --all --pause-ms=0"
  else
    ok "embedder activo (el probe de dimensión pasó)"
    log "Backfill de embeddings pendientes (idempotente, best-effort)..."
    if docker exec domain-mcp domain embed-backfill --all --pause-ms=0 2>&1 | tail -20; then
      ok "backfill completado"
    else
      warn "el backfill falló o quedó incompleto — el stack sigue operativo"
      warn "  reintentá con: docker exec domain-mcp domain embed-backfill --all --pause-ms=0"
    fi
    PENDIENTES=$(docker exec domain-postgres psql -U "$(env_get POSTGRES_USER "$ENV_FILE")" \
      -d "$(env_get POSTGRES_DB "$ENV_FILE")" -tAc \
      "SELECT count(*) FROM knowledge_observations WHERE embedding IS NULL" 2>/dev/null || echo "")
    if [[ -z "$PENDIENTES" ]]; then
      warn "no se pudo verificar cuántas observaciones quedaron sin embedding"
    elif [[ "$PENDIENTES" != "0" ]]; then
      warn "quedan $PENDIENTES observaciones sin embedding — la búsqueda semántica está incompleta"
      warn "  reintentá con: docker exec domain-mcp domain embed-backfill --all --pause-ms=0"
    else
      ok "sin observaciones pendientes de embedding"
    fi
  fi
fi

# Monitoring opt-in (DOMAINSERV-81): Prometheus + Grafana + Loki + Alloy. Se
# levanta solo si el .env define GRAFANA_ADMIN_PASSWORD — sin ella el compose cae
# al default admin/admin y quedaría un Grafana abierto en :3000. Antes el stack
# era enteramente manual, así que Loki y Alloy nunca llegaron al VPS y los paneles
# de logs apuntaban a un datasource inexistente.
if [[ -n "$(env_get GRAFANA_ADMIN_PASSWORD "$ENV_FILE")" ]]; then
  log "Monitoring: levantando Prometheus + Grafana + Loki + Alloy + CrowdSec..."
  if make monitoring-up; then
    ok "monitoring arriba (Grafana solo en loopback — ssh -L 3000:localhost:3000)"
  else
    warn "el stack de monitoring no levantó — el resto del deploy sigue operativo"
    warn "  reintentá con: cd $INSTALL_DIR/services && make monitoring-up"
  fi

  # DOMAINSERV-149: hasta acá el único container de monitoring que se verificaba era
  # CrowdSec (abajo). Los otros seis quedaban sin chequear, así que un Loki que no
  # levanta terminaba en verde: Alloy sin destino y logs perdidos en silencio. Loki
  # además se verifica por su /ready y no solo por presencia — en prod estuvo Up y
  # healthy mientras rechazaba ingesta.
  source "$INSTALL_DIR/services/scripts/monitoring-verify.sh"
  verify_monitoring_stack "$INSTALL_DIR/services/domain-mcp/deploy/monitoring/docker-compose.yml"

  # DOMAINSERV-124: el motor de deteccion es parte de la postura de seguridad, asi
  # que su ausencia se reporta. WARN y no FAIL porque el monitoring es opt-in: un
  # deploy sin el sigue sirviendo, solo queda sin deteccion.
  if docker ps --format '{{.Names}}' 2>/dev/null | grep -qx domain-crowdsec; then
    ok "CrowdSec corriendo"
    log "  ver que detecto:  docker exec domain-crowdsec cscli alerts list"

    # DOMAINSERV-124: el bouncer es la pieza que APLICA los baneos. Se verifica
    # explicito porque su modo de falla es SILENCIOSO: Caddy falla abierto a
    # proposito, asi que un bouncer no registrado (LAPI devolviendo 403) se ve
    # exactamente igual que todo funcionando — hasta que un atacante baneado entra.
    #
    # Se espera un poco: el registro por BOUNCER_KEY_caddy ocurre en el arranque
    # del container y puede no estar listo apenas responde el healthcheck.
    BOUNCER_OK=""
    for _ in $(seq 1 15); do
      if docker exec domain-crowdsec cscli bouncers list -o json 2>/dev/null | grep -q '"name"'; then
        BOUNCER_OK=1; break
      fi
      sleep 2
    done
    if [[ -n "$BOUNCER_OK" ]]; then
      ok "bouncer registrado — los baneos SE APLICAN en la capa HTTP (403)"
      log "  quien esta baneado:  docker exec domain-crowdsec cscli decisions list"
      log "  si te baneaste:      sudo $INSTALL_DIR/services/domain-mcp/deploy/monitoring/unban.sh allow-me"
      log "  el SSH nunca se filtra: este bouncer actua solo sobre HTTP"
    else
      warn "CrowdSec corre pero NO hay bouncer registrado: detecta y decide, pero NADIE aplica los baneos"
      warn "  Caddy falla abierto a proposito, asi que esto NO rompe el sitio — pero no hay enforcement"
      warn "  revisa que CROWDSEC_BOUNCER_API_KEY del .env coincida en ambos lados:"
      warn "    docker exec domain-crowdsec cscli bouncers list"
      warn "    docker logs domain-caddy 2>&1 | grep -i crowdsec"
    fi

    # DOMAINSERV-153: credencial de MAQUINA para el colector de geolocalizacion.
    # No alcanza la bouncer key: el endpoint de bouncer devuelve las decisiones
    # sin el campo `source`, o sea sin pais. Solo una machine ve /v1/alerts, que
    # es donde CrowdSec deja cn/latitude/longitude.
    #
    # Idempotente y con la misma regla que el resto de credenciales: la password
    # se genera UNA vez y se preserva. Si se regenerara en cada deploy, el
    # colector quedaria fallando en silencio contra una credencial vieja.
    CS_MACHINE_ID="domain-geo"
    CS_MACHINE_PASS=$(env_get DOMAIN_CROWDSEC_MACHINE_PASSWORD "$ENV_FILE")
    if [[ -z "$CS_MACHINE_PASS" ]]; then
      CS_MACHINE_PASS=$(gen_uuid)
      env_set DOMAIN_CROWDSEC_MACHINE_PASSWORD "$CS_MACHINE_PASS" "$ENV_FILE"
      env_set DOMAIN_CROWDSEC_MACHINE_ID "$CS_MACHINE_ID" "$ENV_FILE"
      log "  credencial de maquina para el colector de geo: generada"
    fi
    # --force reescribe la password si la machine ya existe, que es lo que hace
    # el registro convergente: tras un .env preservado, ambos lados coinciden.
    if docker exec domain-crowdsec cscli machines add "$CS_MACHINE_ID" \
         --password "$CS_MACHINE_PASS" --force >/dev/null 2>&1; then
      ok "machine '$CS_MACHINE_ID' registrada — el mapa de ataques por pais tiene de donde leer"
    else
      warn "no se pudo registrar la machine '$CS_MACHINE_ID' en CrowdSec"
      warn "  el mapa por pais quedara vacio; el resto del stack no se ve afectado"
      warn "  reintenta:  docker exec domain-crowdsec cscli machines add $CS_MACHINE_ID --password <pass del .env> --force"
    fi
  else
    warn "CrowdSec NO esta corriendo: no hay deteccion de ataques ni baneo automatico"
    warn "  los puertos declarados en SECURITY_ALLOWED_PUBLIC_PORTS quedan sin defensa activa"
  fi
else
  log "Monitoring omitido (definí GRAFANA_ADMIN_PASSWORD en el .env para habilitarlo)"
  warn "sin monitoring no hay CrowdSec: no hay deteccion de ataques ni baneo automatico"
fi

# DOMAINSERV-143: configs montadas stale. Docker monta por INODE y el deploy REEMPLAZA
# los archivos, así que un container que ya estaba corriendo sigue leyendo el inode
# viejo. `make monitoring-up` no lo recrea (compose no mira el contenido de un mount) y
# `docker restart` tampoco (reusa el mismo mount). El stack core no sufre esto —el
# make down + up de arriba lo recrea entero— pero el de monitoring NUNCA baja, y ahí
# fue donde la retención de Loki (DOMAINSERV-128) pasó 17h sin estar activa con el
# archivo correcto en disco.
#
# Va DESPUÉS de todos los `up` y cubre el stack completo: para el core es la
# verificación de que el efecto se produjo (DOMAINSERV-84), no un fix esperado.
log "Verificando que cada container tenga montada la config NUEVA..."
source "$INSTALL_DIR/services/scripts/config-drift.sh"
sync_config_drift
if (( DRIFT_RECREATED == 0 && DRIFT_UNRESOLVED == 0 && DRIFT_UNVERIFIED == 0 )); then
  ok "todas las configs montadas coinciden con el disco"
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

  GRAFANA — usuario: $(env_get GRAFANA_ADMIN_USER "$ENV_FILE")
  GRAFANA_ADMIN_PASSWORD: $(env_get GRAFANA_ADMIN_PASSWORD "$ENV_FILE")
  ──────────────────────────────────────────────────────────────────

  SECRETOS INTERNOS (no se usan a mano, pero perderlos tiene costo):
    DOMAIN_FIELD_ENC_KEY:     $(env_get DOMAIN_FIELD_ENC_KEY "$ENV_FILE")
      ↑ cifra las API keys. Sin ella, las keys emitidas quedan ilegibles.
    DOMAIN_FLOW_TOKEN_SECRET: $(env_get DOMAIN_FLOW_TOKEN_SECRET "$ENV_FILE")
    CROWDSEC_BOUNCER_API_KEY: $(env_get CROWDSEC_BOUNCER_API_KEY "$ENV_FILE")
  ──────────────────────────────────────────────────────────────────

  URLS (HTTP plano por IP):
    Dashboard:  http://$VPS_IP/
    API:        http://$VPS_IP/api/v1/...
    MCP HTTP:   http://$VPS_IP/mcp
    Health:     http://$VPS_IP/healthz
    Grafana:    NO se publica a internet. Túnel SSH:
                  ssh -L 3000:localhost:3000 $VPS_IP
                y después http://localhost:3000

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

# DOMAINSERV-143: una config que quedó vieja NO puede terminar en verde. El stack está
# arriba y sirviendo, así que no abortamos nada — pero el exit code queda en rojo para
# que el log del cron diario lo delate en vez de que se descubra 17h después.
if (( ${DRIFT_UNRESOLVED:-0} > 0 )); then
  warn "═══════════════════════════════════════════════════════════════"
  warn "DEPLOY CON $DRIFT_UNRESOLVED CONFIG(S) VIEJA(S) TODAVÍA MONTADA(S)"
  warn "El stack está arriba, pero al menos un container NO tomó su config nueva."
  warn "Recreálo a mano desde $INSTALL_DIR/services (el --env-file no es opcional):"
  warn "  docker compose --env-file .env -f <compose> up -d --force-recreate --no-deps <servicio>"
  warn "═══════════════════════════════════════════════════════════════"
  exit 1
fi
if (( ${DRIFT_UNVERIFIED:-0} > 0 )); then
  warn "$DRIFT_UNVERIFIED config(s) no se pudieron verificar (docker cp falló): no sabemos si son las nuevas"
fi

ok "DONE"