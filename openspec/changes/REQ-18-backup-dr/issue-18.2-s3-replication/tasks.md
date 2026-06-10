# Tasks: issue-18.2-s3-replication

- [ ] **tf-001**: Módulo `s3-bucket` con versioning, BPA, SSE-KMS
- [ ] **tf-002**: Replication rule + IAM role + CRR a otra región
- [ ] **tf-003**: Lifecycle: STANDARD → IA 30d → Glacier IR 90d → Deep Archive 365d; versiones >90d expiran
- [ ] **tf-004**: Environments dev (MinIO), staging, prod
- [ ] **tf-005**: Métrica `domain_s3_replication_lag_seconds` desde CloudWatch
- [ ] **test-001**: terraform plan idempotente
- [ ] **test-002**: Upload + replication lag check (integration)
- [ ] **test-003**: Versioning restore drill
- [ ] **docs-001**: `docs/runbooks/s3-restore.md` con steps para restaurar versión anterior
