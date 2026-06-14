# Design: issue-25.4-schema-drift

## Tool

**pg-schema-diff** (`stripe/pg-schema-diff`) o **apgdiff** — output SQL diff entre dos schemas.

## Cron Job

```yaml
apiVersion: batch/v1
kind: CronJob
metadata: { name: schema-drift-check }
spec:
  schedule: "0 6 * * *"
  jobTemplate:
    spec:
      template:
        spec:
          containers:
            - name: drift
              image: ghcr.io/domain/drift-tool:latest
              env:
                - { name: PROD_DB_URL, valueFrom: { secretKeyRef: ... } }
              command: ["/check-drift.sh"]
```

## Script

```bash
#!/bin/bash
set -e
# 1. spawn ephemeral pg
pg_ctl init -D /tmp/expected && pg_ctl start
# 2. apply all migrations
migrate -path /migrations -database "postgres://localhost:5432/expected" up
# 3. dump expected
pg_dump --schema-only --no-owner --no-privileges -d expected | sort > /tmp/expected.sql
# 4. dump actual
pg_dump --schema-only --no-owner --no-privileges "$PROD_DB_URL" | sort > /tmp/actual.sql
# 5. diff
diff -u /tmp/expected.sql /tmp/actual.sql > /tmp/drift.diff
if [ -s /tmp/drift.diff ]; then
  aws s3 cp /tmp/drift.diff s3://drift-reports/$(date +%F).diff
  curl -X POST $WEBHOOK_URL --data "drift detected"
  exit 1
fi
```

## Schema (storage de resultados)

```sql
CREATE TABLE schema_drift_checks (
  id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  checked_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  has_drift BOOLEAN NOT NULL,
  diff_summary TEXT,
  s3_full_diff_url TEXT,
  duration_seconds INT
);
```

## TDD plan

1. Drift simulated → detectado
2. No drift → ok
3. Migration dirty → detectado
4. Endpoint admin último resultado
