# issue-24.3-k8s-deployment-examples

**Origen:** `REQ-24-deployment`
**Prioridad tentativa:** media
**Tipo:** docs + examples

## Historia de usuario

**Como** operador evaluando Domain
**Quiero** examples ejecutables paso-a-paso para AWS EKS, GCP GKE y bare-metal k3s
**Para** levantar Domain en mi target sin partir de cero

## Criterios de aceptación

### Escenario 1: AWS EKS example

```gherkin
Dado que existe `deploy/examples/aws-eks/`
Cuando un operador con AWS account sigue el README
Entonces termina con Domain corriendo + RDS Postgres + S3 + ALB
Y tiempo esperado <30min con account ya configurada
Y costo mensual estimado documentado (~$200-400/mes baseline)
```

### Escenario 2: GCP GKE example

```gherkin
Dado que existe `deploy/examples/gcp-gke/`
Cuando operador con GCP project sigue
Entonces Domain corriendo + Cloud SQL Postgres + GCS + GLB ingress
Y costo doc
```

### Escenario 3: Bare-metal k3s

```gherkin
Dado que existe `deploy/examples/baremetal-k3s/`
Cuando operador con 1+ VPS sigue
Entonces Domain corriendo en k3s single-node con MinIO + Postgres self-hosted
Y traefik ingress + cert-manager LetsEncrypt
```

### Escenario 4: Cada example incluye

```gherkin
Dado que examen cualquier example
Entonces contiene:
  - README.md con pre-requisitos, steps numerados, validation
  - terraform/ o cloudformation/ o bash scripts para infra
  - kubernetes/ con values.yaml customizado (chart helm) o overlay (kustomize)
  - validation/ con curl scripts smoke
  - cleanup.sh para tirar todo abajo
```

### Escenario 5: CI test examples (opcional best-effort)

```gherkin
Dado que CI tiene secrets para sandbox accounts
Cuando se ejecuta workflow `examples-smoke.yml` (manual o weekly)
Entonces se ejecuta cada example end-to-end en cuenta sandbox
Y se reporta success/fail por example
```

## Análisis breve

- **Qué pide:** 3 examples ejecutables con IaC + Helm/Kustomize + validation
- **Esfuerzo:** L
- **Riesgos:** mantenimiento alto (cloud APIs cambian); costo de testing cuentas sandbox
