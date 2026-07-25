# DOMAINSERV-130: imagen de Caddy con rate limiting y bouncer de CrowdSec.
#
# Caddy NO trae rate limiting en el core — verificado contra la doc oficial del
# Caddyfile. Es un plugin, y los plugins exigen recompilar con xcaddy. Lo mismo
# vale para el bouncer de CrowdSec que aplica los baneos en la capa HTTP.
#
# Los dos van en el MISMO build a proposito: si hay que salir de la imagen oficial,
# se paga el costo una sola vez.
#
# Paths de modulo verificados contra el go.mod de cada repo, no supuestos:
#   github.com/mholt/caddy-ratelimit             (Matt Holt, autor de Caddy)
#   github.com/hslatman/caddy-crowdsec-bouncer
#
# ESTA IMAGEN NO ESTA EN USO TODAVIA. El compose sigue apuntando a caddy:2-alpine.
# Cambiar la imagen del unico borde HTTP del stack es un movimiento riesgoso y se
# hace deliberadamente, no de arrastre:
#
#   1. Buildear y verificar:  docker build -t domain-caddy:plugins services/caddy
#   2. `caddy validate` con el Caddyfile real (los plugins agregan directivas; una
#      config valida para la imagen oficial puede no serlo para esta, y viceversa)
#   3. Recien entonces apuntar services/caddy/docker-compose.yml a esta imagen
#
# Verificacion critica antes de promoverla: que el binario traiga los modulos.
#   docker run --rm domain-caddy:plugins caddy list-modules | grep -E "rate_limit|crowdsec"

FROM caddy:2-builder AS builder

RUN xcaddy build \
    --with github.com/mholt/caddy-ratelimit \
    --with github.com/hslatman/caddy-crowdsec-bouncer/http

FROM caddy:2-alpine

COPY --from=builder /usr/bin/caddy /usr/bin/caddy
