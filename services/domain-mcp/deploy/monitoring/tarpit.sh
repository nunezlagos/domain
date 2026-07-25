#!/bin/sh
# DOMAINSERV-130: tarpit. La pieza que le pone PRECIO al escaneo.
#
# El honeypot (honeypot.sh) detecta y CrowdSec banea, pero el escaner no paga
# nada: toca, cae en la lista, sigue con el proximo host en milisegundos. El
# tarpit invierte esa economia — acepta la conexion y NO LA SUELTA NUNCA.
#
# Mecanica, la misma de endlessh: un cliente SSH tiene que esperar el banner de
# identificacion (RFC 4253 permite lineas arbitrarias antes de "SSH-2.0-..."),
# asi que goteamos UN byte cada TARPIT_DELAY y jamas mandamos fin de linea. La
# linea del banner nunca termina y el peer espera indefinidamente.
#
# Por que gotear y no simplemente aceptar y callarse: cada byte RESETEA el
# timeout de lectura del peer. Aceptar en silencio retiene lo que dure su
# timeout de inactividad (segundos); goteando, la retencion no tiene techo.
#
# Por que socat y NO endlessh, decidido con evidencia:
#   - la imagen alpine/socat:1.8.0.0 ya esta pineada para el honeypot: cero
#     imagen nueva, cero superficie de supply chain adicional
#   - `socat -d -d` emite la MISMA linea que el honeypot ("accepting connection
#     from AF=2 <ip>:<puerto>"), asi que el parser domain/honeypot-logs ya
#     existente parsea estos hits sin escribir un parser nuevo. endlessh logea
#     "ACCEPT host=... port=..." y obligaria a otro parser
#   - `max-children` acota la concurrencia en la capa correcta (ver abajo)
# Contra, asumida a ojos abiertos: endlessh mantiene N conexiones en un solo
# proceso con epoll, memoria O(1). Aca cada conexion cuesta un socat hijo + un
# sh + un fork de `sleep` por intervalo. Con max-children y mem_limit el costo
# queda acotado y sigue siendo ordenes de magnitud menor que el del atacante,
# que gasta un slot de conexion y su paciencia.
#
# NUNCA el puerto 22: ahi vive el SSH real y bindearlo deja al dueño afuera del
# VPS. El guard de abajo lo hace imposible por configuracion, no por convencion.

set -eu

# 2222 por default: es el "SSH alternativo" que todo escaneo prueba despues del
# 22, asi que la carnada llega sola. Nadie en este host lo usa para nada.
PUERTO="${TARPIT_PUERTO:-2222}"

# 30s entre bytes. Mas alto retiene igual pero arriesga timeouts absolutos de
# escaneres agresivos; mas bajo gasta forks de `sleep` sin comprar retencion.
DELAY="${TARPIT_DELAY:-30}"

# techo de conexiones simultaneas. Un tarpit ACUMULA conexiones abiertas: sin
# techo, el DoS lo sufrimos nosotros. Al llegar al techo socat deja de aceptar y
# las conexiones nuevas quedan en el backlog del kernel — el atacante sigue
# esperando, y nosotros no gastamos ni un proceso mas.
MAX_CONEXIONES="${TARPIT_MAX_CONEXIONES:-64}"

# 22/53/80/443/3000/6300-6303 son del host; el resto son los señuelos de
# honeypot.sh. Colisionar con cualquiera rompe algo que YA funciona
PUERTOS_OCUPADOS="22 53 80 443 3000 6300 6301 6302 6303
23 445 1433 3389 6379 9200 11211 27017 2375 4444 31337 5555 1099 50050"

for ocupado in ${PUERTOS_OCUPADOS}; do
  if [ "${PUERTO}" = "${ocupado}" ]; then
    echo "tarpit: el puerto ${PUERTO} ya esta ocupado en este host, abortando" >&2
    exit 1
  fi
done

echo "tarpit escuchando en ${PUERTO}: 1 byte cada ${DELAY}s, max ${MAX_CONEXIONES} conexiones"

# SYSTEM y no un pipe: el goteo tiene que salir byte a byte y un pipe buffea por
# bloques (mismo motivo por el que honeypot.sh no filtra con `| grep`).
# Verificado dentro de alpine/socat:1.8.0.0 — el printf de busybox ash flushea en
# cada iteracion: 4 bytes en 4s con DELAY=1.
#
# `while true` y no `while :` a proposito: socat parte la direccion por ":" y con
# el dos puntos falla con "SYSTEM: wrong number of parameters".
#
# El byte es "X": cualquier caracter imprimible sirve, pero NUNCA \n ni \r —
# terminarian la linea del banner y el peer seguiria de largo.
#
# El comando va SIN comillas y SIN escapes, y eso NO es descuido: socat parsea la
# direccion antes de pasarla al shell, saca las comillas y convierte "\n" en un
# salto de linea real. Con `printf '%s\n' X` el hijo termina ejecutando
# `printf %s` y despues `X` como comando aparte — el goteo queda en cero y socat
# no reporta ningun error. Comprobado capturando el stderr del hijo.
#
# 2>&1: `-d -d` logea al stderr de socat y ahi va la IP de origen. Se manda a
# stdout para que Alloy lo levante igual que los hits del honeypot.
exec socat -d -d "TCP4-LISTEN:${PUERTO},fork,reuseaddr,max-children=${MAX_CONEXIONES}" \
  SYSTEM:"while true; do printf %s X; sleep ${DELAY}; done" 2>&1
