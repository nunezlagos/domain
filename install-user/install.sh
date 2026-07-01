#!/usr/bin/env bash
# install.sh — entry point simple del installer de user.
#
# Es un wrapper alrededor de bootstrap.sh (que detecta OS/arch, baja Go si falta,
# compila el binario domain-install, y lo ejecuta). Mismo comportamiento, nombre
# mas corto y consistente con services/install.sh del VPS.
#
# Uso:
#   ./install.sh                                    # interactive
#   ./install.sh --url http://vps --email u@x.cl     # flags
#   ./install.sh --uninstall                        # deshacer
#   ./install.sh --dry-run                          # solo detectar
#
# Idempotente: re-ejecuciones saltean Go install + build si el binario ya existe.

set -euo pipefail
exec "$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)/bootstrap.sh" "$@"