#!/bin/bash
# Certs TLS self-signed para Postgres y MinIO. --force para regenerar.
set -euo pipefail

SCRIPT_DIR="$( cd "$( dirname "${BASH_SOURCE[0]}" )" && pwd )"
ROOT_DIR="$( cd "$SCRIPT_DIR/.." && pwd )"
CERTS_DIR="$ROOT_DIR/certs"
PG_CERTS="$CERTS_DIR/postgres"
MINIO_CERTS="$CERTS_DIR/minio"

FORCE=0
[[ "${1:-}" == "--force" ]] && FORCE=1

[[ -f "$ROOT_DIR/.env" ]] && { set -a; source "$ROOT_DIR/.env"; set +a; }

if [[ -z "${VPS_PUBLIC_IP:-}" ]]; then
  VPS_PUBLIC_IP=$(curl -fsS --max-time 5 https://ifconfig.me 2>/dev/null || echo "")
  [[ -z "$VPS_PUBLIC_IP" ]] && { echo "ERROR: setea VPS_PUBLIC_IP en .env" >&2; exit 1; }
  echo "IP detectada: $VPS_PUBLIC_IP"
fi

mkdir -p "$PG_CERTS" "$MINIO_CERTS"

need_regen() {
  local crt="$1"
  [[ ! -f "$crt" ]] && return 0
  [[ $FORCE -eq 1 ]] && return 0
  openssl x509 -in "$crt" -noout -checkend $((30*24*3600)) >/dev/null 2>&1 || return 0
  return 1
}

gen_cert() {
  local key="$1" crt="$2" ou="$3"
  openssl req -x509 -newkey rsa:4096 -nodes \
    -keyout "$key" -out "$crt" -days 730 \
    -subj "/CN=$VPS_PUBLIC_IP/O=domain-services/OU=$ou" \
    -addext "subjectAltName=IP:$VPS_PUBLIC_IP,IP:127.0.0.1,DNS:localhost,DNS:$ou" \
    -addext "keyUsage=digitalSignature,keyEncipherment" \
    -addext "extendedKeyUsage=serverAuth" 2>/dev/null
  chmod 600 "$key"
  chmod 644 "$crt"
}

if need_regen "$PG_CERTS/server.crt"; then
  gen_cert "$PG_CERTS/server.key" "$PG_CERTS/server.crt" postgres
  cp "$PG_CERTS/server.crt" "$PG_CERTS/ca.crt"
  # PG container (pgvector/postgres oficial) corre como uid 999. El bind-mount
  # del cert al container requiere que sea legible por ese uid. Sin esto, PG
  # falla con "could not load private key file: Permission denied".
  chown -R 999:999 "$PG_CERTS" 2>/dev/null || true
  chmod 600 "$PG_CERTS/server.key"
  chmod 644 "$PG_CERTS/server.crt" "$PG_CERTS/ca.crt"
  echo "  → postgres (perms 999:999 aplicados)"
else
  echo "postgres: cert válido"
fi

if need_regen "$MINIO_CERTS/public.crt"; then
  gen_cert "$MINIO_CERTS/private.key" "$MINIO_CERTS/public.crt" minio
  echo "  → minio"
else
  echo "minio: cert válido"
fi

echo "Certs en $CERTS_DIR/ (validez 2 años)"
