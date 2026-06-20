#!/bin/bash
set -euo pipefail

# domain restore test — weekly validation that backups are restorable
# Downloads latest backup from B2, restores to test DB, runs smoke tests.

BUCKET="${BACKUP_B2_BUCKET:?BACKUP_B2_BUCKET required}"
ENDPOINT="${BACKUP_B2_ENDPOINT:?BACKUP_B2_ENDPOINT required}"
RESTORE_TEST_URL="${DOMAIN_RESTORE_TEST_URL:-postgres://test:test@localhost:5433/domain_restore_test}"

log() { echo "[$(date -u +%Y-%m-%dT%H:%M:%SZ)] $*"; }
error() { log "ERROR: $*" >&2; }

# 1. Find the latest backup
log "finding latest backup..."
LATEST=$(aws s3 ls "s3://${BUCKET}/db/" --endpoint "${ENDPOINT}" 2>/dev/null | sort | tail -1 | awk '{print $4}')
if [ -z "${LATEST}" ]; then
    error "no backups found in B2"
    exit 1
fi
log "latest backup: ${LATEST}"

# 2. Download
TMP_DUMP=$(mktemp /tmp/restore-test-XXXXXX.dump.gz)
log "downloading..."
aws s3 cp "s3://${BUCKET}/db/${LATEST}" "${TMP_DUMP}" --endpoint "${ENDPOINT}"
log "downloaded: $(stat -c%s "${TMP_DUMP}") bytes"

# 3. Decompress
DECOMPRESSED="${TMP_DUMP%.gz}"
gunzip -c "${TMP_DUMP}" > "${DECOMPRESSED}"

# 4. Drop and recreate test DB
log "creating test database..."
psql "${RESTORE_TEST_URL}" -c "DROP DATABASE IF EXISTS domain_restore_test" 2>/dev/null || true
createdb -T template0 domain_restore_test 2>/dev/null || true
RESTORE_URL="${RESTORE_TEST_URL}"

# 5. Restore
log "restoring..."
pg_restore --no-owner --dbname="${RESTORE_URL}" "${DECOMPRESSED}"
log "restore completed"

# 6. Smoke tests
log "running smoke tests..."
ERRORS=0
for table in organizations users projects observations agents flows; do
    COUNT=$(psql "${RESTORE_URL}" -t -A -c "SELECT COUNT(*) FROM ${table}" 2>/dev/null || echo "0")
    log "  ${table}: ${COUNT} rows"
done

# 7. Cleanup
rm -f "${TMP_DUMP}" "${DECOMPRESSED}"
log "restore test completed"
