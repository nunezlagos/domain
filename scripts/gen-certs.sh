#!/bin/bash
# ============================================================================
# gen-certs.sh — genera certs TLS self-signed para Postgres y MinIO.
#
# Output:
#   ../certs/postgres/{server.crt, server.key, ca.crt}
#   ../certs/minio/{public.crt, private.key}
#
# CN del cert = IP pública del VPS (detectada o pasada por env VPS_PUBLIC_IP).
# SubjectAltName incluye la IP + localhost para conexiones internas.
#
# Idempotente: si los certs ya existen y no expiraron, no los regenera.
# Forzar regen: gen-certs.sh --force
# ============================================================================
set -euo pipefail

SCRIPT_DIR="$( cd "$( dirname "${BASH_SOURCE[0]}" )" && pwd )"
ROOT_DIR="$( cd "$SCRIPT_DIR/.." && pwd )"
CERTS_DIR="$ROOT_DIR/certs"
PG_CERTS="$CERTS_DIR/postgres"
MINIO_CERTS="$CERTS_DIR/minio"

FORCE=0
if [[ "${1:-}" == "--force" ]]; then
  FORCE=1
fi

# Cargar .env si existe (para VPS_PUBLIC_IP)
if [[ -f "$ROOT_DIR/.env" ]]; then
  set -a
  # shellcheck disable=SC1091
  source "$ROOT_DIR/.env"
  set +a
fi

# Detectar IP pública si no viene seteada
if [[ -z "${VPS_PUBLIC_IP:-}" ]]; then
  echo "VPS_PUBLIC_IP vacío, detectando con ifconfig.me..."
  VPS_PUBLIC_IP=$(curl -fsS --max-time 5 https://ifconfig.me 2>/dev/null || echo "")
  if [[ -z "$VPS_PUBLIC_IP" ]]; then
    echo "ERROR: no pude detectar la IP pública." >&2
    echo "       Pasá VPS_PUBLIC_IP en .env o como env var." >&2
    exit 1
  fi
  echo "Detectada IP: $VPS_PUBLIC_IP"
fi

mkdir -p "$PG_CERTS" "$MINIO_CERTS"

# ----------------------------------------------------------------------------
# Postgres
# ----------------------------------------------------------------------------
need_regen() {
  local crt="$1"
  if [[ ! -f "$crt" ]]; then
    return 0
  fi
  if [[ $FORCE -eq 1 ]]; then
    return 0
  fi
  # Expira en <30d?
  if ! openssl x509 -in "$crt" -noout -checkend $((30*24*3600)) >/dev/null 2>&1; then
    return 0
  fi
  return 1
}

if need_regen "$PG_CERTS/server.crt"; then
  echo "Generando cert Postgres (CN=$VPS_PUBLIC_IP)..."
  openssl req -x509 -newkey rsa:4096 -nodes \
    -keyout "$PG_CERTS/server.key" \
    -out "$PG_CERTS/server.crt" \
    -days 730 \
    -subj "/CN=$VPS_PUBLIC_IP/O=domain-services/OU=postgres" \
    -addext "subjectAltName=IP:$VPS_PUBLIC_IP,IP:127.0.0.1,DNS:localhost,DNS:postgres" \
    -addext "keyUsage=digitalSignature,keyEncipherment" \
    -addext "extendedKeyUsage=serverAuth" 2>/dev/null
  chmod 600 "$PG_CERTS/server.key"
  chmod 644 "$PG_CERTS/server.crt"
  # PG corre como uid 999 dentro del container — el bind-mount necesita perms compatibles
  # En el host queda root:root con 644/600; PG las puede leer porque uid 999 las accede via volume.
  # Si falla "private key has world readable permissions", chown 999:999 antes de mount.
  cp "$PG_CERTS/server.crt" "$PG_CERTS/ca.crt"  # self-signed: cert == CA
  echo "  → $PG_CERTS/server.{crt,key} + ca.crt"
else
  echo "Cert Postgres válido, skip (forzar con --force)"
fi

# ----------------------------------------------------------------------------
# MinIO
# ----------------------------------------------------------------------------
if need_regen "$MINIO_CERTS/public.crt"; then
  echo "Generando cert MinIO (CN=$VPS_PUBLIC_IP)..."
  openssl req -x509 -newkey rsa:4096 -nodes \
    -keyout "$MINIO_CERTS/private.key" \
    -out "$MINIO_CERTS/public.crt" \
    -days 730 \
    -subj "/CN=$VPS_PUBLIC_IP/O=domain-services/OU=minio" \
    -addext "subjectAltName=IP:$VPS_PUBLIC_IP,IP:127.0.0.1,DNS:localhost,DNS:minio" \
    -addext "keyUsage=digitalSignature,keyEncipherment" \
    -addext "extendedKeyUsage=serverAuth" 2>/dev/null
  chmod 600 "$MINIO_CERTS/private.key"
  chmod 644 "$MINIO_CERTS/public.crt"
  echo "  → $MINIO_CERTS/public.crt + private.key"
else
  echo "Cert MinIO válido, skip (forzar con --force)"
fi

echo ""
echo "Certs generados en $CERTS_DIR/"
echo "Validez: 2 años. Re-correr este script para renovar."
