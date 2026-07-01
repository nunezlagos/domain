#!/usr/bin/env bash
# scripts/daily-update.sh
#
# Wrapper invocado por el cron diario (configurado por services/install.sh
# STEP 9). Corre el mismo install.sh que el comando canonico del usuario.
# Es idempotente: re-ejecuciones solo actualizan el codigo.
#
# Path: /opt/services/scripts/daily-update.sh
# Cron: 0 3 * * *  (todos los dias a las 03:00, hora del VPS)
# Log:  /var/log/domain-update.log

exec /opt/services/services/install.sh