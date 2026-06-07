# Proposal: HU-18.2-s3-replication

## Intención

Configurar buckets S3 con versionado, cross-region replication, lifecycle policies y bloqueo de acceso público. Reducir riesgo de pérdida por borrado, corrupción o caída regional.

## Scope

**Incluye:**
- Terraform module `deploy/terraform/s3-buckets/` reusable
- Buckets primary y dr (otra región)
- Versioning + Block Public Access en ambos
- CRR rule con IAM role asociado y replicación de delete markers configurable
- Lifecycle rules con transitions a IA/Glacier
- Métrica replication lag publicada en /metrics
- SSE-KMS con CMK gestionado

**No incluye:**
- Multi-cloud (Azure/GCP)
- Bucket policies de cross-account access (otro escenario futuro)

## Enfoque técnico

1. Terraform con state remoto (S3 + DynamoDB lock)
2. Módulo parametrizable por env (dev/staging/prod)
3. Replication metrics via CloudWatch + scrape a Prometheus
4. Documentar costos esperados por TB/mes

## Riesgos

- CRR cuesta dinero proporcional a datos transferidos → considerar Same-Region Replication para casos sin DR
- Versiones huérfanas si lifecycle no expira → tests en staging
- KMS key rotation → automatizada con policy

## Testing

- Upload a primary → verifica aparece en dr <15min
- Borrar objeto → versión previa restaurable
- Lifecycle: objeto fake con edad simulada → transition ocurre
