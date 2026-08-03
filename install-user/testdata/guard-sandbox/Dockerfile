FROM debian:bookworm-slim

# python3 porque el hook parsea el payload con un bloque python embebido;
# git porque el criterio de radio resuelve la raiz del repo con rev-parse
RUN apt-get update && apt-get install -y --no-install-recommends \
      bash git python3 jq ca-certificates \
 && rm -rf /var/lib/apt/lists/*

COPY casos.sh /sandbox/casos.sh
WORKDIR /sandbox
