#!/usr/bin/env bash
# scripts/tests/test_auto_deploy_entorno.sh
#
# issue-57.2 / DOMAINSERV-258 — el auto-deploy falla en CADA ciclo con exit 128:
#   fatal: detected dubious ownership in repository at '/opt/services'
#
# La causa MEDIDA en el VPS necesita DOS condiciones simultáneas, y el unit cumple las dos:
#
#   1. sin SUDO_UID  -> el chequeo de propiedad se dispara (bajo sudo git usa SUDO_UID para
#      decidir de quién es el repo; /opt/services es de sysadmin y coinciden)
#   2. sin HOME      -> git NO PUEDE LEER la excepción safe.directory que install.sh dejó
#      en /root/.gitconfig
#
# Tabla de verdad medida:
#   sudo git -C /opt/services status                          -> PASA
#   sudo env -u HOME git -C /opt/services status              -> PASA
#   sudo env -u SUDO_UID git -C /opt/services status          -> PASA
#   sudo env -u SUDO_UID -u HOME git -C /opt/services status  -> FALLA
#
# Con UNA sola de las dos, git pasa. Por eso TODA prueba a mano da verde: sudo aporta las
# dos. El único camino que discrimina es correr git bajo systemd, como lo corre el timer.
# Ese es el test (2); el (3) existe para que nadie vuelva a "verificar" esto desde una shell.
#
# Uso: bash scripts/tests/test_auto_deploy_entorno.sh

set -uo pipefail

SELF="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)/$(basename "${BASH_SOURCE[0]}")"
REPO_ROOT="$(cd "$(dirname "$SELF")/../.." && pwd)"
UNIT="$REPO_ROOT/services/systemd/domain-auto-deploy.service"

if [[ ! -f "$UNIT" ]]; then
  echo "RED: $UNIT no existe" >&2
  exit 1
fi

WORK="$(mktemp -d)"
GITCONFIG_TOCADO=""
SAFE_DIR_AGREGADO=""
trap 'limpiar' EXIT

failed=0
skipped=0
skips=()

pass() { echo "PASS: $1"; }
fail() { echo "FAIL: $1"; failed=$((failed + 1)); }

# un skip acá es exactamente el falso verde que este change combate: se cuenta, se nombra
# y se vuelve a listar al final
skip() {
  echo "SKIP: $1 — este test no corrio"
  skips+=("$1")
  skipped=$((skipped + 1))
}

limpiar() {
  if [[ -n "$GITCONFIG_TOCADO" && -n "$SAFE_DIR_AGREGADO" ]]; then
    git config --file "$GITCONFIG_TOCADO" --unset-all safe.directory \
      "^$(printf '%s' "$SAFE_DIR_AGREGADO" | sed 's/[.[\*^$\\]/\\&/g')$" 2>/dev/null || true
  fi
  rm -rf "$WORK"
}

assert_contains() {
  local label="$1" needle="$2" haystack="$3"
  if [[ "$haystack" == *"$needle"* ]]; then pass "$label"; else
    fail "$label — no encontré '$needle' en: $haystack"
  fi
}

assert_no_contains() {
  local label="$1" needle="$2" haystack="$3"
  if [[ "$haystack" != *"$needle"* ]]; then pass "$label"; else
    fail "$label — apareció '$needle' en: $haystack"
  fi
}

# el HOME que el unit declara, o vacío si todavía no declara ninguno
home_declarado_por_el_unit() {
  grep -E '^Environment="?HOME=' "$UNIT" 2>/dev/null | head -1 \
    | sed -E 's/^Environment="?HOME=([^"]*)"?[[:space:]]*$/\1/'
}

# --- (1) TestUnit_AutoDeploy_DeclaraHomeExplicito ----------------------------------------
# El unit no declara User= ni Environment=, así que systemd lo corre como root con un
# entorno mínimo: sin HOME. La excepción safe.directory que install.sh escribe en
# /root/.gitconfig existe, pero git no tiene cómo encontrarla.
if grep -qE '^Environment="?HOME=/root"?[[:space:]]*$' "$UNIT"; then
  pass "TestUnit_AutoDeploy_DeclaraHomeExplicito"
else
  fail "TestUnit_AutoDeploy_DeclaraHomeExplicito — $UNIT no declara 'Environment=HOME=/root': git no puede leer safe.directory de /root/.gitconfig"
fi

# --- (2) TestAutoDeploy_BajoSystemd_NoFallaPorDubiousOwnership ---------------------------
# EL ÚNICO CAMINO QUE DISCRIMINA. Reproduce las dos condiciones a la vez: un repo cuyo
# dueño NO es el euid que corre git, y git lanzado por systemd con los MISMOS -p que el
# unit (WorkingDirectory + EnvironmentFile + los Environment= que el unit declare).
#
# Las precondiciones son duras a propósito:
#   - scope SYSTEM: bajo --user el euid coincide con el dueño y el chequeo NUNCA se dispara,
#     así que el test daría verde con el bug intacto
#   - root: sin chown no hay forma de que el dueño difiera del euid
uid_de_otro_dueno() {
  local uid="${SUDO_UID:-}"
  [[ -n "$uid" ]] || uid="$(stat -c %u "$REPO_ROOT")"
  [[ "$uid" != "0" ]] && printf '%s' "$uid"
}

# /opt/services es de sysadmin y el timer corre como root: mismo desbalance
montar_repo_de_otro_dueno() {
  local repo="$1" dueno_uid="$2"
  git init -q -b main "$repo"
  git -C "$repo" config user.email t@test
  git -C "$repo" config user.name t
  git -C "$repo" config commit.gpgsign false
  git -C "$repo" commit -q --allow-empty -m "base"
  chown -R "$dueno_uid:" "$repo"
}

precondiciones_systemd() {
  local label="$1"
  command -v systemd-run >/dev/null 2>&1 || { skip "$label: systemd-run no disponible"; return 1; }
  if [[ "$(id -u)" != "0" ]]; then
    skip "$label: hace falta root para que el dueño del repo difiera del euid de git (chown) y para systemd-run en scope system"
    return 1
  fi
  systemd-run --quiet --collect --pipe --wait /bin/true >/dev/null 2>&1 \
    || { skip "$label: systemd-run no disponible (el manager de sistema rechazó la unidad transitoria)"; return 1; }
  if [[ -z "$(uid_de_otro_dueno)" ]]; then
    skip "$label: no hay un uid no-root para hacer que el dueño difiera; el chequeo de propiedad no se dispararía"
    return 1
  fi
}

correr_test_bajo_systemd() {
  local label="TestAutoDeploy_BajoSystemd_NoFallaPorDubiousOwnership"
  precondiciones_systemd "$label" || return

  local repo="$WORK/repo-ajeno"
  montar_repo_de_otro_dueno "$repo" "$(uid_de_otro_dueno)"

  local env_file="$WORK/entorno.env"
  printf 'DOMAIN_TEST=1\n' > "$env_file"

  local args=(--quiet --collect --pipe --wait
    -p "WorkingDirectory=$repo"
    -p "EnvironmentFile=$env_file")
  local linea
  while IFS= read -r linea; do
    [[ -n "$linea" ]] && args+=(-p "$linea")
  done < <(grep -E '^Environment=' "$UNIT" 2>/dev/null)

  # la excepción que install.sh (STEP 3) deja en el gitconfig global de root: solo sirve si
  # el proceso tiene HOME para encontrarla
  local home_unit
  home_unit="$(home_declarado_por_el_unit)"
  if [[ -n "$home_unit" && -d "$home_unit" ]]; then
    GITCONFIG_TOCADO="$home_unit/.gitconfig"
    SAFE_DIR_AGREGADO="$repo"
    git config --file "$GITCONFIG_TOCADO" --add safe.directory "$repo"
  fi

  local salida rc
  salida=$(systemd-run "${args[@]}" "$(command -v git)" status --porcelain 2>&1); rc=$?

  assert_no_contains "$label — git bajo systemd no rebota por propiedad dudosa" \
    "dubious ownership" "$salida"
  if (( rc == 0 )); then
    pass "$label — git bajo systemd sale 0"
  else
    fail "$label — git bajo systemd salió $rc: $salida"
  fi
}
correr_test_bajo_systemd

# --- (3) TestAutoDeploy_InvocadoDesdeShell_NoValidaElFix ----------------------------------
# Guard METODOLÓGICO, no de código. El camino de shell con sudo PASA aunque el fix no esté,
# porque sudo aporta SUDO_UID y HOME de una sola vez. Quien "verifique" así va a ver verde
# con el timer roto, que es como este bug sobrevivió sin ser detectado.
label3="TestAutoDeploy_InvocadoDesdeShell_NoValidaElFix"

# las líneas del propio guard nombran lo que prohíben, así que se marcan y se excluyen:
# sin esto el guard se detecta a sí mismo y falla siempre
cuerpo_ejecutable() {
  grep -vE '^[[:space:]]*#' "$SELF" | grep -v 'guard-metodologico'
}

# 3a — que esta suite no se verifique a sí misma por el camino que no discrimina
usos_de_sudo=$(cuerpo_ejecutable | grep -nE '(^|[^[:alnum:]_/-])sudo[[:space:]]' || true)
if [[ -z "$usos_de_sudo" ]]; then
  pass "$label3 — la suite no invoca sudo como método de verificación" # guard-metodologico
else
  fail "$label3 — la suite invoca sudo, y ese camino no discrimina el fix — $usos_de_sudo"
fi

if cuerpo_ejecutable | grep -q 'systemd-run'; then
  pass "$label3 — la verificación real pasa por systemd-run, no por la shell"
else
  fail "$label3 — la suite no invoca systemd-run: no queda ningún camino que discrimine el fix"
fi

# --user corre con el euid del usuario: el dueño coincide y el chequeo jamás se dispara
if cuerpo_ejecutable | grep -q 'systemd-run --user'; then # guard-metodologico
  fail "$label3 — la suite usa el scope de usuario: ahí el euid coincide con el dueño y el test daría verde con el bug intacto"
else
  pass "$label3 — la suite no usa el scope de usuario (daría falso verde)"
fi

# 3b — la demostración empírica de que el camino de shell no discrimina
demostrar_que_la_shell_no_discrimina() {
  if [[ "$(id -u)" != "0" ]]; then
    skip "$label3 (comparación empírica): hace falta root; git solo mira SUDO_UID cuando el euid es 0"
    return
  fi

  local dueno_uid
  dueno_uid="$(uid_de_otro_dueno)"
  if [[ -z "$dueno_uid" ]]; then
    skip "$label3 (comparación empírica): no hay un uid no-root para hacer que el dueño difiera"
    return
  fi

  local repo="$WORK/repo-shell"
  montar_repo_de_otro_dueno "$repo" "$dueno_uid"

  # exactamente lo que una shell con sudo aporta, y que el unit NO aporta
  local salida
  salida=$(SUDO_UID="$dueno_uid" HOME="${HOME:-/root}" git -C "$repo" status --porcelain 2>&1)
  if [[ "$salida" != *"dubious ownership"* ]]; then
    pass "$label3 — con SUDO_UID+HOME (lo que da sudo) git pasa aunque el fix no esté: el camino de shell NO verifica nada"
  else
    fail "$label3 — el camino de shell falló, así que la premisa medida ya no se sostiene: $salida"
  fi
}
demostrar_que_la_shell_no_discrimina

# --- reporte ------------------------------------------------------------------------------
if (( skipped > 0 )); then
  echo
  echo "NO CORRIERON ($skipped):"
  for s in "${skips[@]}"; do echo "  - $s"; done
  echo
fi

if (( failed > 0 )); then
  echo "RED — $failed tests fallaron, $skipped no corrieron"
  exit 1
fi

if (( skipped > 0 )); then
  echo "AMARILLO — 0 fallaron, pero $skipped no corrieron: NO es una verificación del fix"
  exit 0
fi

echo "GREEN — todos los tests pasaron"
exit 0
