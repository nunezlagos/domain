#!/bin/sh
# REQ-74: genera /assets/env.js desde env vars al arrancar el container.
# El index.html ya incluye <script src="/assets/env.js"></script> (lo
# inyectamos vía un postprocess al COPY si hace falta — por ahora el
# Angular lee window.__DOMAIN_ENV__ que ESTE script setea).
#
# Variables soportadas:
#   API_URL          → backend a llamar. Vacío = mismo origen (Caddy proxy).
set -eu

OUT=/usr/share/nginx/html/assets/env.js
mkdir -p "$(dirname "$OUT")"

API_URL="${API_URL:-}"

cat > "$OUT" <<EOF
// Generado por docker-entrypoint.d/40-env-inject.sh al arrancar.
// NO editar manualmente; cambia el .env y recreá el container.
window.__DOMAIN_ENV__ = {
  API_URL: "${API_URL}"
};
EOF

# Asegurar que el index.html cargue env.js. Idempotente: solo inyecta
# si todavía no está.
INDEX=/usr/share/nginx/html/index.html
if ! grep -q '/assets/env.js' "$INDEX" 2>/dev/null; then
  sed -i 's#</head>#<script src="/assets/env.js"></script></head>#' "$INDEX"
fi
