#!/usr/bin/env bash
# Test del detector de configs stale (DOMAINSERV-143). Aísla todo con un shim de
# `docker` en PATH — sin Docker real. El shim sirve `docker cp` como un tar de verdad
# (así se ejercita el parseo real del stream) y registra los `docker compose` en un log
# para poder afirmar QUÉ se recreó y qué no.
#
# Regresión que motiva el ticket: domain-loki con el config viejo montado por inode
# mientras el archivo en disco ya era el nuevo → el deploy terminaba en verde.
set -uo pipefail

SCRIPT_DIR="$( cd "$( dirname "${BASH_SOURCE[0]}" )" && pwd )"
WORK="$(mktemp -d)"
trap 'rm -rf "$WORK"' EXIT

SHIM_DIR="$WORK/bin"; mkdir -p "$SHIM_DIR"
HOST_DIR="$WORK/host"; mkdir -p "$HOST_DIR"          # configs "en disco"
INNER_DIR="$WORK/inner"; mkdir -p "$INNER_DIR"        # configs "dentro del container"
COMPOSE_LOG="$WORK/compose.log"
: > "$COMPOSE_LOG"

# --- fixtures: dos containers, uno sano y uno stale ---
printf 'retention_stream: si\n' > "$HOST_DIR/loki-config.yml"
printf 'retention_stream: NO\n' > "$INNER_DIR/loki-config.yml"     # inode viejo → stale
printf 'scrape: ok\n'           > "$HOST_DIR/prometheus.yml"
printf 'scrape: ok\n'           > "$INNER_DIR/prometheus.yml"      # coincide → sin drift

# El shim de docker. Estado por env:
#   DOCKER_CONTAINERS       nombres que devuelve `docker ps`
#   DOCKER_NO_MD5SUM        1 = la imagen no trae md5sum (cae al fallback de `cat`)
#   DOCKER_NO_CAT           1 = la imagen tampoco trae cat (distroless)
#   DOCKER_EXEC_FAIL        si == nombre-container, ninguna lectura funciona
#   DOCKER_RECREATE_FAIL    si == nombre-container, `docker compose up` falla
#   DOCKER_FIX_ON_RECREATE  1 = recrear copia el archivo del host al "container"
cat > "$SHIM_DIR/docker" <<SHIM
#!/usr/bin/env bash
HOST_DIR="$HOST_DIR"
INNER_DIR="$INNER_DIR"
COMPOSE_LOG="$COMPOSE_LOG"
SHIM
cat >> "$SHIM_DIR/docker" <<'SHIM'
case "$1" in
  ps)
    printf '%s\n' ${DOCKER_CONTAINERS:-} ;;
  inspect)
    # $2 = --format, $3 = template, $4 = container
    tmpl="$3"; c="$4"
    case "$tmpl" in
      *Mounts*)
        # cada container monta un solo archivo, con el nombre derivado del container
        f="${c#domain-}"
        case "$c" in
          domain-loki)       printf '%s|%s\n' "$HOST_DIR/loki-config.yml" /etc/loki/loki-config.yml ;;
          domain-prometheus) printf '%s|%s\n' "$HOST_DIR/prometheus.yml" /etc/prometheus/prometheus.yml ;;
          domain-nolabels)   printf '%s|%s\n' "$HOST_DIR/loki-config.yml" /etc/loki/loki-config.yml ;;
          domain-secreto)    printf '%s|%s\n' "$HOST_DIR/ilegible.key" /etc/certs/ilegible.key ;;
          domain-sockonly)   printf '%s|%s\n' /var/run/docker.sock /var/run/docker.sock ;;
          *) : ;;
        esac ;;
      *config_files*)
        [[ "$c" == "domain-nolabels" ]] || printf '/opt/services/services/domain-mcp/deploy/monitoring/docker-compose.yml\n' ;;
      *compose.service*)
        [[ "$c" == "domain-nolabels" ]] || printf '%s\n' "${c#domain-}" ;;
      *State.Pid*)
        printf '0\n' ;;   # sin /proc real en el test: el 3er fallback no aplica
    esac ;;
  exec)
    # docker exec <container> md5sum|cat <path>  → lee el archivo "dentro del container"
    c="$2"; cmd="$3"; path="$4"
    [[ "$c" == "${DOCKER_EXEC_FAIL:-}" ]] && exit 1
    base="$(basename "$path")"
    [[ -f "$INNER_DIR/$base" ]] || exit 1
    case "$cmd" in
      md5sum) [[ "${DOCKER_NO_MD5SUM:-0}" == "1" ]] && exit 127
              md5sum "$INNER_DIR/$base" | awk -v p="$path" '{print $1"  "p}' ;;
      cat)    [[ "${DOCKER_NO_CAT:-0}" == "1" ]] && exit 127
              cat "$INNER_DIR/$base" ;;
      *) exit 127 ;;
    esac ;;
  compose)
    printf '%s\n' "$*" >> "$COMPOSE_LOG"
    svc="${!#}"
    [[ "domain-$svc" == "${DOCKER_RECREATE_FAIL:-}" ]] && exit 1
    if [[ "${DOCKER_FIX_ON_RECREATE:-0}" == "1" ]]; then
      cp "$HOST_DIR/$svc-config.yml" "$INNER_DIR/$svc-config.yml" 2>/dev/null \
        || cp "$HOST_DIR/$svc.yml" "$INNER_DIR/$svc.yml" 2>/dev/null || true
    fi ;;
esac
exit 0
SHIM
chmod +x "$SHIM_DIR/docker"
export PATH="$SHIM_DIR:$PATH"
export DOCKER_CONTAINERS DOCKER_EXEC_FAIL DOCKER_RECREATE_FAIL DOCKER_FIX_ON_RECREATE
export DOCKER_NO_MD5SUM DOCKER_NO_CAT
DOCKER_EXEC_FAIL=; DOCKER_NO_MD5SUM=0; DOCKER_NO_CAT=0

# SERVICES_DIR con un .env presente: compose_recreate hace cd ahí (en el VPS real ese
# .env es el symlink que obliga al --env-file)
export SERVICES_DIR="$WORK/services"; mkdir -p "$SERVICES_DIR"; : > "$SERVICES_DIR/.env"

source "$SCRIPT_DIR/config-drift.sh"

FAILS=0
check() { # descripción, esperado, actual
  if [[ "$2" == "$3" ]]; then
    printf 'PASS: %s\n' "$1"
  else
    printf 'FAIL: %s (esperado "%s", obtuve "%s")\n' "$1" "$2" "$3"; FAILS=$((FAILS + 1))
  fi
}

# --- host_hash / container_hash ---
h_host=$(host_hash "$HOST_DIR/prometheus.yml")
h_cont=$(container_hash domain-prometheus /etc/prometheus/prometheus.yml)
check "config igual dentro y fuera -> mismo hash" "$h_host" "$h_cont"

h_host=$(host_hash "$HOST_DIR/loki-config.yml")
h_cont=$(container_hash domain-loki /etc/loki/loki-config.yml)
[[ "$h_host" != "$h_cont" ]]
check "config stale -> hashes distintos" 0 $?

host_hash "$HOST_DIR/no-existe.yml" >/dev/null 2>&1
check "archivo inexistente en disco -> host_hash rc 1" 1 $?

# Falso positivo encontrado verificando el deploy en prod: domain-postgres reportaba
# stale en certs/postgres/server.key. Era falso — el archivo no cambiaba desde junio y
# el container tenía minutos. La key es 600 de otro owner, así que md5sum del lado del
# HOST fallaba por permisos, host_hash devolvía string VACÍO con rc 0, y la comparación
# "" != chash daba stale. Un archivo ilegible es NO-VERIFICABLE, nunca stale: si no,
# `make config-drift` sin sudo recrearía el container de la base de datos sin necesidad.
printf 'secreto\n' > "$HOST_DIR/ilegible.key"
chmod 000 "$HOST_DIR/ilegible.key"
if [[ -r "$HOST_DIR/ilegible.key" ]]; then
  # root lee todo: el escenario no es reproducible acá
  printf 'SKIP: archivo ilegible (corriendo como root, no aplica)\n'
else
  host_hash "$HOST_DIR/ilegible.key" >/dev/null 2>&1
  check "archivo ilegible en disco -> host_hash rc 2 (no-verificable)" 2 $?

  printf 'secreto\n' > "$INNER_DIR/ilegible.key"
  estado=$(container_config_state domain-secreto 2>/dev/null | cut -d'|' -f1)
  check "config ilegible -> unverified explícito, NUNCA stale ni silencio" "unverified" "$estado"

  # el caso del ticket: sin permisos, el sweep NO recrea nada. Recrear el container de la
  # base de datos porque no se pudo LEER un cert es peor que no verificarlo.
  DOCKER_CONTAINERS="domain-secreto"
  : > "$COMPOSE_LOG"
  sync_config_drift >/dev/null 2>&1
  check "  y el sweep no recrea por una config ilegible" "0 0" "$DRIFT_RECREATED $DRIFT_UNRESOLVED"
  check "  ningún docker compose invocado" 0 "$(wc -l < "$COMPOSE_LOG")"
  (( DRIFT_UNVERIFIED >= 1 )); check "  pero SÍ lo cuenta como no-verificable" 0 $?
fi

DOCKER_EXEC_FAIL=domain-loki
container_hash domain-loki /etc/loki/loki-config.yml >/dev/null 2>&1
check "no se puede leer adentro -> container_hash rc 2 (no-verificable, NO stale)" 2 $?
DOCKER_EXEC_FAIL=

# fallback 2: imagen sin md5sum. El hash tiene que seguir siendo IDÉNTICO al de md5sum
# adentro — si el fallback cortara el newline final, todo config daría stale sin serlo.
h_md5=$(container_hash domain-prometheus /etc/prometheus/prometheus.yml)
DOCKER_NO_MD5SUM=1
h_cat=$(container_hash domain-prometheus /etc/prometheus/prometheus.yml)
check "imagen sin md5sum -> fallback a cat, mismo hash (no corta el \\n final)" "$h_md5" "$h_cat"
check "  y sigue coincidiendo con el disco (sin falso stale)" \
  "$(host_hash "$HOST_DIR/prometheus.yml")" "$h_cat"

DOCKER_NO_CAT=1
container_hash domain-prometheus /etc/prometheus/prometheus.yml >/dev/null 2>&1
check "imagen sin md5sum ni cat y sin /proc -> no-verificable, no adivina" 2 $?
DOCKER_NO_MD5SUM=0; DOCKER_NO_CAT=0

# un config vacío adentro y vacío en disco NO es stale (el fallback por rc, no por
# hash-de-vacío, es lo que lo distingue de una lectura fallida)
: > "$INNER_DIR/vacio.yml"; : > "$HOST_DIR/vacio.yml"
DOCKER_NO_MD5SUM=1
check "config vacía en ambos lados -> mismo hash, no stale" \
  "$(host_hash "$HOST_DIR/vacio.yml")" "$(container_hash domain-loki /etc/vacio.yml)"
DOCKER_NO_MD5SUM=0

# --- mount_file_pairs: archivo, directorio, socket ---
pairs=$(mount_file_pairs "$HOST_DIR/loki-config.yml" /etc/loki/loki-config.yml)
check "mount de archivo -> 1 par" "$HOST_DIR/loki-config.yml|/etc/loki/loki-config.yml" "$pairs"

mkdir -p "$HOST_DIR/prov/dashboards"
printf 'a\n' > "$HOST_DIR/prov/dashboards/d.json"
printf 'b\n' > "$HOST_DIR/prov/datasources.yml"
n=$(mount_file_pairs "$HOST_DIR/prov" /etc/grafana/provisioning | wc -l)
check "mount de directorio -> se expande a cada archivo de adentro" 2 "$n"

n=$(mount_file_pairs /var/run/docker.sock /var/run/docker.sock | wc -l)
check "mount de socket -> se ignora (no es una config)" 0 "$n"

# --- container_config_state ---
state=$(container_config_state domain-prometheus)
check "container sano -> sin novedades" "" "$state"

state=$(container_config_state domain-loki)
check "container stale -> reporta stale" "stale|$HOST_DIR/loki-config.yml|/etc/loki/loki-config.yml" "$state"

DOCKER_EXEC_FAIL=domain-loki
state=$(container_config_state domain-loki | cut -d'|' -f1)
check "lectura imposible -> unverified, no stale" "unverified" "$state"
DOCKER_EXEC_FAIL=

# --- sync_config_drift: el caso del ticket ---
DOCKER_CONTAINERS="domain-loki domain-prometheus"
DOCKER_FIX_ON_RECREATE=1
: > "$COMPOSE_LOG"
sync_config_drift >/dev/null 2>&1
check "regresión DOMAINSERV-128: el stale se recrea" 1 "$DRIFT_RECREATED"
check "  y queda resuelto" 0 "$DRIFT_UNRESOLVED"
check "  sin falsos no-verificables" 0 "$DRIFT_UNVERIFIED"
check "  se recreó UN solo servicio (el sano no se toca)" 1 "$(wc -l < "$COMPOSE_LOG")"
grep -q 'loki' "$COMPOSE_LOG"; check "  el recreado es loki" 0 $?
grep -q 'prometheus' "$COMPOSE_LOG"; check "  prometheus NUNCA se reinicia" 1 $?
grep -q -- '--force-recreate' "$COMPOSE_LOG"; check "  usa --force-recreate (up -d sin force NO reaplica el mount)" 0 $?
grep -q -- '--no-deps' "$COMPOSE_LOG"; check "  usa --no-deps (no arrastra dependencias sanas)" 0 $?
grep -q -- '--env-file .env' "$COMPOSE_LOG"; check "  pasa --env-file (el .env de services/ es un symlink)" 0 $?

# stack ya sincronizado -> no se recrea nada
: > "$COMPOSE_LOG"
sync_config_drift >/dev/null 2>&1
check "segunda corrida idempotente: nada que recrear" 0 "$DRIFT_RECREATED"
check "  ningún docker compose invocado" 0 "$(wc -l < "$COMPOSE_LOG")"

# recreate que no arregla -> NO se reporta en verde
printf 'retention_stream: si v2\n' > "$HOST_DIR/loki-config.yml"
DOCKER_FIX_ON_RECREATE=0
sync_config_drift >/dev/null 2>&1
check "recreate que no aplica la config -> unresolved (no verde)" 1 "$DRIFT_UNRESOLVED"
check "  no se cuenta como recreado ok" 0 "$DRIFT_RECREATED"

# recreate que falla -> unresolved
DOCKER_RECREATE_FAIL=domain-loki
sync_config_drift >/dev/null 2>&1
check "compose up falla -> unresolved" 1 "$DRIFT_UNRESOLVED"
DOCKER_RECREATE_FAIL=

# container stale sin labels de compose -> unresolved con aviso, no crash
DOCKER_CONTAINERS="domain-nolabels"
sync_config_drift >/dev/null 2>&1
check "stale sin labels de compose -> unresolved (no se puede recrear solo)" 1 "$DRIFT_UNRESOLVED"

# container que solo monta el socket -> nada que comparar
DOCKER_CONTAINERS="domain-sockonly"
sync_config_drift >/dev/null 2>&1
check "container sin configs comparables -> todo en cero" "0 0 0" \
  "$DRIFT_RECREATED $DRIFT_UNRESOLVED $DRIFT_UNVERIFIED"

# install.sh corre con `set -euo pipefail` y nos sourcea: una lista `&&` que evalúa
# falso o un `(( ))` que da 0 no puede abortarle el deploy a mitad de camino.
DOCKER_CONTAINERS="domain-loki domain-prometheus"
DOCKER_FIX_ON_RECREATE=1
printf 'retention_stream: si v3\n' > "$HOST_DIR/loki-config.yml"
(
  set -euo pipefail
  source "$SCRIPT_DIR/config-drift.sh"
  sync_config_drift >/dev/null 2>&1
  [[ "$DRIFT_RECREATED" == "1" ]] || exit 3
  echo "vivo despues del scan"
) >/dev/null 2>&1
check "bajo set -euo pipefail no aborta (install.sh lo sourcea así)" 0 $?

if [[ "$FAILS" -gt 0 ]]; then
  printf '\n%d test(s) fallaron\n' "$FAILS"; exit 1
fi
printf '\nTodos los tests verdes\n'
