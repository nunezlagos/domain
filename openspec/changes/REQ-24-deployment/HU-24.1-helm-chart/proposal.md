# Proposal: HU-24.1-helm-chart

## Intención

Helm chart oficial `domain/domain-mcp` con values documentados, schema validation, hooks migraciones, helm tests, CI contra Kind, publish OCI con cada release.

## Scope

**Incluye:**
- `deploy/helm/domain/` con estructura completa
- values.yaml + values.schema.json
- templates con Deployment, Service, Ingress, HPA, PDB, NetworkPolicy, ServiceAccount, ConfigMap, Secret, ServiceMonitor (Prometheus Operator opcional)
- Hooks pre-upgrade: migration Job
- Hooks helm test: smoke /health
- README.md autogenerado con norwoodj/helm-docs
- CI Kind: lint, kubeval, install, test
- Publish OCI a ghcr.io en tag

**No incluye:**
- Subchart Postgres (recomendamos external managed; opcional via dependencies)
- Subchart Redis (no aplica, Postgres-only)
- Subchart MinIO (recomendamos S3 external en prod)

## Enfoque técnico

1. Chart apiVersion v2 con SemVer
2. values.schema.json valida tipos + required
3. Sealed-secrets compatible (no secret value en values)
4. NetworkPolicy default-deny + explicit allows
5. Probes calibradas con HU-01.3 health

## Riesgos

- Chart drift: bump chart version cada release
- Migration hook idempotency: golang-migrate ya es idempotente
- Values overrides romper: schema validation + tests

## Testing

- helm lint clean
- kubeval rendered manifests
- Kind install + helm test
- Upgrade in-place
- Migration hook ejecuta y falla rolls back
