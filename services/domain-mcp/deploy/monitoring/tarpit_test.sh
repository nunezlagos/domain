#!/bin/sh
# DOMAINSERV-130: verificacion del tarpit.
#
# Lo que se mide NO es "el tarpit arranca" sino la unica propiedad que lo
# justifica: RETIENE. Para eso hay tres sujetos corriendo a la vez:
#
#   tarpit   -> tarpit.sh, gotea un byte cada TP_DELAY y nunca cierra
#   normal   -> un servicio que responde su banner y cierra (control positivo)
#   cerrado  -> un puerto sin listener              (control negativo)
#
# El control positivo prueba que el metodo de medicion detecta un servicio
# rapido; sin el, un tarpit roto que colgara al cliente por un bug de red daria
# "verde" igual. El negativo prueba que un puerto muerto no retiene nada.
#
# Corre 100% local: no necesita el VPS ni docker.

set -eu

DIR=$(CDPATH= cd -- "$(dirname -- "$0")" && pwd)
TARPIT_SH="${DIR}/tarpit.sh"
SNIPPET="${DIR}/tarpit-compose-snippet.yml"

# puertos altos y raros a proposito: este test corre en la maquina del dev y no
# debe pisar nada. TP_* son overridables para poder correrlo en paralelo
PUERTO_TARPIT="${TP_PUERTO_TARPIT:-19241}"
PUERTO_NORMAL="${TP_PUERTO_NORMAL:-19242}"
PUERTO_CERRADO="${TP_PUERTO_CERRADO:-19243}"

# cuanto aguanta el cliente antes de rendirse. La retencion real es infinita:
# esto es lo que estamos dispuestos a esperar en un test
ESPERA=4
# goteo rapido para no esperar 30s por un byte
DELAY=1

fallas=0
LOG_TARPIT=$(mktemp)
tmp_borrar="${LOG_TARPIT}"

ok() { printf 'ok   %s\n' "$1"; }
fail() { printf 'FAIL %s\n' "$1"; fallas=$((fallas + 1)); }

limpiar() {
  pkill -f "TCP4-LISTEN:${PUERTO_TARPIT}" 2>/dev/null || true
  pkill -f "TCP4-LISTEN:${PUERTO_NORMAL}" 2>/dev/null || true
  rm -f "${tmp_borrar}"
}
trap limpiar EXIT INT TERM

# mide cuanto retiene el peer y cuantos bytes manda. socat como cliente (no nc)
# porque ya es dependencia del tarpit y su exit code distingue rechazo de espera
medir() {
  puerto="$1"
  salida=$(mktemp)
  ini=$(date +%s%N)
  timeout "${ESPERA}" socat -u "TCP4:127.0.0.1:${puerto}" - >"${salida}" 2>/dev/null || true
  fin=$(date +%s%N)
  ms=$(((fin - ini) / 1000000))
  bytes=$(wc -c <"${salida}" | tr -d ' ')
  rm -f "${salida}"
  echo "${ms} ${bytes}"
}

# --- 1. el guard de puertos ocupados ------------------------------------------
# el modo de falla que este check existe para atrapar es el peor de todos:
# bindear el 22 deja al dueño sin SSH al VPS
#
# Con `timeout` y exigiendo exit 1 exacto: sin el timeout, un guard roto BINDEA y
# se queda escuchando para siempre, y el test cuelga en vez de fallar. Exigir
# exactamente 1 tambien evita el falso verde de un puerto privilegiado que falla
# por "permission denied" en lugar de por el guard.
test_guard_puertos_ocupados() {
  for prohibido in 22 53 80 443 3000 6300 6303 3389 31337; do
    TARPIT_PUERTO="${prohibido}" timeout 2 sh "${TARPIT_SH}" >/dev/null 2>&1 && rc=0 || rc=$?
    if [ "${rc}" -ne 1 ]; then
      fail "guard: puerto ocupado ${prohibido} no rechazado (exit ${rc}, esperado 1)"
      pkill -f "TCP4-LISTEN:${prohibido}" 2>/dev/null || true
      return
    fi
  done
  ok "guard: rechaza 22/53/80/443/3000/6300-6303 y los del honeypot"
}

# el puerto que realmente vamos a usar en prod tiene que pasar el guard
test_puerto_default_no_colisiona() {
  configurado=$(grep -oE 'TARPIT_PUERTO:-[0-9]+' "${TARPIT_SH}" | cut -d- -f2)
  [ -n "${configurado}" ] || { fail "no se pudo leer el puerto default"; return; }
  ocupados="22 53 80 443 3000 6300 6301 6302 6303 23 445 1433 3389 6379 9200 11211 27017 2375 4444 31337 5555 1099 50050"
  for o in ${ocupados}; do
    [ "${configurado}" = "${o}" ] && { fail "puerto default ${configurado} colisiona"; return; }
  done
  # el snippet de compose tiene que declarar el MISMO puerto que el default
  grep -q "TARPIT_PUERTO: \"${configurado}\"" "${SNIPPET}" ||
    { fail "el snippet no declara TARPIT_PUERTO ${configurado}"; return; }
  ok "puerto ${configurado} libre y coherente entre script y snippet"
}

# --- 2. retencion medida ------------------------------------------------------
test_retencion() {
  set -- $(medir "${PUERTO_NORMAL}")
  ms_normal="$1"
  set -- $(medir "${PUERTO_TARPIT}")
  ms_tarpit="$1"
  bytes_tarpit="$2"
  set -- $(medir "${PUERTO_CERRADO}")
  ms_cerrado="$1"

  printf '     medido: normal=%sms  tarpit=%sms  cerrado=%sms  bytes_goteados=%s\n' \
    "${ms_normal}" "${ms_tarpit}" "${ms_cerrado}" "${bytes_tarpit}"

  # control positivo: un servicio real cierra rapido
  if [ "${ms_normal}" -lt 1000 ]; then
    ok "control positivo: servicio normal cerro en ${ms_normal}ms"
  else
    fail "control positivo: servicio normal tardo ${ms_normal}ms (>=1000)"
  fi

  # el tarpit tiene que agotar la paciencia del cliente, no cerrar
  if [ "${ms_tarpit}" -ge $((ESPERA * 1000 - 200)) ]; then
    ok "tarpit retuvo ${ms_tarpit}ms (el cliente se rindio, el tarpit no)"
  else
    fail "tarpit solto la conexion a los ${ms_tarpit}ms"
  fi

  # lo que hace que la retencion sea ILIMITADA es el goteo: cada byte resetea el
  # timeout de lectura del peer. Sin bytes, un escaner cierra por inactividad
  if [ "${bytes_tarpit}" -gt 0 ]; then
    ok "goteo activo: ${bytes_tarpit} bytes en ${ESPERA}s"
  else
    fail "el tarpit no mando un solo byte: aceptar y callarse no retiene"
  fi

  if [ "${ms_normal}" -gt 0 ] && [ $((ms_tarpit / ms_normal)) -lt 5 ]; then
    fail "ratio tarpit/normal = $((ms_tarpit / ms_normal))x (<5x)"
  else
    ok "ratio tarpit/normal >= 5x"
  fi

  # control negativo: sin listener no hay retencion posible
  if [ "${ms_cerrado}" -lt 500 ]; then
    ok "control negativo: puerto cerrado rechazo en ${ms_cerrado}ms"
  else
    fail "control negativo: puerto cerrado retuvo ${ms_cerrado}ms"
  fi
}

# el byte goteado no puede ser \n ni \r: cualquiera de los dos termina la linea
# del banner y el cliente sigue de largo
test_goteo_sin_fin_de_linea() {
  salida=$(mktemp)
  timeout 2 socat -u "TCP4:127.0.0.1:${PUERTO_TARPIT}" - >"${salida}" 2>/dev/null || true
  if [ "$(wc -l <"${salida}" | tr -d ' ')" -eq 0 ] && [ -s "${salida}" ]; then
    ok "goteo sin CR/LF: el banner nunca termina"
  else
    fail "el goteo contiene fin de linea o esta vacio"
  fi
  rm -f "${salida}"
}

# --- 3. la IP de origen queda logueada como la espera CrowdSec ----------------
# el grok es el de crowdsec-parser-honeypot.yaml: si esto matchea, el parser
# existente banea los hits del tarpit sin escribir un parser nuevo
test_log_con_ip_de_origen() {
  patron='accepting connection from AF=2 127\.0\.0\.1:[0-9]+ on AF=2 127\.0\.0\.1:'"${PUERTO_TARPIT}"
  if grep -qE "${patron}" "${LOG_TARPIT}"; then
    ok "log con IP de origen parseable por domain/honeypot-logs"
  else
    fail "el log no trae la IP de origen en el formato del parser del honeypot"
    grep -c . "${LOG_TARPIT}" >/dev/null 2>&1 && tail -3 "${LOG_TARPIT}"
  fi
}

# --- 4. el container del snippet esta endurecido ------------------------------
test_snippet_endurecido() {
  for clave in 'read_only: true' 'cap_drop' 'no-new-privileges:true' 'mem_limit' 'network_mode: host'; do
    grep -q "${clave}" "${SNIPPET}" || { fail "snippet sin '${clave}'"; return; }
  done
  # 22 no puede aparecer como puerto en el snippet ni por accidente
  if grep -qE '^\s*-?\s*"?(22|0\.0\.0\.0:22)"?\s*:' "${SNIPPET}"; then
    fail "el snippet publica el puerto 22"
    return
  fi
  ok "snippet: read_only + cap_drop + no-new-privileges + mem_limit + host net"
}

# --- arranque de los sujetos --------------------------------------------------
command -v socat >/dev/null || { echo "hace falta socat para correr este test" >&2; exit 1; }
# sin esto el check del guard pasaria en verde por el motivo equivocado: un
# tarpit.sh inexistente tambien "falla al bindear" el 22
for requerido in "${TARPIT_SH}" "${SNIPPET}"; do
  [ -f "${requerido}" ] || { echo "falta ${requerido}" >&2; exit 1; }
done

TARPIT_PUERTO="${PUERTO_TARPIT}" TARPIT_DELAY="${DELAY}" TARPIT_MAX_CONEXIONES=8 \
  sh "${TARPIT_SH}" >"${LOG_TARPIT}" 2>&1 &

# control positivo: banner SSH plausible y cierre inmediato, como un sshd real
socat "TCP4-LISTEN:${PUERTO_NORMAL},fork,reuseaddr" \
  SYSTEM:'printf "SSH-2.0-OpenSSH_9.6\r\n"' >/dev/null 2>&1 &

# los listen tardan unos ms en estar arriba: esperar por evidencia, no por sleep fijo
i=0
while [ "${i}" -lt 100 ]; do
  grep -q listening "${LOG_TARPIT}" 2>/dev/null && break
  sleep 0.1
  i=$((i + 1))
done

echo "== tarpit: retencion medida =="
test_guard_puertos_ocupados
test_puerto_default_no_colisiona
test_retencion
test_goteo_sin_fin_de_linea
test_log_con_ip_de_origen
test_snippet_endurecido

if [ "${fallas}" -eq 0 ]; then
  echo "TODO OK"
  exit 0
fi
echo "${fallas} fallas"
exit 1
