#!/usr/bin/env bash
# services/scripts/pg-backup-guard.sh — guard del backup pre-migración (DOMAINSERV-37).
#
# El backup se decide por existencia del VOLUMEN de datos persistente, NO por el
# container corriendo: STEP 0 de install.sh borra el container antes del guard, así
# que chequear "docker ps" omitía el backup en todo redeploy (anulaba DOMAINSERV-26).
#
# Funciones puras (solo dependen de docker) para poder testearlas con un shim.

POSTGRES_VOLUME="${POSTGRES_VOLUME:-domain_postgres_data}"

# hay datos que respaldar si el volumen persistente existe
postgres_data_exists() {
  docker volume inspect "$POSTGRES_VOLUME" >/dev/null 2>&1
}

# el container está corriendo ahora
postgres_running() {
  [[ -n "$(docker ps -q -f "name=^domain-postgres$")" ]]
}

# decide si corre el backup pre-migración: hay datos persistentes que proteger,
# independiente de si el container está vivo (STEP 0 pudo haberlo borrado)
should_run_pre_migration_backup() {
  postgres_data_exists
}

# espera a que postgres acepte conexiones; timeout en segundos (default 60)
pg_wait_ready() {
  local timeout="${1:-60}" elapsed=0
  while ! docker exec domain-postgres pg_isready >/dev/null 2>&1; do
    (( elapsed >= timeout )) && return 1
    sleep 2
    elapsed=$(( elapsed + 2 ))
  done
  return 0
}
