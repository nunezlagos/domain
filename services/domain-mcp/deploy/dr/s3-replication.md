# S3 Cross-Region Replication para Backups (HU-18.2)

Configura replicación automática de backups Postgres (pgBackRest, HU-18.1)
desde el bucket primario a un bucket DR en una región distinta. RPO objetivo:
< 15 min para snapshots, < 5 min para WAL streaming.

## Arquitectura

```
┌──────────────────┐         ┌──────────────────┐
│  AWS us-east-1   │  CRR    │  AWS eu-west-1   │
│  domain-backups  │ ──────▶ │ domain-backups-dr│
│  (primary)       │ replic. │  (replica)       │
└──────────────────┘         └──────────────────┘
       ▲                              │
       │                              │ restore (DR scenario)
       │                              ▼
   pgBackRest                  pgBackRest restore
   (HU-18.1)                   --repo=dr
```

## Buckets

| Bucket | Región | Versionado | Lifecycle | SSE | Replicación |
|---|---|---|---|---|---|
| `domain-backups` | us-east-1 | enabled | retain 90 días + Glacier 30+ | aws:kms | source |
| `domain-backups-dr` | eu-west-1 | enabled | retain 90 días + Glacier 30+ | aws:kms | destination |

Ambos requieren versionado ENABLED para que la replicación funcione.

## Configuración Terraform

```hcl
resource "aws_s3_bucket" "backups_primary" {
  bucket = "domain-backups"
  region = "us-east-1"
}

resource "aws_s3_bucket_versioning" "backups_primary" {
  bucket = aws_s3_bucket.backups_primary.id
  versioning_configuration { status = "Enabled" }
}

resource "aws_s3_bucket" "backups_dr" {
  bucket = "domain-backups-dr"
  region = "eu-west-1"
  provider = aws.eu
}

resource "aws_s3_bucket_versioning" "backups_dr" {
  bucket = aws_s3_bucket.backups_dr.id
  versioning_configuration { status = "Enabled" }
  provider = aws.eu
}

resource "aws_iam_role" "replication" {
  name = "domain-s3-replication"
  assume_role_policy = jsonencode({
    Version = "2012-10-17"
    Statement = [{
      Effect = "Allow"
      Principal = { Service = "s3.amazonaws.com" }
      Action = "sts:AssumeRole"
    }]
  })
}

resource "aws_iam_policy" "replication" {
  name = "domain-s3-replication"
  policy = jsonencode({
    Version = "2012-10-17"
    Statement = [
      {
        Effect = "Allow"
        Action = [
          "s3:GetReplicationConfiguration",
          "s3:ListBucket"
        ]
        Resource = aws_s3_bucket.backups_primary.arn
      },
      {
        Effect = "Allow"
        Action = [
          "s3:GetObjectVersionForReplication",
          "s3:GetObjectVersionAcl",
          "s3:GetObjectVersionTagging",
          "s3:GetObjectRetention",
          "s3:GetObjectLegalHold"
        ]
        Resource = "${aws_s3_bucket.backups_primary.arn}/*"
      },
      {
        Effect = "Allow"
        Action = [
          "s3:ReplicateObject",
          "s3:ReplicateDelete",
          "s3:ReplicateTags",
          "s3:GetObjectVersionTagging",
          "s3:ObjectOwnerOverrideToBucketOwner"
        ]
        Resource = "${aws_s3_bucket.backups_dr.arn}/*"
      }
    ]
  })
}

resource "aws_s3_bucket_replication_configuration" "backups" {
  bucket = aws_s3_bucket.backups_primary.id
  role   = aws_iam_role.replication.arn

  rule {
    id     = "replicate-all-encrypted"
    status = "Enabled"
    priority = 1

    filter {} # toda la jerarquía

    source_selection_criteria {
      sse_kms_encrypted_objects { status = "Enabled" }
    }

    destination {
      bucket        = aws_s3_bucket.backups_dr.arn
      storage_class = "STANDARD_IA"  # cheaper for DR usage
      encryption_configuration {
        replica_kms_key_id = aws_kms_key.dr.arn
      }
      replication_time {
        status = "Enabled"
        time   { minutes = 15 }
      }
      metrics {
        status = "Enabled"
        event_threshold { minutes = 15 }
      }
    }

    delete_marker_replication { status = "Enabled" }
  }
}
```

## Lifecycle

Ambos buckets aplican la misma lifecycle policy:

```hcl
resource "aws_s3_bucket_lifecycle_configuration" "backups" {
  for_each = toset(["primary", "dr"])
  bucket   = local.backup_buckets[each.key].id

  rule {
    id     = "tier-and-expire"
    status = "Enabled"

    filter {}

    transition {
      days          = 30
      storage_class = "GLACIER"
    }

    expiration {
      days = 90
    }

    noncurrent_version_expiration {
      noncurrent_days = 30
    }

    abort_incomplete_multipart_upload {
      days_after_initiation = 7
    }
  }
}
```

## Métricas Prometheus

Expuestas vía aws-cloudwatch-exporter o equivalente:

| Métrica | Significado |
|---|---|
| `aws_s3_replication_latency_seconds` | tiempo entre PUT origen y replicado destino |
| `aws_s3_replication_pending_bytes` | bytes pendientes de replicar |
| `aws_s3_replication_failed_total` | counter de objetos que fallaron replicación |

Alertas:
- `aws_s3_replication_latency_seconds > 900` por 10min → critical
- `aws_s3_replication_failed_total > 0` → critical
- `aws_s3_replication_pending_bytes > 100GB` → warning

## Smoke test (post-deploy)

```bash
# 1. Subir test object al primary
aws s3 cp testfile.txt s3://domain-backups/dr-test/$(date -u +%s).txt

# 2. Esperar ~15 min y verificar en DR
aws s3 ls s3://domain-backups-dr/dr-test/ --region eu-west-1
```

## DR runbook (HU-18.3 vinculado)

Si la región primaria está caída:

```bash
# 1. Cambiar config pgBackRest para apuntar al bucket DR
pgbackrest --stanza=domain --repo=2 restore

# 2. Levantar Postgres con datos restaurados
systemctl start postgresql

# 3. Una vez recuperado el primary, re-replicar y volver a us-east-1
```

## Notas

- El KMS key del DR es independiente del primario (cross-region keys).
- Storage class STANDARD_IA es ~40% más barato que STANDARD para DR
  (acceso infrecuente esperado).
- Replication Time Control (RTC) garantiza 99.99% replicación < 15min.
- Delete markers se replican: si borrás del primary, borrás del DR (cuidado).
