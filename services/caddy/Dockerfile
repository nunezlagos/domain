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
# ESTA IMAGEN YA ESTA EN USO (DOMAINSERV-124, promovida 2026-07-25). El compose
# apunta aca y `make build` la construye; install.sh la buildea y corre
# `caddy validate` ANTES del `make down`, para que una config mala aborte el deploy
# con el stack anterior todavia sirviendo.
#
# Verificado en el build del 2026-07-25 con caddy v2.11.4:
#   crowdsec · http.handlers.crowdsec · admin.api.crowdsec · http.handlers.rate_limit
#
# El bouncer se probo end-to-end contra una LAPI real, no solo validando sintaxis:
# se registro via BOUNCER_KEY_caddy, autentico (last_pull poblado, caddy-cs-bouncer
# v0.13.1) y con una decision activa devolvio 403 en TODAS las rutas del Caddyfile
# real —incluidas las de @scanners/abort—, volviendo al comportamiento normal al
# borrar la decision.
#
# Si hace falta re-verificar los modulos:
#   docker run --rm domain-caddy:plugins caddy list-modules | grep -E "rate_limit|crowdsec"

FROM caddy:2-builder AS builder

RUN xcaddy build \
    --with github.com/mholt/caddy-ratelimit \
    --with github.com/hslatman/caddy-crowdsec-bouncer/http

FROM caddy:2-alpine

COPY --from=builder /usr/bin/caddy /usr/bin/caddy
