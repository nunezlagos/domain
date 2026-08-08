#!/usr/bin/env bash
# scripts/tests/test_auto_deploy_alerta.sh
#
# issue-57.2 / DOMAINSERV-258 — el auto-deploy falló en CADA ciclo del timer con exit 128
# (dubious ownership) y NADIE se enteró: el timer marca el ciclo fallido en el journal y
# ahí muere. Lo que se prueba acá es la capa que faltaba, la que avisa:
#
#   - la unidad declara OnFailure= (ADR-4: la alerta la dispara systemd, NO un trap del
#     script — el fallo real ocurre ANTES de que un trap llegue a existir)
#   - sin canal configurado, alertar no puede romper el ciclo
#   - la alerta NO filtra el topic de ntfy, que ES la credencial del canal
#
# Método: la unidad se lee del repo; el script de alerta se copia a una caja de arena con
# su propio .env y un `curl` de mentira en el PATH, así el test no depende del .env real
# del checkout ni le pega a ntfy de verdad.
#
# Uso: bash scripts/tests/test_auto_deploy_alerta.sh

set -uo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
REPO_ROOT="$(cd "$SCRIPT_DIR/../.." && pwd)"
UNIT="$REPO_ROOT/services/systemd/domain-auto-deploy.service"
ALERTA_REL="services/scripts/auto-deploy-alert.sh"
ALERTA="$REPO_ROOT/$ALERTA_REL"

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

assert_no_contains() {
  local label="$1" needle="$2" haystack="$3"
  if [[ "$haystack" != *"$needle"* ]]; then pass "$label"; else
    fail "$label — encontré '$needle' donde no debía, en: $(printf '%s' "$haystack" | grep -F "$needle" | tr '\n' '|')"
  fi
}

# systemd solo honra OnFailure= en [Unit]: puesto en [Service] lo ignora con un warning en
# el journal y la alerta jamás se dispara, sin ningún otro síntoma
seccion_de_directiva() {
  local clave="$1" archivo="$2"
  awk -v clave="$clave" '
    /^\[/     { seccion = $0; next }
    index($0, clave) == 1 { print seccion; exit }
  ' "$archivo"
}

# Caja de arena con el layout que el script espera (services/scripts + services/.env) y un
# curl que registra su invocación en vez de postear. $1 = contenido del .env.
# Imprime la ruta de la caja.
preparar_caja() {
  local env_contenido="$1"
  local caja="$WORK/caja-$RANDOM$RANDOM"

  mkdir -p "$caja/services/scripts" "$caja/bin"
  cp "$ALERTA" "$caja/services/scripts/"
  printf '%s\n' "$env_contenido" > "$caja/services/.env"

  # un argumento por línea: así el assert puede distinguir la URL del canal (donde el topic
  # es inevitable) del resto del comando (donde es una fuga)
  cat > "$caja/bin/curl" <<'STUB'
#!/usr/bin/env bash
for arg in "$@"; do printf '%s\n' "$arg"; done >> "$CURL_ARGS"
# el cuerpo puede viajar por stdin (curl -d @-); stdin siempre es un archivo regular acá,
# así que este cat corta en EOF y no cuelga el test
cat >> "$CURL_ARGS" 2>/dev/null
exit "${CURL_RC:-0}"
STUB
  chmod +x "$caja/bin/curl"

  printf '%s' "$caja"
}

# corre el script de alerta de la caja pasando el motivo por argumento Y por stdin: el
# contrato admite las dos formas y el test no debe casarse con una
correr_alerta() {
  local caja="$1" motivo="$2"
  printf '%s\n' "$motivo" > "$caja/motivo.txt"
  : > "$caja/curl-args.txt"
  PATH="$caja/bin:$PATH" CURL_ARGS="$caja/curl-args.txt" \
    bash "$caja/services/scripts/auto-deploy-alert.sh" "$motivo" \
    < "$caja/motivo.txt" 2>&1
}

alerta_disponible() {
  local label="$1"
  if [[ -f "$ALERTA" ]]; then return 0; fi
  fail "$label — $ALERTA_REL no existe todavia"
  return 1
}

# --- TestUnit_AutoDeploy_DeclaraOnFailure -------------------------------------------------
# Sin OnFailure=, un ciclo fallido queda solo en el journal del VPS: exactamente el modo de
# falla de DOMAINSERV-258, que corrió roto durante semanas sin que nadie lo viera.
test_unit_declara_onfailure() {
  local T="TestUnit_AutoDeploy_DeclaraOnFailure"

  if [[ ! -f "$UNIT" ]]; then
    fail "$T — falta $UNIT"
    return
  fi

  local linea destino
  linea=$(grep -E '^OnFailure=' "$UNIT" | head -1)
  destino="${linea#OnFailure=}"

  if [[ -z "$destino" ]]; then
    fail "$T — la unidad no declara OnFailure=: un ciclo fallido no avisa a nadie"
    return
  fi
  pass "$T — la unidad declara OnFailure=$destino"

  assert_eq "$T — OnFailure vive en [Unit] (en [Service] systemd lo ignora)" \
    "[Unit]" "$(seccion_de_directiva "OnFailure=" "$UNIT")"

  assert_contains "$T — OnFailure apunta a una unidad de servicio" ".service" "$destino"

  # la unidad de alerta tiene que existir en el repo: un OnFailure a una unidad inexistente
  # no rompe nada al instalar, solo no alerta nunca
  local nombre="${destino%% *}" archivo
  if [[ "$nombre" == *@*.service ]]; then
    # plantilla instanciada (foo@%n.service) -> el archivo del repo es foo@.service
    archivo="${nombre%%@*}@.service"
  else
    archivo="$nombre"
  fi
  if [[ -f "$REPO_ROOT/services/systemd/$archivo" ]]; then
    pass "$T — la unidad de alerta $archivo existe en services/systemd"
  else
    fail "$T — OnFailure apunta a '$nombre' pero services/systemd/$archivo no existe"
  fi
}

# --- TestAlerta_SinNtfyTopic_NoRompeElCiclo -----------------------------------------------
# La alerta es una capa de observabilidad: si su propio fallo tumba el ciclo, el remedio es
# peor que la enfermedad. Mismo degradado que services/scripts/healthcheck-alert.sh (|| true).
test_alerta_sin_topic_no_rompe() {
  local T="TestAlerta_SinNtfyTopic_NoRompeElCiclo"
  alerta_disponible "$T" || return

  local caja salida rc
  caja=$(preparar_caja "POSTGRES_PASSWORD=irrelevante")
  salida=$(correr_alerta "$caja" "auto-deploy fallo con exit 128"); rc=$?

  assert_eq "$T — sin NTFY_TOPIC la alerta sale 0" "0" "$rc"

  # sin canal, el journal es la única salida que queda: callarse deja el fallo invisible dos veces
  if [[ -n "$salida" ]]; then
    pass "$T — sin canal igual deja rastro en el journal"
  else
    fail "$T — sin canal no imprimió nada: el fallo queda invisible"
  fi

  assert_eq "$T — sin canal no intenta postear" "" "$(cat "$caja/curl-args.txt")"

  # el otro degradado: hay canal, pero ntfy o la red se cayeron. curl != 0 no puede propagarse
  local caja2 rc2
  caja2=$(preparar_caja "NTFY_TOPIC=tpc-de-prueba")
  CURL_RC=7 correr_alerta "$caja2" "auto-deploy fallo con exit 128" >/dev/null; rc2=$?
  assert_eq "$T — con curl fallando (rc=7) la alerta igual sale 0" "0" "$rc2"
}

# --- TestAlerta_NoFiltraElTopic -----------------------------------------------------------
# El topic ES la credencial del canal: quien lo lee puede publicar Y suscribirse. Meterlo en
# el cuerpo del mensaje lo reenvía al propio canal y lo deja en el journal en texto plano.
# La forma canónica del repo es POST a $SERVER/$TOPIC con el cuerpo en -d, igual que
# healthcheck-alert.sh: por eso el único lugar donde el topic puede aparecer es la URL.
test_alerta_no_filtra_el_topic() {
  local T="TestAlerta_NoFiltraElTopic"
  alerta_disponible "$T" || return

  local topic="tpc-secreto-9f3a1c" secreto="pg-clave-4d7b2e"
  local caja salida args
  caja=$(preparar_caja "NTFY_TOPIC=$topic
NTFY_SERVER=https://ntfy.example.invalid
NTFY_URL=https://ntfy.example.invalid
POSTGRES_PASSWORD=$secreto")

  salida=$(correr_alerta "$caja" "auto-deploy fallo con exit 128")
  args=$(cat "$caja/curl-args.txt")

  # sin esto el resto de los asserts pasan en verde con el script sin postear nada
  if [[ -n "$args" ]]; then
    pass "$T — con canal configurado sí postea (hay algo que auditar)"
  else
    fail "$T — con NTFY_TOPIC configurado no invocó curl: la alerta no llega. salida: $salida"
    return
  fi
  assert_contains "$T — el POST va a la URL del canal" "$topic" "$args"

  # todo lo que NO es la URL del endpoint: cuerpo, headers, flags
  local fuera_de_url=""
  while IFS= read -r arg; do
    [[ "$arg" == http://* || "$arg" == https://* ]] && continue
    fuera_de_url+="$arg"$'\n'
  done <<< "$args"

  assert_no_contains "$T — el topic no viaja fuera de la URL (cuerpo, headers ni flags)" \
    "$topic" "$fuera_de_url"
  assert_no_contains "$T — la URL completa del canal no va en el cuerpo" \
    "ntfy.example.invalid/$topic" "$fuera_de_url"
  assert_no_contains "$T — el cuerpo no vuelca valores del .env" "$secreto" "$fuera_de_url"

  # el journal del VPS lo lee cualquiera con acceso a la máquina: ahí tampoco va la credencial
  assert_no_contains "$T — la salida al journal no imprime el topic" "$topic" "$salida"
  assert_no_contains "$T — la salida al journal no vuelca el .env" "$secreto" "$salida"
}

test_unit_declara_onfailure
test_alerta_sin_topic_no_rompe
test_alerta_no_filtra_el_topic

if (( failed > 0 )); then
  echo "RED — $failed tests fallaron"
  exit 1
fi

echo "GREEN — todos los tests pasaron"
exit 0
