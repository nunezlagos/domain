#!/bin/bash
set -euo pipefail

# domain backup script — pg_dump → gzip → B2 (S3-compatible)
# Env vars:
#   DOMAIN_DATABASE_URL      (required)
#   BACKUP_B2_BUCKET         (required)
#   BACKUP_B2_ENDPOINT       (required)
#   BACKUP_B2_KEY_ID         (required)
#   BACKUP_B2_APP_KEY        (required)
#   BACKUP_RETENTION_DAYS    (default: 30)
#   BACKUP_ADMIN_EMAIL       (optional, for notifications)

DATE=$(date -u +%Y%m%d)
BACKUP_FILE="/tmp/db-${DATE}.dump"
COMPRESSED="${BACKUP_FILE}.gz"
BUCKET="${BACKUP_B2_BUCKET:?BACKUP_B2_BUCKET required}"
ENDPOINT="${BACKUP_B2_ENDPOINT:?BACKUP_B2_ENDPOINT required}"
RETENTION_DAYS="${BACKUP_RETENTION_DAYS:-30}"
LAST_SUCCESS="s3://${BUCKET}/state/last-success.txt"

log() { echo "[$(date -u +%Y-%m-%dT%H:%M:%SZ)] $*"; }
error() { log "ERROR: $*" >&2; }

cleanup() {
    rm -f "${BACKUP_FILE}" "${COMPRESSED}"
}
trap cleanup EXIT

# 1. pg_dump
log "starting pg_dump..."
pg_dump --format=custom --no-owner --file="${BACKUP_FILE}" "${DOMAIN_DATABASE_URL:?DOMAIN_DATABASE_URL required}"
log "pg_dump completed: $(stat -c%s "${BACKUP_FILE}") bytes"

# 2. gzip
log "compressing..."
gzip "${BACKUP_FILE}"
log "compressed: $(stat -c%s "${COMPRESSED}") bytes"

# 3. Upload to B2
log "uploading to B2..."
aws s3 cp "${COMPRESSED}" "s3://${BUCKET}/db/${DATE}.dump.gz" --endpoint "${ENDPOINT}"
log "upload completed"

# 4. Verify size
LOCAL_SIZE=$(stat -c%s "${COMPRESSED}")
REMOTE_SIZE=$(aws s3api head-object --bucket "${BUCKET}" --key "db/${DATE}.dump.gz" --endpoint "${ENDPOINT}" --query ContentLength --output text 2>/dev/null || echo "0")
if [ "${LOCAL_SIZE}" != "${REMOTE_SIZE}" ]; then
    error "size mismatch: local=${LOCAL_SIZE} remote=${REMOTE_SIZE}"
    exit 1
fi
log "size verified: ${LOCAL_SIZE} bytes"

# 5. Retention: delete files older than RETENTION_DAYS
log "running retention (${RETENTION_DAYS} days)..."
aws s3 ls "s3://${BUCKET}/db/" --endpoint "${ENDPOINT}" 2>/dev/null | while read -r line; do
    FILE_DATE=$(echo "${line}" | awk '{print $4}' | grep -oE '[0-9]{8}')
    if [ -n "${FILE_DATE}" ]; then
        AGE_DAYS=$(( ($(date +%s) - $(date -d "${FILE_DATE}" +%s)) / 86400 ))
        if [ "${AGE_DAYS}" -gt "${RETENTION_DAYS}" ]; then
            aws s3 rm "s3://${BUCKET}/db/${FILE_DATE}.dump.gz" --endpoint "${ENDPOINT}" 2>/dev/null || true
            log "deleted old backup: ${FILE_DATE}.dump.gz"
        fi
    fi
done

# 6. Write success marker
echo "${DATE}" | aws s3 cp - "${LAST_SUCCESS}" --endpoint "${ENDPOINT}" 2>/dev/null || true

log "backup completed successfully"
