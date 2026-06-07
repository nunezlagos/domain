# HU-24.1-helm-chart

**Origen:** `REQ-24-deployment`
**Persona:** platform-engineer, security-officer
**Prioridad tentativa:** alta
**Tipo:** infrastructure

## Historia de usuario

**Como** operador instalando Domain en Kubernetes
**Quiero** un Helm chart oficial publicado en OCI registry con valores documentados
**Para** instalar Domain con `helm install` sin escribir manifests a mano

## Criterios de aceptación

### Escenario 1: Helm chart estructura

```gherkin
Dado que existe `deploy/helm/domain/`
Cuando inspecciono
Entonces tiene estructura estándar:
  - Chart.yaml (apiVersion v2, version semver, appVersion)
  - values.yaml documentado con comentarios
  - values.schema.json para validación
  - templates/ con deployment, service, ingress, hpa, pdb, networkpolicy, serviceaccount, secret, configmap
  - templates/tests/ con helm test pods
  - README.md generado con helm-docs
```

### Escenario 2: helm install minimal

```gherkin
Dado que tengo Kubernetes + Postgres externa
Cuando ejecuto:
  helm install domain oci://ghcr.io/domain/charts/domain \
    --set domainDatabaseUrl="postgres://..." \
    --set domainS3Endpoint="..." \
    --set domainS3AccessKey="..." \
    --set domainS3SecretKey="..."
Entonces Domain queda corriendo en 60s
Y helm test pasa (smoke /health 200)
```

### Escenario 3: Values documentados

```gherkin
Dado que values.yaml incluye sections:
  - image (repository, tag, pullPolicy, pullSecrets)
  - replicaCount + hpa (min, max, targetCPU/Memory)
  - resources (requests/limits)
  - podDisruptionBudget (minAvailable)
  - ingress (enabled, className, hosts, tls)
  - serviceAccount (annotations IRSA opcional)
  - networkPolicy (enabled, ingress allowFrom, egress)
  - secrets (existingSecret OR inline create)
  - postgres (external vs subchart)
  - persistence (no aplicable; stateless app)
  - extraEnv, extraEnvFrom
  - probes (livenessProbe, readinessProbe, startupProbe)
Y todas tienen defaults sensatos
```

### Escenario 4: CI test contra Kind

```gherkin
Dado que existe `.github/workflows/helm-chart-ci.yml`
Cuando se ejecuta en PR que toca deploy/helm/
Entonces se hace:
  - helm lint
  - kubeval / kubeconform sobre rendered manifests
  - kind create cluster
  - helm install
  - kubectl wait for ready
  - helm test
  - kind delete cluster
```

### Escenario 5: Upgrade in-place

```gherkin
Dado que existe instalación previa
Cuando helm upgrade con nueva version
Entonces se hace rolling update
Y se ejecutan migraciones DB en hook pre-upgrade (Job)
Y si la migración falla, no se rolla el deploy
```

### Escenario 6: Publicación OCI

```gherkin
Dado que se taggea release vX.Y.Z
Cuando CI corre
Entonces se publica chart en oci://ghcr.io/domain/charts/domain:X.Y.Z
Y appVersion matchea binary version
```

## Análisis breve

- **Qué pide:** chart Helm v3 + OCI publish + CI Kind + helm-docs + hooks migraciones
- **Esfuerzo:** L
- **Riesgos:** drift entre chart y binary; values overrides incompatibles; hook orphans
