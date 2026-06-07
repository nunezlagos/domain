# Design: HU-18.2-s3-replication

## Decisión arquitectónica

**IaC:** Terraform 1.6+ con state remoto S3+DynamoDB.
**Encryption:** SSE-KMS con CMK por env, rotation anual.
**Replication:** Cross-Region (no Same-Region) para protección regional real.
**Storage class default:** STANDARD; lifecycle transitions hacia IA/Glacier.

## Alternativas descartadas

- **AWS Backup en lugar de CRR:** AWS Backup es bueno para EBS/RDS, no añade valor sobre CRR para S3
- **MinIO en lugar de S3 prod:** MinIO requiere operación propia; S3 administrado es default

## Estructura Terraform

```
deploy/terraform/
  modules/s3-bucket/
    main.tf       # bucket + versioning + bpa + sse-kms
    replication.tf
    lifecycle.tf
    variables.tf
    outputs.tf
  envs/
    dev/main.tf      # MinIO local, no CRR
    staging/main.tf  # CRR a misma cuenta otra región
    prod/main.tf     # CRR cross-account opcional
```

## TDD plan

1. terraform plan idempotente (2da run sin cambios)
2. Upload → presente en dr <15min (integration test con AWS real)
3. Lifecycle simulado: objeto con S3 Object Tagging fake age → transition
4. Sabotaje: deshabilitar block public access → terraform plan diff alarma
