#!/usr/bin/env bash
# scripts/tests/test_orchestrator.sh
#
# TDD green/red para HU 38.12 — valida el orquestador deploy.sh.
# Cubre: log_phase, validate_env_readonly, should_rollback y un smoke
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

# --- validate_env_readonly ---

mkdir -p "$WORK/c1/services"
rc=0
DEPLOY_ENV_FILE="$WORK/c1/services/.env" validate_env_readonly || rc=$?
assert_rc "validate: .env inexistente -> rc 1" 1 "$rc"

mkdir -p "$WORK/c2/services"
printf 'DOMAIN_FIELD_ENC_KEY=abc\n' > "$WORK/c2/services/.env"
chmod 644 "$WORK/c2/services/.env"
rc=0
DEPLOY_ENV_FILE="$WORK/c2/services/.env" validate_env_readonly || rc=$?
assert_rc "validate: .env writable -> rc 1" 1 "$rc"

mkdir -p "$WORK/c3/services"
printf 'DOMAIN_FIELD_ENC_KEY=abc\n' > "$WORK/c3/services/.env"
chmod 444 "$WORK/c3/services/.env"
rc=0
DEPLOY_ENV_FILE="$WORK/c3/services/.env" validate_env_readonly || rc=$?
assert_rc "validate: .env readonly + KEY -> rc 0" 0 "$rc"

# --- log_phase ---

mkdir -p "$WORK/c4"
LOG_FILE="$WORK/c4/deploy.log" log_phase "test:phase" >/dev/null 2>&1
content4="$(cat "$WORK/c4/deploy.log" 2>/dev/null || true)"
assert_contains "log_phase: contiene 'test:phase'" "test:phase" "$content4"
assert_contains "log_phase: formato ISO datetime" "T" "$content4"

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
