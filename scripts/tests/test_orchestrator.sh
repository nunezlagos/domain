#!/usr/bin/env bash
# scripts/tests/test_orchestrator.sh
#
# TDD green/red para HU 38.12 — valida el orquestador deploy.sh.
# Cubre: log_phase, validate_env_no_laxo, should_rollback y un smoke
# del flujo completo en --dry-run contra un repo fake para no tocar
# Docker.
#
# Uso: ./scripts/tests/test_orchestrator.sh

set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
REPO_ROOT="$(cd "$SCRIPT_DIR/../.." && pwd)"

ORCH="$REPO_ROOT/scripts/lib/orchestrator.sh"
DEPLOY="$REPO_ROOT/scripts/deploy.sh"

for f in "$ORCH" "$DEPLOY"; do
  if [[ ! -f "$f" ]]; then
    echo "RED: $f no existe aun — esperado para green phase" >&2
    exit 1
  fi
done

WORK="$(mktemp -d)"
trap 'rm -rf "$WORK"' EXIT

failed=0

assert_eq() {
  local label="$1" expected="$2" actual="$3"
  if [[ "$expected" == "$actual" ]]; then
    echo "PASS: $label"
  else
    echo "FAIL: $label — expected='$expected' actual='$actual'"
    failed=$((failed + 1))
  fi
}

assert_contains() {
  local label="$1" needle="$2" haystack="$3"
  if [[ "$haystack" == *"$needle"* ]]; then
    echo "PASS: $label"
  else
    echo "FAIL: $label — '$needle' no esta en '$haystack'"
    failed=$((failed + 1))
  fi
}

assert_rc() {
  local label="$1" expected="$2" actual="$3"
  if (( expected == actual )); then
    echo "PASS: $label"
  else
    echo "FAIL: $label — expected rc=$expected got rc=$actual"
    failed=$((failed + 1))
  fi
}

# shellcheck source=/dev/null
source "$ORCH"

# --- validate_env_no_laxo ---

mkdir -p "$WORK/c1/services"
rc=0
DEPLOY_ENV_FILE="$WORK/c1/services/.env" validate_env_no_laxo || rc=$?
assert_rc "validate: .env inexistente -> rc 1" 1 "$rc"

# El check medía con `[[ ! -w ]]`, y eso NO expresa lo que quiere. `-w` responde "¿puede
# escribirlo QUIEN PREGUNTA?", que bajo root es siempre sí —root ignora los bits— y bajo
# el dueño de un 600 también. Resultado medido en producción: 61 ciclos del auto-deploy,
# 61 abortos, cero deploys, porque el servicio corre como root.
#
# Y el criterio viejo se contradecía con el resto del change: el MUST-4 exige que el .env
# conserve 600, o sea escribible por su dueño, mientras el validate exigía que nadie
# pudiera escribirlo. Los dos no pueden ser ciertos a la vez.
#
# El criterio nuevo mira los BITS de group y other, que dan la misma respuesta bajo
# cualquier euid, root incluido. El caso decisivo es el 600: reproduce exactamente la
# condición que rompía en producción sin necesidad de ser root, porque el dueño del
# archivo de prueba sí puede escribirlo.
env_de_prueba() {
  local dir="$WORK/$1/services" modo="$2"
  mkdir -p "$dir"
  printf 'DOMAIN_FIELD_ENC_KEY=abc\n' > "$dir/.env"
  chmod "$modo" "$dir/.env"
  echo "$dir/.env"
}

for caso in "600:0:el modo de produccion, escribible por su dueño" \
            "640:0:group solo lee" \
            "644:0:group y other solo leen" \
            "444:0:nadie escribe" \
            "660:1:group escribe" \
            "666:1:cualquiera escribe" \
            "602:1:other escribe aunque group no" \
            "607:1:other tiene rwx"; do
  modo="${caso%%:*}"; resto="${caso#*:}"
  esperado="${resto%%:*}"; glosa="${resto#*:}"
  rc=0
  DEPLOY_ENV_FILE="$(env_de_prueba "c2-$modo" "$modo")" validate_env_no_laxo || rc=$?
  assert_rc "validate: .env $modo ($glosa) -> rc $esperado" "$esperado" "$rc"
done

# el otro motivo de aborto sigue vivo y no se lo tapa el cambio de criterio
ruta_sin_key="$(env_de_prueba c3 600)"
printf 'OTRA=cosa\n' > "$ruta_sin_key"
chmod 600 "$ruta_sin_key"
rc=0
DEPLOY_ENV_FILE="$ruta_sin_key" validate_env_no_laxo || rc=$?
assert_rc "validate: sin DOMAIN_FIELD_ENC_KEY -> rc 1" 1 "$rc"

# --- log_phase ---

mkdir -p "$WORK/c4"
LOG_FILE="$WORK/c4/deploy.log" log_phase "test:phase" >/dev/null 2>&1
content4="$(cat "$WORK/c4/deploy.log" 2>/dev/null || true)"
assert_contains "log_phase: contiene 'test:phase'" "test:phase" "$content4"
assert_contains "log_phase: formato ISO datetime" "T" "$content4"

# El log en disco es ACCESORIO y no puede tumbar el deploy. Medido en producción: el
# auto-deploy corre como root y creó /opt/services/.deploy.log con dueño root:root 644,
# así que el redeploy MANUAL de sysadmin moría con "tee: Permission denied" y rc=1 en su
# primera línea de log, sin llegar a ninguna fase. El camino de emergencia caído por no
# poder escribir un archivo de texto.
mkdir -p "$WORK/c5"
printf 'previo\n' > "$WORK/c5/deploy.log"
chmod 444 "$WORK/c5/deploy.log"
rc=0
err5="$(LOG_FILE="$WORK/c5/deploy.log" log_phase "test:degradado" 2>&1 >/dev/null)" || rc=$?
assert_rc "log_phase: log no escribible -> NO aborta" 0 "$rc"
assert_contains "log_phase: sin log en disco, el motivo igual sale por stderr" \
  "test:degradado" "$err5"

# --- should_rollback ---

NOOP=1
rc=0
should_rollback || rc=$?
assert_rc "should_rollback: NOOP=1 -> rc 1 (skip)" 1 "$rc"

unset NOOP
rc=0
should_rollback || rc=$?
assert_rc "should_rollback: NOOP unset -> rc 0 (rollback)" 0 "$rc"

# --- Smoke: deploy.sh --dry-run contra repo fake ---

WORK7="$WORK/c7"
mkdir -p "$WORK7/repo/services"
(
  cd "$WORK7/repo"
  git init -q -b main
  git config user.email o@o
  git config user.name o
  echo init > README.md
  printf 'DOMAIN_FIELD_ENC_KEY=test\n' > services/.env
  chmod 444 services/.env
  git add README.md services/.env
  git commit -q -m first
  echo v2 > services/Makefile
  git add services/Makefile
  git commit -q -m second
)

LOG_FILE="$WORK7/deploy.log" \
  DEPLOY_REPO_ROOT="$WORK7/repo" \
  PREV_SHA=HEAD~1 \
  "$DEPLOY" --dry-run > /dev/null 2> "$WORK7/stderr.txt"
rc7=$?
assert_rc "smoke: deploy --dry-run termina rc 0" 0 "$rc7"

content7="$(cat "$WORK7/deploy.log" 2>/dev/null || true)"
for phase in "fetch" "detect" "build" "restart" "verify"; do
  assert_contains "smoke: log contiene '$phase'" "$phase" "$content7"
done

if (( failed > 0 )); then
  echo "RED — $failed tests fallaron"
  exit 1
fi

echo "GREEN — todos los tests pasaron"
exit 0
