#!/usr/bin/env bash
# scripts/redeploy.sh
#
# Redeploy manual del VPS: pull + build + restart + verify con rollback.
# Es lo mismo que dispara el workflow deploy.yml en push a main.
#
# Uso (en el VPS):
#   ./scripts/redeploy.sh             # redeploy real
#   ./scripts/redeploy.sh --dry-run   # muestra que haria, sin tocar nada

set -euo pipefail
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
exec "$SCRIPT_DIR/deploy.sh" "$@"