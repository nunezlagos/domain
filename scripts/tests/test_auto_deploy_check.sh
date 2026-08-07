#!/usr/bin/env bash
# scripts/tests/test_auto_deploy_check.sh
#
# REQ-5 de issue-57.1 (fase 5) — auto-deploy del VPS disparado por tag.
#
# El decisor NO despliega: decide SI hay que desplegar y delega en scripts/deploy.sh,
# que ya tiene sus 5 fases, su trap de rollback y su --dry-run. Lo que se prueba acá es
# exactamente esa decisión, que es donde vive todo el riesgo:
#
#   - un tag nuevo que ES la punta de origin/main  -> despliega
#   - un tag nuevo que NO es la punta de main      -> se abstiene (código no publicado)
#   - sin tag nuevo                                -> NOOP, nada se reinicia
#   - sin ningún tag                               -> NOOP (VPS recién instalado)
#   - dos ciclos solapados                         -> el segundo se retira
#
# Método: repos git temporales de verdad (bare origin + working copy) y un scripts/deploy.sh
# de mentira DENTRO del repo temporal, que registra si lo invocaron y con qué. Como el
# decisor resuelve deploy.sh contra DEPLOY_REPO_ROOT, el shim entra sin ninguna puerta de
# test en el código de producción.
#
# Uso: ./scripts/tests/test_auto_deploy_check.sh

set -uo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
REPO_ROOT="$(cd "$SCRIPT_DIR/../.." && pwd)"
CHECK="$REPO_ROOT/scripts/auto-deploy-check.sh"

if [[ ! -f "$CHECK" ]]; then
  echo "RED: $CHECK no existe aun — esperado para green phase" >&2
  exit 1
fi

command -v flock >/dev/null 2>&1 || { echo "SKIP: hace falta flock (util-linux)" >&2; exit 0; }

WORK="$(mktemp -d)"
trap 'rm -rf "$WORK"' EXIT

failed=0

pass() { echo "PASS: $1"; }
fail() { echo "FAIL: $1"; failed=$((failed + 1)); }

assert_eq() {
  local label="$1" expected="$2" actual="$3"
  if [[ "$expected" == "$actual" ]]; then pass "$label"; else
    fail "$label — expected='$expected' actual='$actual'"
  fi
}

assert_contains() {
  local label="$1" needle="$2" haystack="$3"
  if [[ "$haystack" == *"$needle"* ]]; then pass "$label"; else
    fail "$label — no encontré '$needle' en: $haystack"
  fi
}

# Monta bare origin + working copy con el shim de deploy.sh ya commiteado.
# El shim registra su invocación en $repo/.deploy-invocaciones, fuera del árbol versionado.
montar_repo() {
  local nombre="$1"
  local origin="$WORK/$nombre.git" repo="$WORK/$nombre"

  git init --bare -q -b main "$origin"
  git init -q -b main "$repo"
  git -C "$repo" config user.email t@test
  git -C "$repo" config user.name t
  git -C "$repo" config commit.gpgsign false
  git -C "$repo" config tag.gpgsign false

  mkdir -p "$repo/scripts" "$repo/services/domain-mcp"
  cat > "$repo/scripts/deploy.sh" <<'SHIM'
#!/usr/bin/env bash
echo "PREV_SHA=${PREV_SHA:-<unset>} args=$*" >> "$(dirname "$0")/../.deploy-invocaciones"
exit "${DEPLOY_SHIM_RC:-0}"
SHIM
  chmod +x "$repo/scripts/deploy.sh"
  printf 'mcp\n' > "$repo/services/domain-mcp/Dockerfile"
  git -C "$repo" add -A
  git -C "$repo" commit -q -m "base"
  git -C "$repo" remote add origin "$origin"
  git -C "$repo" push -q -u origin main
}

commitear() {
  local repo="$1" mensaje="$2"
  printf '%s\n' "$mensaje" >> "$repo/services/domain-mcp/Dockerfile"
  git -C "$repo" commit -aq -m "$mensaje"
}

# Publica en el origin un commit + su tag, y deja la working copy DETRÁS: es el estado real
# del VPS cuando alguien acaba de taggear desde otra máquina.
publicar_con_tag() {
  local repo="$1" tag="$2"
  local clon="$WORK/publicador-$$-$RANDOM"
  git clone -q "$repo/../$(basename "$repo").git" "$clon"
  git -C "$clon" config user.email p@test
  git -C "$clon" config user.name p
  git -C "$clon" config tag.gpgsign false
  git -C "$clon" commit -q --allow-empty -m "release $tag"
  git -C "$clon" tag -a "$tag" -m "$tag"
  git -C "$clon" push -q origin main
  git -C "$clon" push -q origin "$tag"
  rm -rf "$clon"
}

publicar_commit_sin_tag() {
  local repo="$1" mensaje="$2"
  local clon="$WORK/publicador-$$-$RANDOM"
  git clone -q "$repo/../$(basename "$repo").git" "$clon"
  git -C "$clon" config user.email p@test
  git -C "$clon" config user.name p
  git -C "$clon" commit -q --allow-empty -m "$mensaje"
  git -C "$clon" push -q origin main
  rm -rf "$clon"
}

correr_check() {
  local repo="$1"; shift
  DEPLOY_REPO_ROOT="$repo" bash "$CHECK" "$@" 2>&1
}

invocaciones() {
  cat "$1/.deploy-invocaciones" 2>/dev/null || true
}

# --- (a) un tag nuevo, y es la punta de origin/main -> despliega -------------------------
montar_repo repo-a
publicar_con_tag "$WORK/repo-a" v0.1.0
SHA_A_PREVIO=$(git -C "$WORK/repo-a" rev-parse HEAD)
out_a=$(correr_check "$WORK/repo-a"); rc_a=$?
assert_eq "(a) tag nuevo en la punta de main -> exit 0" "0" "$rc_a"
inv_a=$(invocaciones "$WORK/repo-a")
if [[ -n "$inv_a" ]]; then
  pass "(a) tag nuevo -> deploy.sh fue invocado"
else
  fail "(a) tag nuevo -> deploy.sh NO se invocó; salida: $out_a"
fi
assert_contains "(a) el deploy recibe PREV_SHA con el SHA que estaba desplegado" \
  "PREV_SHA=$SHA_A_PREVIO" "$inv_a"
assert_contains "(a) el log nombra el tag que dispara el deploy" "v0.1.0" "$out_a"

# --- (b) sin tag nuevo -> NOOP, nada se reinicia -----------------------------------------
montar_repo repo-b
publicar_con_tag "$WORK/repo-b" v0.1.0
correr_check "$WORK/repo-b" >/dev/null   # primer ciclo: despliega (shim no mueve HEAD)
# el shim no hace pull, así que se simula el efecto del deploy real moviendo HEAD al tag
git -C "$WORK/repo-b" merge -q --ff-only origin/main
rm -f "$WORK/repo-b/.deploy-invocaciones"
out_b=$(correr_check "$WORK/repo-b"); rc_b=$?
assert_eq "(b) sin tag nuevo -> exit 0" "0" "$rc_b"
assert_eq "(b) sin tag nuevo -> deploy.sh NO se invoca" "" "$(invocaciones "$WORK/repo-b")"

# --- (c1) ya desplegado y main siguió avanzando sin tag -> NOOP --------------------------
# El caso cotidiano de "un push a main sin tag NO despliega": el VPS está en el último tag
# y alguien mergeó a main sin publicar.
montar_repo repo-c1
publicar_con_tag "$WORK/repo-c1" v0.1.0
git -C "$WORK/repo-c1" fetch -q --tags origin
git -C "$WORK/repo-c1" merge -q --ff-only origin/main
rm -f "$WORK/repo-c1/.deploy-invocaciones"
publicar_commit_sin_tag "$WORK/repo-c1" "hotfix sin taggear"
out_c1=$(correr_check "$WORK/repo-c1"); rc_c1=$?
assert_eq "(c1) main adelantado sin tag -> exit 0 (no es un error, es un estado)" "0" "$rc_c1"
assert_eq "(c1) main adelantado sin tag -> deploy.sh NO se invoca" "" "$(invocaciones "$WORK/repo-c1")"

# --- (c2) hay tag NUEVO, pero main ya lo pasó de largo -> se abstiene ---------------------
# El caso peligroso, y la única razón por la que el decisor compara contra origin/main:
# hay un tag sin desplegar, así que el ciclo va a querer actuar — pero deploy.sh hace
# pull_ff de origin/main, o sea que desplegaría el hotfix sin taggear que quedó ARRIBA del
# tag. Sin esta comparación, este escenario pone en producción código no publicado.
montar_repo repo-c2
publicar_con_tag "$WORK/repo-c2" v0.2.0
publicar_commit_sin_tag "$WORK/repo-c2" "hotfix sin taggear"
out_c2=$(correr_check "$WORK/repo-c2"); rc_c2=$?
assert_eq "(c2) tag nuevo por debajo de main -> exit 0" "0" "$rc_c2"
assert_eq "(c2) tag nuevo por debajo de main -> deploy.sh NO se invoca" "" "$(invocaciones "$WORK/repo-c2")"
assert_contains "(c2) el log explica que el tag no es la punta de main" "main" "$out_c2"

# --- (d) repo sin ningún tag -> NOOP ------------------------------------------------------
# Un VPS recién instalado no tiene tag desplegado. Tratar eso como error dejaría el timer
# fallando en cada ciclo desde el minuto cero.
montar_repo repo-d
out_d=$(correr_check "$WORK/repo-d"); rc_d=$?
assert_eq "(d) sin ningún tag v* -> exit 0" "0" "$rc_d"
assert_eq "(d) sin ningún tag v* -> deploy.sh NO se invoca" "" "$(invocaciones "$WORK/repo-d")"

# --- (e) orden de versiones: v0.10.0 gana a v0.9.0 ---------------------------------------
# Con sort alfabético v0.10.0 queda ANTES que v0.9.0 y el auto-deploy se apagaría solo en
# cuanto el minor llegue a dos dígitos.
montar_repo repo-e
publicar_con_tag "$WORK/repo-e" v0.9.0
publicar_con_tag "$WORK/repo-e" v0.10.0
out_e=$(correr_check "$WORK/repo-e"); rc_e=$?
assert_eq "(e) con v0.9.0 y v0.10.0 -> exit 0" "0" "$rc_e"
assert_contains "(e) elige v0.10.0 como el último tag" "v0.10.0" "$out_e"
if [[ "$out_e" == *"v0.9.0"* ]]; then
  fail "(e) eligió v0.9.0: el orden es alfabético, no de versión — salida: $out_e"
else
  pass "(e) NO eligió v0.9.0"
fi

# --- (f) dos ciclos no se pisan -----------------------------------------------------------
# El deploy tarda minutos y el timer corre cada pocos; sin lock, dos build/restart
# simultáneos sobre los mismos containers.
montar_repo repo-f
publicar_con_tag "$WORK/repo-f" v0.1.0
LOCK_F="$WORK/repo-f/.git/auto-deploy.lock"
: > "$LOCK_F"
flock -x "$LOCK_F" sleep 5 &
ocupante=$!
sleep 0.5
out_f=$(correr_check "$WORK/repo-f"); rc_f=$?
kill "$ocupante" 2>/dev/null
wait "$ocupante" 2>/dev/null
assert_eq "(f) con un ciclo en curso -> exit 0" "0" "$rc_f"
assert_eq "(f) con un ciclo en curso -> deploy.sh NO se invoca" "" "$(invocaciones "$WORK/repo-f")"
assert_contains "(f) el log dice que se retira por un ciclo en curso" "curso" "$out_f"

# --- (g) el fallo del deploy se propaga ---------------------------------------------------
# Si el decisor se tragara el exit code, systemd marcaría el ciclo OK y un deploy roto
# quedaría invisible. El rollback ya lo hace el trap de deploy.sh; acá lo que importa es
# que el fallo SALGA.
montar_repo repo-g
publicar_con_tag "$WORK/repo-g" v0.1.0
out_g=$(DEPLOY_SHIM_RC=17 correr_check "$WORK/repo-g"); rc_g=$?
assert_eq "(g) deploy fallido -> el decisor sale != 0 con el mismo código" "17" "$rc_g"

# --- (h) --dry-run llega hasta deploy.sh --------------------------------------------------
montar_repo repo-h
publicar_con_tag "$WORK/repo-h" v0.1.0
correr_check "$WORK/repo-h" --dry-run >/dev/null
assert_contains "(h) --dry-run se le pasa a deploy.sh" "args=--dry-run" "$(invocaciones "$WORK/repo-h")"

# --- (i) la unidad systemd apunta a algo que existe --------------------------------------
# Un ExecStart con una ruta equivocada no falla al copiar la unidad: falla en el primer
# disparo del timer, dentro del journal, semanas después y sin que nadie mire.
UNIT="$REPO_ROOT/services/systemd/domain-auto-deploy.service"
if [[ -f "$UNIT" ]]; then
  ejecutable=$(grep '^ExecStart=' "$UNIT" | cut -d= -f2-)
  relativo="${ejecutable#/opt/services/}"
  if [[ -x "$REPO_ROOT/$relativo" ]]; then
    pass "(i) el ExecStart de la unidad existe y es ejecutable en el repo"
  else
    fail "(i) ExecStart='$ejecutable' no resuelve a un ejecutable del repo ($relativo)"
  fi
  assert_contains "(i) la unidad corre desde el root del repo, no desde services/" \
    "WorkingDirectory=/opt/services" "$(grep '^WorkingDirectory=' "$UNIT")"
  if [[ "$(grep '^WorkingDirectory=' "$UNIT")" == "WorkingDirectory=/opt/services/services" ]]; then
    fail "(i) WorkingDirectory apunta a services/, pero el script vive en el root del repo"
  fi
else
  fail "(i) falta $UNIT"
fi

# --- (j) el redeploy deja el auto-deploy ANDANDO ------------------------------------------
# CONTRATO INVERTIDO por decisión del usuario (2026-08-07). Antes este assert exigía lo
# contrario: el timer quedaba instalado e inactivo porque el pedido era no encender nada en
# el VPS. Ahora el pedido es que instalar sea UN solo paso, así que el guard cambia de lado.
#
# Sigue haciendo falta, solo que ahora defiende el modo de falla opuesto: install.sh copia
# las unidades con un GLOB pero las enciende NOMBRÁNDOLAS una por una, así que olvidarse de
# nombrar esta la deja instalada y apagada — un auto-deploy que no corre nunca y que no
# produce ningún síntoma, porque el archivo está donde debe.
INSTALL_SH="$REPO_ROOT/services/install.sh"
if grep -q "enable --now domain-auto-deploy.timer" "$INSTALL_SH"; then
  pass "(j) install.sh activa el timer de auto-deploy"
else
  fail "(j) install.sh NO activa domain-auto-deploy.timer: el redeploy dejaría el auto-deploy instalado pero apagado"
fi

# --- (k) el ExecStart existe en el layout del VPS, no solo en el repo ---------------------
# MEDIDO en el VPS 2026-08-07: install.sh clona el repo ENTERO en INSTALL_DIR, así que
# /opt/services/scripts/ y su lib/ existen. Es la premisa que sostiene el ExecStart: si
# alguien lo cambiara por un rsync selectivo, el timer fallaría en cada ciclo dentro del
# journal, semanas después y sin que nadie lo mire.
if grep -qE 'git clone .*"\$INSTALL_DIR"' "$INSTALL_SH"; then
  pass "(k) install.sh clona el repo completo: el ExecStart resuelve en el VPS"
else
  fail "(k) install.sh ya no clona el repo completo en INSTALL_DIR: el ExecStart del timer podría no existir"
fi

if (( failed > 0 )); then
  echo "RED — $failed tests fallaron"
  exit 1
fi

echo "GREEN — todos los tests pasaron"
exit 0
