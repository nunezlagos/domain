# Proposal: issue-24.3-k8s-deployment-examples

## Intención

Tres examples ejecutables con IaC + chart Helm + validation: AWS EKS, GCP GKE, bare-metal k3s. README paso-a-paso, cost estimates, cleanup script.

## Scope

**Incluye:**
- AWS EKS: Terraform module + RDS Postgres + S3 + ALB ingress + IRSA
- GCP GKE: Terraform module + Cloud SQL + GCS + GLB ingress + Workload Identity
- Bare-metal k3s: install script + Postgres + MinIO + Traefik + cert-manager
- Validation scripts smoke por example
- Cleanup scripts
- Cost estimates docs

**No incluye:**
- Azure AKS (priorizar AWS/GCP por mayor demanda; agregar después si pedido)
- On-prem OpenShift (requiere certificación; futuro)

## Enfoque técnico

1. Terraform 1.6+ con state remoto S3+DynamoDB / GCS+lock
2. Helm chart de issue-24.1 referenciado
3. Validation con curl + jq sobre /health, /api/version, /metrics
4. Cleanup destruye TODO incluido state remoto

## Riesgos

- Cloud APIs change: pin provider versions
- Cost surprise: docs claros con upper bound
- Maintenance: contributors mensual update

## Testing

- Por example: kind / minikube si aplica para CI fast
- Weekly CI manual contra sandbox real
- README readable
