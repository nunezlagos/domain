#!/usr/bin/env bash
# scripts/tests/test_install_normaliza_duenos.sh
#
# issue-57.2 / DOMAINSERV-258 — normalización de dueños del repo del VPS.
#
# Esta suite NO prueba el fix del auto-deploy. El servicio corre como root y falla por su
# ENTORNO (sin SUDO_UID el chequeo de propiedad de git se dispara; sin HOME git no puede
# leer la excepción safe.directory que install.sh dejó en /root/.gitconfig): un chown a
# sysadmin lo deja igual de roto. Lo que se prueba acá es la OTRA capa, que tiene otro
# propósito: que sysadmin pueda operar el repo A MANO después de que 1940 rutas quedaran
# en manos de root.
#
# Y sobre todo se prueba la GUARDA. El comando que consigue eso es un `chown -R` cuyo
# objetivo sale de una variable: si INSTALL_DIR llega vacía o relativa, el recursivo se
# come el cwd o el filesystem entero, sin vuelta atrás. Los tests 3 y 4 verifican que la
# guarda está ANTES del comando, no después.
#
# CONTRATO ESPERADO (fase verde):
#
#   source services/scripts/normalizar-duenos.sh     # o scripts/lib/normalizar_duenos.sh
#   normalizar_duenos <install_dir> [dueno=sysadmin]
#
#     -> aplica `chown -Rc <dueno> <install_dir>` y NADA MÁS: nunca chmod, nunca
#        --reference, nunca un chown por archivo con modo de por medio.
#     -> deja en la global DUENOS_CAMBIADOS la cantidad de rutas que REALMENTE cambiaron
#        de dueño. Sale de la salida de `--changes` del propio chown, no de un barrido
#        previo con `find -not -user`: ese barrido es una segunda pasada completa sobre
#        el repo y corre en otro instante que el cambio.
#     -> return != 0, con mensaje, y SIN ejecutar ningún chown si <install_dir> está
#        vacía, es relativa, o es "/".
#
#   La normalización tiene que vivir en una FUNCIÓN SOURCEABLE. Inline dentro de
#   install.sh es intesteable: install.sh corre de arriba a abajo con set -e y clona
#   repos, así que no se puede sourcear para llamar una sola parte.
#
# MÉTODO: árbol de prueba en un tmpdir y un stub de `chown` en el PATH. Cambiar dueños de
# verdad exige root, así que el stub mantiene un registro simulado (ruta -> dueño) y emite
# las líneas de `--changes` solo para las rutas que cambian. Los modos, en cambio, son
# REALES: `chmod` no está stubeado a propósito, para medir el efecto y no la llamada.
# NUNCA se toca /opt/services ni ninguna ruta del sistema.
#
# Uso: bash scripts/tests/test_install_normaliza_duenos.sh

set -uo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
REPO_ROOT="$(cd "$SCRIPT_DIR/../.." && pwd)"

# dos ubicaciones posibles: install.sh sourcea sus helpers desde services/scripts/, y
# scripts/lib/ es donde viven los del deploy — la suite acepta cualquiera de las dos
LIBS_CANDIDATAS=(
  "$REPO_ROOT/services/scripts/normalizar-duenos.sh"
  "$REPO_ROOT/scripts/lib/normalizar_duenos.sh"
)

DUENO_DESTINO="sysadmin"
DUENO_PREVIO="root"
# uid de postgres DENTRO de su container. Un chown del host no puede reconstruirlo, así
# que certs/postgres/server.key queda legítimamente con otro dueño y el criterio del
# MUST-3 lo exceptúa
UID_CONTAINER="999"

WORK="$(mktemp -d)"
trap 'rm -rf "$WORK"' EXIT
# el stub lo usa para negarse a expandir cualquier objetivo de afuera del tmpdir
export WORK_PRUEBA="$WORK"

STUB_DIR="$WORK/bin"
mkdir -p "$STUB_DIR"

cat > "$STUB_DIR/chown" <<'STUB'
#!/usr/bin/env bash
# stub de chown: cambiar dueños de verdad exige root. Mantiene el dueño simulado de cada
# ruta en $REGISTRO (ruta<TAB>dueño) y registra cada invocación en $CHOWN_LOG.
printf '%s\n' "$*" >> "$CHOWN_LOG"

recursivo=0
cambios=0
sin_deref=0
spec=""
rutas=()
for arg in "$@"; do
  case "$arg" in
    --recursive) recursivo=1 ;;
    --changes|--verbose) cambios=1 ;;
    --no-dereference) sin_deref=1 ;;
    --*) ;;
    -*)
      [[ "$arg" == *R* ]] && recursivo=1
      [[ "$arg" == *c* || "$arg" == *v* ]] && cambios=1
      [[ "$arg" == *h* ]] && sin_deref=1
      ;;
    *)
      if [[ -z "$spec" ]]; then spec="$arg"; else rutas+=("$arg"); fi
      ;;
  esac
done

dueno="${spec%%:*}"
[[ -n "$dueno" ]] || { echo "chown-stub: falta el dueño" >&2; exit 1; }
(( ${#rutas[@]} > 0 )) || { echo "chown-stub: falta la ruta" >&2; exit 1; }

for ruta in "${rutas[@]}"; do
  # "/dir" y "/dir/" son el mismo objetivo para chown, pero find arrastra la barra a cada
  # línea y las claves del registro no matchearían
  while [[ "$ruta" == */ && ${#ruta} -gt 1 ]]; do ruta="${ruta%/}"; done
  # contención: la invocación ya quedó registrada, que es lo único que el test necesita
  # para fallar. Una implementación sin guarda apunta a "/" y expandirla acá colgaría la
  # suite recorriendo el filesystem entero en vez de fallarla en un segundo.
  if [[ "$ruta" != "$WORK_PRUEBA"/* ]]; then
    echo "chown-stub: '$ruta' cae fuera del tmpdir de la suite, no se expande" >&2
    continue
  fi
  objetivos=("$ruta")
  if (( recursivo )); then
    mapfile -t objetivos < <(find "$ruta" 2>/dev/null)
  fi
  for objetivo in "${objetivos[@]}"; do
    # la semántica que importa: sin -h, chown SIGUE el symlink y le cambia el dueño al
    # TARGET, dejando el enlace con su dueño original para siempre. Con -h opera sobre el
    # enlace mismo. Modelarlo es lo que permite medir la diferencia con un stub.
    if [[ -L "$objetivo" ]] && (( ! sin_deref )); then
      objetivo="$(readlink -f "$objetivo" 2>/dev/null)" || continue
      [[ -n "$objetivo" && "$objetivo" == "$WORK_PRUEBA"/* ]] || continue
    fi
    actual=$(awk -F'\t' -v p="$objetivo" '$1==p{v=$2} END{print v}' "$REGISTRO")
    [[ "$actual" == "$dueno" ]] && continue
    printf '%s\t%s\n' "$objetivo" "$dueno" >> "$REGISTRO"
    (( cambios )) && printf "changed ownership of '%s' from %s to %s\n" \
      "$objetivo" "${actual:-desconocido}" "$dueno"
  done
done
exit 0
STUB
chmod +x "$STUB_DIR/chown"

# === arnés ===

asserts_fallados=0
tests_fallados=0
test_actual=""
test_ok=1

pass() { echo "  PASS: $1"; }
fail() {
  echo "  FAIL: $1"
  asserts_fallados=$((asserts_fallados + 1))
  test_ok=0
}

assert_eq() {
  local label="$1" esperado="$2" actual="$3"
  if [[ "$esperado" == "$actual" ]]; then pass "$label"; else
    fail "$label — esperado='$esperado' actual='$actual'"
  fi
}

assert_ne() {
  local label="$1" prohibido="$2" actual="$3"
  if [[ "$prohibido" != "$actual" ]]; then pass "$label"; else
    fail "$label — no esperaba '$prohibido'"
  fi
}

assert_contains() {
  local label="$1" needle="$2" haystack="$3"
  if [[ "$haystack" == *"$needle"* ]]; then pass "$label"; else
    fail "$label — no encontré '$needle' en: $haystack"
  fi
}

# Sourcea la lib y deja limpia la global del contrato. Falla (return 1) si la lib no
# existe todavía o si no define la función — el estado de la fase roja.
cargar_lib() {
  unset DUENOS_CAMBIADOS
  local lib
  for lib in "${LIBS_CANDIDATAS[@]}"; do
    if [[ -f "$lib" ]]; then
      # shellcheck source=/dev/null
      source "$lib" || return 1
      declare -F normalizar_duenos >/dev/null && return 0
      echo "  (la lib $lib existe pero no define normalizar_duenos)"
      return 1
    fi
  done
  echo "  (no existe ninguna de: ${LIBS_CANDIDATAS[*]})"
  return 1
}

# Árbol de prueba con los modos que importan: dos secretos en 600 y las configs que los
# containers de monitoring leen POR EL PERMISO DE OTROS (prometheus corre como nobody,
# loki como 10001, grafana como 472: ninguno es dueño de nada de esto).
crear_arbol() {
  local dir="$1"
  mkdir -p "$dir/.git" "$dir/services/monitoring" "$dir/scripts"
  printf '[core]\n'          > "$dir/.git/config"
  printf 'DOMAIN_KEY=x\n'    > "$dir/.env"
  printf 'services:\n'       > "$dir/docker-compose.yml"
  printf 'scrape_configs:\n' > "$dir/services/monitoring/prometheus.yml"
  printf 'limits_config:\n'  > "$dir/services/monitoring/loki-config.yml"
  printf '#!/bin/sh\n'       > "$dir/scripts/deploy.sh"
  chmod 600 "$dir/.env" "$dir/.git/config"
  chmod 644 "$dir/docker-compose.yml" \
            "$dir/services/monitoring/prometheus.yml" \
            "$dir/services/monitoring/loki-config.yml"
  chmod 755 "$dir/scripts/deploy.sh"
}

# Réplica del layout REAL del VPS, medido el 2026-08-09: los per-service .env son symlinks
# a ../../.env y services/certs es un symlink a ../certs. Va aparte de crear_arbol porque
# certs/ está pruneado del recorrido, y los tests que exigen "ninguna ruta con otro dueño"
# sobre el árbol entero no aplican acá.
crear_arbol_con_symlinks() {
  local dir="$1"
  crear_arbol "$dir"
  mkdir -p "$dir/certs/postgres" "$dir/services/domain-mcp"
  printf -- '-----BEGIN\n' > "$dir/certs/postgres/server.key"
  chmod 600 "$dir/certs/postgres/server.key"
  ln -sfn ../certs      "$dir/services/certs"
  ln -sfn ../../.env    "$dir/services/domain-mcp/.env"
  ln -sfn ../.env       "$dir/services/.env"
}

# Estado inicial del VPS medido: todo el árbol en manos de root.
sembrar_duenos() {
  local dir="$1" dueno="$2"
  : > "$REGISTRO"
  local ruta
  while IFS= read -r ruta; do
    printf '%s\t%s\n' "$ruta" "$dueno" >> "$REGISTRO"
  done < <(find "$dir")
}

dueno_de() {
  awk -F'\t' -v p="$1" '$1==p{v=$2} END{print (v == "" ? "<sin-registro>" : v)}' "$REGISTRO"
}

# Prepara registro + log limpios para un test y deja la raíz del árbol en REPO_PRUEBA.
# No devuelve la ruta por stdout a propósito: en command substitution las asignaciones
# quedarían en el subshell y el registro se perdería.
preparar() {
  local nombre="$1"
  REGISTRO="$WORK/$nombre.registro"   # fuera del árbol: si no, find lo levantaría
  CHOWN_LOG="$WORK/$nombre.chown.log"
  REPO_PRUEBA="$WORK/$nombre/repo"
  export REGISTRO CHOWN_LOG
  : > "$REGISTRO"
  : > "$CHOWN_LOG"
  mkdir -p "$REPO_PRUEBA"
}

# Corre la función bajo prueba con el stub de chown al frente del PATH. Va en command
# substitution para que ni el PATH ni un `exit` de la guarda se escapen al arnés; el rc
# vuelve por el exit del subshell, así que sirve igual si la guarda usa return o exit.
correr_normalizacion() {
  local dir="$1" dueno="$2" bruto
  bruto=$(
    export PATH="$STUB_DIR:$PATH"
    normalizar_duenos "$dir" "$dueno" 2>&1
    rc=$?
    printf '__CAMBIADOS=%s\n' "${DUENOS_CAMBIADOS-<unset>}"
    exit "$rc"
  )
  RC=$?
  CAMBIADOS=$(sed -n 's/^__CAMBIADOS=//p' <<<"$bruto")
  SALIDA=$(grep -vE '^__CAMBIADOS=' <<<"$bruto")
}

chown_invocaciones() { cat "$CHOWN_LOG" 2>/dev/null || true; }

modos_de() { find "$1" -printf '%m %P\n' 2>/dev/null | sort; }

# === tests ===

TestInstall_Normaliza_DejaUnicoDueno() {
  cargar_lib || { fail "la lib no está: fase roja"; return; }
  preparar dueno-unico
  local repo="$REPO_PRUEBA"
  crear_arbol "$repo"
  sembrar_duenos "$repo" "$DUENO_PREVIO"

  correr_normalizacion "$repo" "$DUENO_DESTINO"
  assert_eq "exit 0" "0" "$RC"

  local ruta ajenas=""
  while IFS= read -r ruta; do
    [[ "$(dueno_de "$ruta")" == "$DUENO_DESTINO" ]] || ajenas+="$ruta "
  done < <(find "$repo")
  assert_eq "ninguna ruta queda con otro dueño" "" "$ajenas"

  assert_contains "el chown es recursivo y apunta al árbol entero" "$repo" "$(chown_invocaciones)"
  assert_ne "reporta cuántas rutas cambiaron (no <unset>)" "<unset>" "$CAMBIADOS"
  if [[ "${CAMBIADOS:-0}" =~ ^[0-9]+$ ]] && (( CAMBIADOS > 0 )); then
    pass "el conteo sale de los cambios reales, no de cero fijo"
  else
    fail "DUENOS_CAMBIADOS='$CAMBIADOS' con todo el árbol en manos de $DUENO_PREVIO"
  fi
}

TestInstall_Normaliza_EsIdempotente() {
  cargar_lib || { fail "la lib no está: fase roja"; return; }
  preparar idempotente
  local repo="$REPO_PRUEBA"
  crear_arbol "$repo"
  sembrar_duenos "$repo" "$DUENO_PREVIO"

  correr_normalizacion "$repo" "$DUENO_DESTINO"
  assert_eq "1ra corrida: exit 0" "0" "$RC"
  local cambiados_1="$CAMBIADOS"

  correr_normalizacion "$repo" "$DUENO_DESTINO"
  assert_eq "2da corrida: exit 0" "0" "$RC"
  # el modo de falla que esto ataca: contar TODO lo recorrido en vez de lo que cambió.
  # Un install que reporta 1940 rutas "normalizadas" en cada redeploy entrena a que el
  # número no signifique nada, y el día que sí cambian dueños nadie lo mira.
  assert_eq "2da corrida: 0 cambios" "0" "$CAMBIADOS"
  assert_ne "el conteo distingue las dos corridas" "$cambiados_1" "$CAMBIADOS"
}

TestInstall_InstallDirVacio_AbortaSinChown() {
  cargar_lib || { fail "la lib no está: fase roja"; return; }
  preparar dir-vacio

  correr_normalizacion "" "$DUENO_DESTINO"
  assert_ne "INSTALL_DIR='' -> no sale 0" "0" "$RC"
  assert_eq "INSTALL_DIR='' -> chown NO se invocó" "" "$(chown_invocaciones)"
  # el mensaje tiene que nombrar el VACÍO, no solo abortar. Hay tres guardas encadenadas
  # (-z, no-absoluta, no-es-dir) y la cadena entera atrapa el vacío: con un assert que
  # acepta cualquier mensaje, borrar la primera guarda deja el test EN VERDE porque la
  # segunda dispara. Medido en la tabla de sabotajes de t9, y es el patrón registrado de
  # este proyecto: la defensa en profundidad enmascarando el test del fix primario.
  local minuscula="${SALIDA,,}"
  if [[ "$minuscula" == *vac* ]]; then
    pass "INSTALL_DIR='' -> el mensaje nombra el vacío, no otro motivo"
  else
    fail "INSTALL_DIR='' -> abortó por otra guarda, no por la del vacío: '$SALIDA'"
  fi

  # "/" viaja con el vacío por la misma razón: los dos son el modo de falla irreversible
  # de un recursivo, y el vacío suele degradar justo en "/" al concatenar una subruta
  : > "$CHOWN_LOG"
  correr_normalizacion "/" "$DUENO_DESTINO"
  assert_ne "INSTALL_DIR='/' -> no sale 0" "0" "$RC"
  assert_eq "INSTALL_DIR='/' -> chown NO se invocó" "" "$(chown_invocaciones)"
}

TestInstall_InstallDirRelativo_AbortaSinChown() {
  cargar_lib || { fail "la lib no está: fase roja"; return; }
  preparar dir-relativo

  # una ruta relativa se resuelve contra el cwd del que llama, que en install.sh cambia
  # entre steps: el mismo argumento apunta a otro lado según desde dónde se invoque
  correr_normalizacion "../x" "$DUENO_DESTINO"
  assert_ne "INSTALL_DIR='../x' -> no sale 0" "0" "$RC"
  assert_eq "INSTALL_DIR='../x' -> chown NO se invocó" "" "$(chown_invocaciones)"
  # el || con *"../x"* que había acá hacía que el assert no discriminara: "../x" no existe,
  # así que la guarda de "no es directorio" aborta igual y su mensaje también nombra la
  # ruta. Con eso, borrar la guarda de ruta absoluta dejaba el test en verde
  local minuscula="${SALIDA,,}"
  if [[ "$minuscula" == *absolut* ]]; then
    pass "el mensaje dice que la ruta tiene que ser absoluta"
  else
    fail "abortó por otra guarda, no por la de ruta absoluta: '$SALIDA'"
  fi
}

TestInstall_Normaliza_NoCambiaModos() {
  cargar_lib || { fail "la lib no está: fase roja"; return; }
  preparar modos
  local repo="$REPO_PRUEBA"
  crear_arbol "$repo"
  sembrar_duenos "$repo" "$DUENO_PREVIO"
  local antes; antes=$(modos_de "$repo")

  correr_normalizacion "$repo" "$DUENO_DESTINO"
  assert_eq "exit 0" "0" "$RC"

  # el chown es real en el VPS, así que los modos se miden de verdad: chmod NO está
  # stubeado, se mide el efecto y no la llamada
  assert_eq "ningún modo del árbol cambió" "$antes" "$(modos_de "$repo")"
  assert_eq ".env sigue en 600" "600" "$(stat -c '%a' "$repo/.env")"
  assert_eq ".git/config sigue en 600" "600" "$(stat -c '%a' "$repo/.git/config")"

  # lo que de verdad tira el stack: prometheus (nobody), loki (10001) y grafana (472) leen
  # sus configs montadas read-only por el bit de OTROS. Un chmod en masa a 640 o 600 las
  # deja sin lectura y el monitoring se cae con el deploy en verde.
  local ruta sin_lectura=""
  for ruta in "$repo/docker-compose.yml" \
              "$repo/services/monitoring/prometheus.yml" \
              "$repo/services/monitoring/loki-config.yml"; do
    [[ "$(stat -c '%a' "$ruta")" == "644" ]] || sin_lectura+="$ruta($(stat -c '%a' "$ruta")) "
  done
  assert_eq "las configs montadas siguen en 644 (otros conservan lectura)" "" "$sin_lectura"
  assert_eq "el script sigue ejecutable" "755" "$(stat -c '%a' "$repo/scripts/deploy.sh")"
}

# El MUST-3 exige `find $INSTALL_DIR ! -user <dueño> | wc -l` == 0, y en el VPS da 9. Ocho
# de esos nueve son symlinks: `chown` sin -h sigue el enlace y le cambia el dueño al TARGET,
# así que el enlace conserva el suyo POR SIEMPRE y el criterio es inalcanzable por
# construcción. Medido el 2026-08-09 en producción: services/domain-mcp/.env quedó root:root
# mientras /opt/services/.env quedaba sysadmin:sysadmin.
#
# Y ese mismo seguimiento abre un agujero peor: services/certs es un symlink a ../certs, o
# sea que el chown lo atraviesa y alcanza el directorio que el -prune excluye. Que postgres
# sobreviviera fue suerte —tocó el dir, no server.key— y no diseño. -h cierra los dos.
TestInstall_Normaliza_NoSigueSymlinks() {
  cargar_lib || { fail "la lib no está: fase roja"; return; }
  preparar symlinks
  local repo="$REPO_PRUEBA"
  crear_arbol_con_symlinks "$repo"
  sembrar_duenos "$repo" "$DUENO_PREVIO"

  correr_normalizacion "$repo" "$DUENO_DESTINO"
  assert_eq "exit 0" "0" "$RC"

  local enlace ajenos=""
  for enlace in "$repo/services/.env" "$repo/services/domain-mcp/.env" "$repo/services/certs"; do
    [[ "$(dueno_de "$enlace")" == "$DUENO_DESTINO" ]] || ajenos+="$enlace($(dueno_de "$enlace")) "
  done
  assert_eq "los symlinks cambian de dueño ellos mismos" "" "$ajenos"

  # los asserts de arriba y de abajo miden el EFECTO, que es el criterio de esta suite.
  # Este mide la FORMA a propósito y es el único así: sirve para distinguir "la lib no
  # dereferencia" de "el stub dejó de modelar la dereferencia", que darían lo mismo en el
  # efecto y son bugs opuestos. El regex acepta -h suelto, agrupado (-ch, -hc) o largo.
  local invocaciones; invocaciones="$(chown_invocaciones)"
  if grep -qE '(^|[[:space:]])(--no-dereference|-[a-gi-zA-Z]*h[a-zA-Z]*)([[:space:]]|$)' \
       <<<"$invocaciones"; then
    pass "el chown pide explícitamente no dereferenciar"
  else
    fail "ninguna invocación pide -h/--no-dereference: $invocaciones"
  fi

  # certs/ está fuera del recorrido de find; si el chown atraviesa services/certs, el
  # -prune deja de valer y el paso vuelve a poder tumbar postgres
  assert_eq "el -prune de certs/ no se evade por el symlink" \
    "$DUENO_PREVIO" "$(dueno_de "$repo/certs")"
  assert_eq "server.key nunca se toca" \
    "$DUENO_PREVIO" "$(dueno_de "$repo/certs/postgres/server.key")"

  # el .env real es el target de tres enlaces: si el chown lo siguiera, cambiaría de dueño
  # tantas veces como enlaces haya y el conteo de la idempotencia mentiría
  assert_eq ".env sigue en 600 tras normalizar" "600" "$(stat -c '%a' "$repo/.env")"
}

# El criterio del MUST-3, corregido el 2026-08-09: no "un solo dueño" —que es
# inalcanzable— sino "ningún resto de root". La diferencia importa porque el árbol tiene
# legítimamente un archivo de otro dueño: certs/postgres/server.key pertenece al uid 999
# de DENTRO del container de postgres, un chown del host no puede reconstruirlo, y
# tocarlo tumba producción. El criterio viejo contaba ese archivo como una falla.
TestInstall_Normaliza_NoDejaRestosDeRoot() {
  cargar_lib || { fail "la lib no está: fase roja"; return; }
  preparar restos-de-root
  local repo="$REPO_PRUEBA"
  crear_arbol_con_symlinks "$repo"
  sembrar_duenos "$repo" "$DUENO_PREVIO"

  # estado del VPS medido: certs/ ya es de sysadmin y solo server.key conserva el uid
  # interno del container. Sembrarlo todo de root haría fallar el criterio por una
  # condición que en producción no existe.
  local ruta
  while IFS= read -r ruta; do
    printf '%s\t%s\n' "$ruta" "$DUENO_DESTINO" >> "$REGISTRO"
  done < <(find "$repo/certs")
  printf '%s\t%s\n' "$repo/certs/postgres/server.key" "$UID_CONTAINER" >> "$REGISTRO"

  correr_normalizacion "$repo" "$DUENO_DESTINO"
  assert_eq "exit 0" "0" "$RC"

  local restos="" dueno
  while IFS= read -r ruta; do
    dueno="$(dueno_de "$ruta")"
    [[ "$dueno" == "$DUENO_DESTINO" || "$dueno" == "$UID_CONTAINER" ]] && continue
    restos+="$ruta($dueno) "
  done < <(find "$repo")
  assert_eq "no queda ningún resto de $DUENO_PREVIO en el árbol" "" "$restos"

  # el complemento del criterio: el uid del container sobrevive. Sin este assert, un
  # chown que arrasara con certs/ pasaría el test de arriba y tumbaría postgres
  assert_eq "el uid interno del container sigue siendo el dueño de server.key" \
    "$UID_CONTAINER" "$(dueno_de "$repo/certs/postgres/server.key")"
}

# === runner ===

TESTS=(
  TestInstall_Normaliza_DejaUnicoDueno
  TestInstall_Normaliza_EsIdempotente
  TestInstall_InstallDirVacio_AbortaSinChown
  TestInstall_InstallDirRelativo_AbortaSinChown
  TestInstall_Normaliza_NoCambiaModos
  TestInstall_Normaliza_NoSigueSymlinks
  TestInstall_Normaliza_NoDejaRestosDeRoot
)

for test_actual in "${TESTS[@]}"; do
  echo "--- $test_actual"
  test_ok=1
  "$test_actual"
  if (( test_ok )); then
    echo "OK   $test_actual"
  else
    echo "FALLA $test_actual"
    tests_fallados=$((tests_fallados + 1))
  fi
done

if (( tests_fallados > 0 )); then
  echo "RED — $tests_fallados de ${#TESTS[@]} tests fallaron ($asserts_fallados asserts)"
  exit 1
fi

echo "GREEN — los ${#TESTS[@]} tests pasaron"
exit 0
