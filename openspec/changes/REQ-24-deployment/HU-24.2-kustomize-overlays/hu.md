# HU-24.2-kustomize-overlays

**Origen:** `REQ-24-deployment`
**Persona:** platform-engineer, security-officer
**Prioridad tentativa:** media
**Tipo:** infrastructure

## Historia de usuario

**Como** operador que prefiere Kustomize sobre Helm o necesita personalizaciones complejas
**Quiero** base + overlays Kustomize listos
**Para** generar manifests sin templating engine

## Criterios de aceptación

### Escenario 1: Base manifests

```gherkin
Dado que existe `deploy/kustomize/base/`
Cuando inspecciono
Entonces contiene:
  - kustomization.yaml
  - deployment.yaml
  - service.yaml
  - configmap.yaml
  - serviceaccount.yaml
  - networkpolicy.yaml
Y `kubectl apply -k deploy/kustomize/base` funciona en un cluster con dependencias externas resueltas
```

### Escenario 2: Overlays per environment

```gherkin
Dado que existen overlays:
  - deploy/kustomize/overlays/dev/
  - deploy/kustomize/overlays/staging/
  - deploy/kustomize/overlays/prod/
Cuando aplico cada uno
Entonces patches modifican replicas, resources, env vars, image tag específicos
Y `kubectl apply -k overlays/prod` rinde manifests prod-ready
```

### Escenario 3: Secrets via ESO o SealedSecrets

```gherkin
Dado que prod overlay incluye `external-secrets/` (ESO CRDs)
Cuando aplico
Entonces los Secrets se sync desde AWS Secrets Manager / GCP Secret Manager
Y NUNCA se committea plaintext
```

### Escenario 4: kustomize build validation

```gherkin
Dado que CI corre en PR que toca deploy/kustomize/
Cuando se ejecuta
Entonces hace `kustomize build overlays/X | kubeconform`
Y falla si hay drift / errores
```

### Escenario 5: Coexistencia con Helm

```gherkin
Dado que ya hay un release Helm instalado
Cuando aplico kustomize overlay
Entonces NO conflict (resources diferentes namespaces o labels diferenciadores)
Y docs explican cuándo usar cuál
```

## Análisis breve

- **Qué pide:** base + 3 overlays + ESO + CI validate + docs
- **Esfuerzo:** M
- **Riesgos:** drift entre Helm y Kustomize templates; secret management cross-cloud
