# Tasks: issue-18.2-s3-replication

- [x] **tf-001**: Módulo `s3-bucket` con versioning, BPA, SSE-KMS
- [x] **tf-002**: Replication rule + IAM role + CRR a otra región
- [x] **tf-003**: Lifecycle: STANDARD → IA 30d → Glacier IR 90d → Deep Archive 365d; versiones >90d expiran
- [x] **tf-004**: Environments dev (MinIO), staging, prod
- [x] **tf-005**: Métrica `domain_s3_replication_lag_seconds` desde CloudWatch
- [x] **test-001**: terraform plan idempotente
- [x] **test-002**: Upload + replication lag check (integration)
- [x] **test-003**: Versioning restore drill
- [x] **docs-001**: `docs/runbooks/s3-restore.md` con steps para restaurar versión anterior
