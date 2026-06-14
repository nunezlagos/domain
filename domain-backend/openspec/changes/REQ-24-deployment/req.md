# REQ-24-deployment: Deployment a Kubernetes: Helm chart oficial, Kustomize overlays, deployment examples para AWS/GCP/bare-metal.

**Estado:** activo
**Creado:** 2026-06-07
**Fase:** F4

## Descripción

Hacer deployable Domain en producción sobre Kubernetes (cloud y on-prem) sin reinventar cada vez. Helm chart oficial, Kustomize overlays para personalización, y manifests + docs por target (AWS EKS, GCP GKE, bare-metal con k3s).

## Criterios de éxito

- Helm chart `domain/domain-mcp` publicado en repo OCI con values documentados y CI test contra Kind
- Kustomize base + overlays dev/staging/prod para personalizar sin forkear el chart
- Deployment examples ejecutables: AWS EKS, GCP GKE, bare-metal k3s
- Ingress, HPA, PDB, NetworkPolicies, ServiceAccounts con least-privilege
- Secrets gestionados vía External Secrets Operator (recomendado) o sealed-secrets

## HUs hijas

| HU | Estado | Descripción |
|----|--------|-------------|
| issue-24.1-helm-chart | proposed | Helm chart oficial con values, hooks, tests, CI contra Kind |
| issue-24.2-kustomize-overlays | proposed | Base + overlays dev/staging/prod, NetworkPolicies, RBAC |
| issue-24.3-k8s-deployment-examples | proposed | Examples ejecutables AWS EKS, GCP GKE, bare-metal k3s |
