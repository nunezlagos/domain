#!/usr/bin/env bash
# Verificación del guard A2 en un container DESCARTABLE, con árbol de sacrificio.
#
# Por qué existe: hasta ahora el guard se probaba pasándole el comando como payload
# al hook y leyendo su decisión. Eso no ejecuta nada, pero verifica solo la DECISIÓN.
# Acá el comando SE EJECUTA de verdad contra un repo de sacrificio dentro del
# container, así que se verifica el EFECTO: si el guard falla, se ve el destrozo en
# un árbol que no le importa a nadie.
#
# El repo del host se monta :ro y el container va con --network none.
set -euo pipefail

REPO="${1:-/home/nunezlagos/Proyectos/domain-services}"
IMG=guard-sandbox:local

cd "$(dirname "$0")"

docker build -q -t "$IMG" -f Dockerfile . >/dev/null

# --rm: el container se destruye al terminar. :ro sobre el repo real: ni el
# container ni un bug del guard pueden tocar el working tree del host.
docker run --rm --network none \
  -v "$REPO/install-user/hooks:/hooks:ro" \
  "$IMG" bash /sandbox/casos.sh
