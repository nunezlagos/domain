#!/usr/bin/env bash
# Test del guard de backup pre-migración (DOMAINSERV-37). Aísla los predicados con
# un shim de `docker` en PATH — sin Docker real. Regresión: post STEP-0 el container
# no corre pero el volumen existe → el backup DEBE dispararse (antes se omitía).
set -uo pipefail

SCRIPT_DIR="$( cd "$( dirname "${BASH_SOURCE[0]}" )" && pwd )"
SHIM_DIR="$(mktemp -d)"
trap 'rm -rf "$SHIM_DIR"' EXIT

# shim de docker: comportamiento controlado por env DOCKER_VOLUME_PRESENT / DOCKER_PG_RUNNING
cat > "$SHIM_DIR/docker" <<'SHIM'
#!/usr/bin/env bash
case "$1 $2" in
  "volume inspect")
    [[ "${DOCKER_VOLUME_PRESENT:-0}" == "1" ]] && exit 0 || exit 1 ;;
  "ps -q"|"ps ")
    [[ "${DOCKER_PG_RUNNING:-0}" == "1" ]] && echo "abc123container"; exit 0 ;;
  "exec domain-postgres")
    # docker exec domain-postgres pg_isready
    [[ "${DOCKER_PG_READY:-0}" == "1" ]] && exit 0 || exit 1 ;;
esac
exit 0
SHIM
chmod +x "$SHIM_DIR/docker"
export PATH="$SHIM_DIR:$PATH"

source "$SCRIPT_DIR/pg-backup-guard.sh"

# exportadas para que el shim de docker (proceso hijo) las vea
export DOCKER_VOLUME_PRESENT DOCKER_PG_RUNNING DOCKER_PG_READY

FAILS=0
check() { # descripción, esperado(0/1), actual(0/1)
  if [[ "$2" == "$3" ]]; then
    printf 'PASS: %s\n' "$1"
  else
    printf 'FAIL: %s (esperado rc=%s, obtuve rc=%s)\n' "$1" "$2" "$3"; FAILS=$((FAILS + 1))
  fi
}

DOCKER_VOLUME_PRESENT=1; postgres_data_exists; check "volumen presente -> data_exists ok" 0 $?
DOCKER_VOLUME_PRESENT=0; postgres_data_exists; check "volumen ausente -> data_exists no" 1 $?

DOCKER_PG_RUNNING=1; postgres_running; check "container corriendo -> running ok" 0 $?
DOCKER_PG_RUNNING=0; postgres_running; check "container abajo -> running no" 1 $?

# regresión DOMAINSERV-37: estado post STEP-0 (container borrado) pero con datos
DOCKER_VOLUME_PRESENT=1; DOCKER_PG_RUNNING=0
should_run_pre_migration_backup
check "post-STEP0 con volumen -> backup SÍ corre (regresión DOMAINSERV-26)" 0 $?

# install fresh real: sin volumen -> no backup
DOCKER_VOLUME_PRESENT=0; DOCKER_PG_RUNNING=0
should_run_pre_migration_backup
check "install fresh sin volumen -> backup NO corre" 1 $?

# pg_wait_ready: postgres listo -> retorna 0 sin esperar
DOCKER_PG_READY=1; pg_wait_ready 4; check "pg_isready ok -> pg_wait_ready 0" 0 $?
# pg_wait_ready: nunca listo con timeout 0 -> retorna 1 de inmediato (sin colgar)
DOCKER_PG_READY=0; pg_wait_ready 0; check "pg_isready falla + timeout 0 -> pg_wait_ready 1" 1 $?

if [[ "$FAILS" -gt 0 ]]; then
  printf '\n%d test(s) fallaron\n' "$FAILS"; exit 1
fi
printf '\nTodos los tests verdes\n'
