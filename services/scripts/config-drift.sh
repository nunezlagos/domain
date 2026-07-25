#!/usr/bin/env bash
# services/scripts/config-drift.sh — detecta y corrige configs montadas stale (DOMAINSERV-143).
#
# EL PROBLEMA
# Docker monta archivos individuales por INODE. El deploy REEMPLAZA el archivo (no lo
# edita in-place), así que un container que ya estaba corriendo sigue leyendo el inode
# anterior. `docker compose up -d` no lo recrea: el servicio no cambió en el compose,
# solo el contenido de un archivo montado, que compose no mira. Resultado: deploy en
# verde con la config vieja cargada. Le pasó a la retención de Loki (DOMAINSERV-128):
# el archivo en disco era correcto y domain-loki llevaba 17h con la versión anterior.
#
# Medido 2026-07-25 con Docker 29.6.2, reemplazando el archivo con mv (inode nuevo):
#   up -d (sin force)   -> el container SIGUE con la vieja   <- la causa del bug
#   docker restart      -> toma la nueva (re-resuelve el bind al arrancar)
#   up -d --force-recreate -> toma la nueva
# O sea: el ticket afirmaba que `restart` tampoco alcanzaba, y en esta versión sí
# alcanza. Igual se usa --force-recreate, que es lo que compose GARANTIZA por
# semántica en vez de depender de un detalle de implementación del daemon.
#
# LA DETECCIÓN
# No hay tabla de configs hardcodeada a propósito: se deriva de `docker inspect`, así
# que un servicio o un mount nuevo queda cubierto sin tocar este archivo. Para cada
# bind mount se compara el hash del archivo EN DISCO contra el que el container tiene
# montado de verdad. Esa comparación sirve para las dos mitades del ticket: decidir qué
# recrear, y verificar que después de recrear la config nueva llegó.
#
# `docker cp` NO SIRVE para leer el lado del container, y es una trampa: sobre un path
# bind-mounteado el daemon resuelve la lectura por el mount SOURCE del host, así que
# devuelve el contenido NUEVO y es ciego exactamente al inode viejo que buscamos.
# Verificado 2026-07-25 con Docker 29.6.2 sobre un container con el archivo ya
# reemplazado: disco y `docker cp` daban "nueva", `docker exec cat` daba "original".
# Un detector construido sobre docker cp reporta "todo en orden" con la config vieja
# cargada — el mismo silencio del bug original.
#
# Se lee desde el mount namespace del container, con tres intentos en orden de
# portabilidad: `md5sum` adentro, `cat` adentro, y /proc/<pid>/root (no necesita NINGÚN
# binario en la imagen, para casos distroless; requiere root, que install.sh ya tiene).
#
# TRES ESTADOS, no dos: ok / stale / no-verificable. Distinguir el tercero es lo que
# evita que una lectura que falla se reporte como config vieja y pinte de rojo un
# deploy sano.
#
# Funciones con seams testeables (todo pasa por `docker`) — ver config-drift_test.sh.

# solo containers del stack. Los rds-* y demás del host son de OTROS proyectos: este
# script no los gestiona y no los toca (mismo criterio que DOMAINSERV-121).
DOMAIN_CONTAINER_PREFIX="${DOMAIN_CONTAINER_PREFIX:-domain-}"

# CWD para `docker compose`: en /opt/services/services el .env es un SYMLINK a ../.env
# y compose no lo toma solo (falla con "required variable GRAFANA_ADMIN_PASSWORD is
# missing a value"). Hay que correr desde acá y pasar --env-file explícito.
SERVICES_DIR="${SERVICES_DIR:-$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)}"

# contadores del último scan (los lee install.sh para el reporte final)
DRIFT_RECREATED=0
DRIFT_UNRESOLVED=0
DRIFT_UNVERIFIED=0

# log: si install.sh nos sourcea, reusa el suyo; si corremos solos, definimos uno igual
if ! declare -f log >/dev/null 2>&1; then
  log()  { printf '\033[36m[install]\033[0m %s\n' "$*" >&2; }
  ok()   { log "✓ $*"; }
  warn() { log "! $*"; }
fi

# containers del stack corriendo ahora
stack_containers() {
  docker ps --filter "name=^${DOMAIN_CONTAINER_PREFIX}" --format '{{.Names}}' 2>/dev/null || true
}

# bind mounts del container, una línea "SOURCE|DESTINATION" por mount
container_bind_mounts() {
  docker inspect --format \
    '{{range .Mounts}}{{if eq .Type "bind"}}{{.Source}}|{{.Destination}}{{"\n"}}{{end}}{{end}}' \
    "$1" 2>/dev/null || true
}

container_label() {
  docker inspect --format "{{index .Config.Labels \"$2\"}}" "$1" 2>/dev/null || true
}

host_hash() {
  [[ -f "$1" ]] || return 1
  md5sum "$1" 2>/dev/null | awk '{print $1}'
}

# hash del archivo TAL CUAL lo ve el proceso del container. rc=2 si no se pudo leer
# (no-verificable) — NUNCA se devuelve un hash inventado, porque un hash falso acá se
# traduce en un recreate innecesario o, peor, en un stale no detectado.
container_hash() {
  local container="$1" path="$2" hash pid

  # 1) md5sum adentro: no necesita shell, solo el binario (busybox alcanza)
  hash=$(docker exec "$container" md5sum "$path" 2>/dev/null | awk '{print $1}')
  if [[ "$hash" =~ ^[0-9a-f]{32}$ ]]; then
    printf '%s\n' "$hash"; return 0
  fi

  # 2) cat adentro + md5sum del lado del host: para imágenes sin md5sum.
  # Va por archivo temporal y NO por $(...): la sustitución de comando corta los
  # newlines finales, así que el hash no daría igual al del archivo en disco y todo
  # config que termine en \n (o sea, todos) se reportaría stale sin serlo.
  local tmp
  tmp=$(mktemp 2>/dev/null) || tmp=""
  if [[ -n "$tmp" ]]; then
    if docker exec "$container" cat "$path" >"$tmp" 2>/dev/null; then
      hash=$(md5sum "$tmp" 2>/dev/null | awk '{print $1}')
      rm -f "$tmp"
      [[ -n "$hash" ]] && { printf '%s\n' "$hash"; return 0; }
    else
      rm -f "$tmp"
    fi
  fi

  # 3) sin ningún binario adentro (distroless): el filesystem del container visto desde
  # el host. Resuelve por el mount namespace del proceso, así que ve el inode viejo
  # igual que él. Requiere root.
  pid=$(docker inspect --format '{{.State.Pid}}' "$container" 2>/dev/null)
  if [[ -n "$pid" && "$pid" != "0" && -r "/proc/$pid/root$path" ]]; then
    hash=$(md5sum "/proc/$pid/root$path" 2>/dev/null | awk '{print $1}')
    [[ -n "$hash" ]] && { printf '%s\n' "$hash"; return 0; }
  fi

  return 2
}

# Lista los archivos (host_path|container_path) a comparar de un mount. Un mount de
# archivo es uno solo; uno de directorio (grafana-provisioning, postgres/init) se
# expande a cada archivo de adentro — ahí también hay configs que pueden quedar stale.
mount_file_pairs() {
  local src="$1" dest="$2" f rel
  if [[ -f "$src" ]]; then
    printf '%s|%s\n' "$src" "$dest"
  elif [[ -d "$src" ]]; then
    while IFS= read -r f; do
      rel="${f#"$src"/}"
      printf '%s|%s\n' "$f" "$dest/$rel"
    done < <(find "$src" -type f 2>/dev/null | sort)
  fi
  # sockets, devices y fuentes inexistentes se ignoran: /var/run/docker.sock no es
  # una config y no tiene contenido comparable
}

# Compara todas las configs montadas de un container.
# stdout: una línea "STATE|host_path|container_path" por archivo con novedad
#         (STATE = stale | unverified). Los que coinciden no se reportan.
container_config_state() {
  local container="$1" line src dest pair hpath cpath hhash chash rc
  while IFS= read -r line; do
    [[ -z "$line" ]] && continue
    src="${line%%|*}"; dest="${line#*|}"
    while IFS= read -r pair; do
      [[ -z "$pair" ]] && continue
      hpath="${pair%%|*}"; cpath="${pair#*|}"
      hhash=$(host_hash "$hpath") || continue
      chash=$(container_hash "$container" "$cpath"); rc=$?
      if (( rc != 0 )); then
        printf 'unverified|%s|%s\n' "$hpath" "$cpath"
      elif [[ "$hhash" != "$chash" ]]; then
        printf 'stale|%s|%s\n' "$hpath" "$cpath"
      fi
    done < <(mount_file_pairs "$src" "$dest")
  done < <(container_bind_mounts "$container")
}

# Recrea el container por su compose de origen (labels), sin tocar sus dependencias:
# --no-deps es lo que cumple "no se reinicia un container cuya config no cambió".
compose_recreate() {
  local container="$1" config_files service f
  config_files=$(container_label "$container" com.docker.compose.project.config_files)
  service=$(container_label "$container" com.docker.compose.service)
  [[ -n "$config_files" && -n "$service" ]] || return 2

  local -a args=() files=()
  IFS=',' read -r -a files <<< "$config_files"
  for f in "${files[@]}"; do
    [[ -n "$f" ]] && args+=(-f "$f")
  done
  ( cd "$SERVICES_DIR" && docker compose "${args[@]}" --env-file .env \
      up -d --force-recreate --no-deps "$service" ) >/dev/null 2>&1
}

# Barrida completa: detecta, recrea lo que quedó stale, y RE-VERIFICA que la config
# nueva efectivamente llegó. Deja el resultado en DRIFT_RECREATED / DRIFT_UNRESOLVED /
# DRIFT_UNVERIFIED. Siempre retorna 0: el reporte lo decide el caller.
sync_config_drift() {
  DRIFT_RECREATED=0; DRIFT_UNRESOLVED=0; DRIFT_UNVERIFIED=0
  local container state stale_count line

  for container in $(stack_containers); do
    stale_count=0
    while IFS= read -r line; do
      [[ -z "$line" ]] && continue
      state="${line%%|*}"
      if [[ "$state" == "stale" ]]; then
        stale_count=$((stale_count + 1))
        warn "$container tiene montada una versión VIEJA de ${line##*|} (el archivo en disco ya cambió)"
      else
        DRIFT_UNVERIFIED=$((DRIFT_UNVERIFIED + 1))
        warn "$container: no se pudo leer ${line##*|} para verificarla (docker cp falló)"
      fi
    done < <(container_config_state "$container")

    (( stale_count == 0 )) && continue

    log "  recreando $container para que tome las $stale_count config(s) nuevas..."
    if ! compose_recreate "$container"; then
      DRIFT_UNRESOLVED=$((DRIFT_UNRESOLVED + stale_count))
      warn "  no se pudo recrear $container — su config sigue siendo la vieja"
      continue
    fi

    # re-verificación: recrear sin confirmar es volver al modo de falla silencioso
    local remaining=0
    while IFS= read -r line; do
      [[ "${line%%|*}" == "stale" ]] && remaining=$((remaining + 1))
    done < <(container_config_state "$container")

    if (( remaining > 0 )); then
      DRIFT_UNRESOLVED=$((DRIFT_UNRESOLVED + remaining))
      warn "  $container SIGUE con config vieja después de recrearlo — revisar a mano"
    else
      DRIFT_RECREATED=$((DRIFT_RECREATED + 1))
      ok "  $container recreado y verificado con la config nueva"
    fi
  done

  return 0
}
