# Design: issue-24.3-k8s-deployment-examples

## Estructura

```
deploy/examples/
  aws-eks/
    README.md
    terraform/
      main.tf  # EKS + RDS + S3 + IAM IRSA
      variables.tf
      outputs.tf
    kubernetes/
      values.yaml
      install.sh
    validation/
      smoke.sh
    cleanup.sh
  gcp-gke/
    README.md
    terraform/
    kubernetes/
    validation/
    cleanup.sh
  baremetal-k3s/
    README.md
    install/
      00-k3s.sh
      01-cert-manager.sh
      02-postgres.sh
      03-minio.sh
      04-domain.sh
    kubernetes/
      values.yaml
    validation/
    cleanup.sh
```

## Cost estimates (docs)

| target | baseline mensual |
|--------|------------------|
| AWS EKS 2x t3.medium + RDS db.t3.small + S3 | $200-400 |
| GCP GKE Autopilot + Cloud SQL db-f1-small + GCS | $150-350 |
| Bare-metal 1 VPS 4vCPU/8GB DigitalOcean | $48 |

## TDD plan

1. README readable end-to-end
2. terraform plan idempotent
3. install.sh idempotent
4. smoke valida /health
5. cleanup.sh tira todo (no orphans)
6. CI weekly contra sandbox (best-effort)
