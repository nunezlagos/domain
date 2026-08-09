#!/usr/bin/env bash
# scripts/tests/test_capas_independientes.sh
#
# issue-57.2 / DOMAINSERV-258 — MUST-6: el fix primario no queda enmascarado por la
# defensa en profundidad.
#
# El change tiene dos capas y el ADR-1 sostiene que NO pueden taparse mutuamente porque
# no arreglan lo mismo:
#
#   capa ENTORNO        Environment=HOME=/root en el unit.
#                       Propósito: que el SERVICIO corra. Sin HOME, git no lee el
#                       safe.directory de /root/.gitconfig y aborta con exit 128.
#
#   capa NORMALIZACIÓN  chown en install.sh.
#                       Propósito: que SYSADMIN pueda operar el repo a mano, después de
#                       que 1940 rutas quedaran en manos de root. No arregla el servicio:
#                       el auto-deploy corre como root, así que normalizar a sysadmin deja
#                       la condición de "dueño distinto del euid" exactamente igual.
#
# Ese argumento es correcto y no es una verificación. El MUST-6 existe porque este
# proyecto tiene el antecedente registrado de un bug que volvió con la suite en verde, y
# un criterio que se cumple "por construcción" según un ADR es precisamente un guard que
# nadie ejecuta. Este test lo ejecuta.
#
# MÉTODO: matriz de sabotaje cruzado 2x2. Se copia lo mínimo del repo a un tmpdir, se
# rompe UNA capa por vez, y se corre AMBAS suites contra la copia:
#
#                          suite entorno    suite normalización
#   sabotear entorno    ->     ROJO               VERDE
#   sabotear chown      ->     VERDE              ROJO
#
# Las diagonales son el criterio: si romper una capa pone en rojo la suite de la OTRA,
# están acopladas. Si romper una capa NO pone en rojo su propia suite, esa capa no tiene
# guard y el MUST-6 no se cumple aunque todo lo demás siga verde.
#
# El repo real NUNCA se modifica: el sabotaje ocurre sobre la copia. Es deliberado —
# sabotear in-place y restaurar deja una ventana en la que un fallo del script te deja el
# repo roto, y `git checkout --` dentro de un .sh evade el git-guard del proyecto.
#
# Uso: bash scripts/tests/test_capas_independientes.sh

set -uo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
REPO_ROOT="$(cd "$SCRIPT_DIR/../.." && pwd)"

UNIT_REL="services/systemd/domain-auto-deploy.service"
LIB_REL="services/scripts/normalizar-duenos.sh"
SUITE_ENTORNO_REL="scripts/tests/test_auto_deploy_entorno.sh"
SUITE_NORMALIZA_REL="scripts/tests/test_install_normaliza_duenos.sh"

WORK="$(mktemp -d)"
trap 'rm -rf "$WORK"' EXIT

asserts_fallados=0
pass() { echo "  PASS: $1"; }
fail() { echo "  FAIL: $1"; asserts_fallados=$((asserts_fallados + 1)); }

# Copia mínima del repo. services/ pesa 130M —hay certs y datos ahí— así que se copian
# los archivos que las suites leen y nada más; un cp -r del árbol entero por cada sabotaje
# haría el test inusable.
crear_espejo() {
  local destino="$1"
  mkdir -p "$destino/scripts/tests" "$destino/services/systemd" "$destino/services/scripts"
  cp -a "$REPO_ROOT/scripts/." "$destino/scripts/"
  cp -a "$REPO_ROOT/services/systemd/." "$destino/services/systemd/"
  cp -a "$REPO_ROOT/services/scripts/." "$destino/services/scripts/"
}

# Corre una suite contra el espejo y devuelve solo su veredicto. El rc es lo que importa:
# las suites de este change salen != 0 cuando algún test falla.
correr_suite() {
  local espejo="$1" suite="$2"
  ( cd "$espejo" && bash "$suite" >/dev/null 2>&1 )
  echo $?
}

# Un sabotaje que no rompe nada produce un falso PASS en toda la matriz: la suite queda
# verde porque la capa sigue intacta, no porque las capas sean independientes. Por eso se
# verifica que el archivo REALMENTE cambió antes de creerle al resultado.
verificar_que_el_sabotaje_mordio() {
  local espejo="$1" rel="$2" etiqueta="$3"
  if [[ "$(sha256sum < "$REPO_ROOT/$rel")" == "$(sha256sum < "$espejo/$rel")" ]]; then
    fail "$etiqueta: el sabotaje no modificó $rel — la matriz mediría nada"
    return 1
  fi
  pass "$etiqueta: el sabotaje modificó $rel"
  return 0
}

TestCapasIndependientes_QuitarUnaNoTapaLaOtra() {
  # --- línea base: sin sabotaje, las dos suites verdes ---
  local base="$WORK/base"
  crear_espejo "$base"
  local base_entorno base_normaliza
  base_entorno="$(correr_suite "$base" "$SUITE_ENTORNO_REL")"
  base_normaliza="$(correr_suite "$base" "$SUITE_NORMALIZA_REL")"
  # sin esto, una suite rota de antemano haría pasar las dos diagonales por la razón
  # equivocada: el rojo esperado estaría ahí desde antes del sabotaje
  [[ "$base_entorno" == "0" ]] \
    && pass "línea base: la suite de entorno arranca verde" \
    || fail "línea base: la suite de entorno YA está en rojo (rc=$base_entorno), la matriz no discrimina"
  [[ "$base_normaliza" == "0" ]] \
    && pass "línea base: la suite de normalización arranca verde" \
    || fail "línea base: la suite de normalización YA está en rojo (rc=$base_normaliza), la matriz no discrimina"

  # --- sabotaje A: quitar la capa de entorno ---
  local sab_a="$WORK/sabotaje-entorno"
  crear_espejo "$sab_a"
  sed -i '/^Environment=HOME=/d' "$sab_a/$UNIT_REL"
  if verificar_que_el_sabotaje_mordio "$sab_a" "$UNIT_REL" "sabotaje A"; then
    local a_entorno a_normaliza
    a_entorno="$(correr_suite "$sab_a" "$SUITE_ENTORNO_REL")"
    a_normaliza="$(correr_suite "$sab_a" "$SUITE_NORMALIZA_REL")"
    [[ "$a_entorno" != "0" ]] \
      && pass "quitar el entorno pone EN ROJO la suite de entorno" \
      || fail "quitar Environment=HOME=/root y la suite de entorno sigue verde: esa capa no tiene guard"
    [[ "$a_normaliza" == "0" ]] \
      && pass "quitar el entorno NO toca la suite de normalización" \
      || fail "quitar el entorno rompió la suite de normalización (rc=$a_normaliza): las capas están acopladas"
  fi

  # --- sabotaje B: quitar la capa de normalización ---
  local sab_b="$WORK/sabotaje-chown"
  crear_espejo "$sab_b"
  # se neutraliza el chown dejando la función y sus guardas intactas: así el sabotaje
  # apunta al efecto de la capa y no a que el archivo deje de sourcear
  sed -i 's/chown -ch "\$dueno" "\$ruta"/true "\$dueno" "\$ruta"/' "$sab_b/$LIB_REL"
  if verificar_que_el_sabotaje_mordio "$sab_b" "$LIB_REL" "sabotaje B"; then
    local b_entorno b_normaliza
    b_entorno="$(correr_suite "$sab_b" "$SUITE_ENTORNO_REL")"
    b_normaliza="$(correr_suite "$sab_b" "$SUITE_NORMALIZA_REL")"
    [[ "$b_normaliza" != "0" ]] \
      && pass "quitar el chown pone EN ROJO la suite de normalización" \
      || fail "neutralizar el chown y la suite de normalización sigue verde: esa capa no tiene guard"
    [[ "$b_entorno" == "0" ]] \
      && pass "quitar el chown NO toca la suite de entorno" \
      || fail "quitar el chown rompió la suite de entorno (rc=$b_entorno): las capas están acopladas"
  fi

  # --- el repo real quedó intacto ---
  local rel intactos=""
  for rel in "$UNIT_REL" "$LIB_REL"; do
    [[ -f "$REPO_ROOT/$rel" ]] || intactos+="$rel(ausente) "
  done
  grep -q '^Environment=HOME=/root$' "$REPO_ROOT/$UNIT_REL" \
    || intactos+="$UNIT_REL(sin HOME) "
  grep -q 'chown -ch' "$REPO_ROOT/$LIB_REL" \
    || intactos+="$LIB_REL(sin chown -ch) "
  [[ -z "$intactos" ]] \
    && pass "el repo real quedó intacto: el sabotaje vivió en el tmpdir" \
    || fail "el repo real quedó tocado: $intactos"
}

echo "--- TestCapasIndependientes_QuitarUnaNoTapaLaOtra"
TestCapasIndependientes_QuitarUnaNoTapaLaOtra

if (( asserts_fallados > 0 )); then
  echo "RED — $asserts_fallados asserts fallaron: las capas NO son independientes"
  exit 1
fi

echo "GREEN — las dos capas fallan por separado y ninguna tapa a la otra"
exit 0
