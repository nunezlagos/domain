# K8s Deployment Examples — HU-24.3

Manifiestos K8s puros (sin Helm) para deployar Domain. Útiles cuando:

- Helm no está disponible en el cluster
- Querés customizar cada recurso a mano
- Onboarding inicial: ver los recursos sin abstracción

Para producción con upgrades automáticos, preferí el Helm chart en
`deploy/helm/domain/`.

## Orden de aplicación

```bash
kubectl apply -f deploy/k8s-examples/01-namespace.yaml
# Reemplazá los valores REPLACE_ME en 02-secrets.yaml antes de aplicar
kubectl apply -f deploy/k8s-examples/02-secrets.yaml
kubectl apply -f deploy/k8s-examples/06-migrations-job.yaml
# Esperar a que el Job complete
kubectl wait --for=condition=complete job/domain-migrate -n domain --timeout=5m
# Aplicar el resto
kubectl apply -f deploy/k8s-examples/03-deployment.yaml
kubectl apply -f deploy/k8s-examples/04-service-hpa.yaml
kubectl apply -f deploy/k8s-examples/05-networkpolicy.yaml
```

## Pre-requisitos

- **Cluster K8s 1.27+** (HPA v2, PodSecurityStandards stable)
- **Postgres 16+** con pgvector + pgaudit en namespace `postgres` (o donde sea
  accesible vía el Service `postgres` en el DSN)
- **Image registry** con la imagen `ghcr.io/nunezlagos/domain:VERSION` accesible
- **Ingress controller** (nginx, traefik, etc.) en namespace `ingress-nginx`
  si querés exponer la API externamente
- **Prometheus + ServiceMonitor CRD** en namespace `monitoring` para scrape
  de métricas

## Recursos creados

| Archivo | Recurso | Propósito |
|---|---|---|
| `01-namespace.yaml` | Namespace | Aislamiento + PSS restricted |
| `02-secrets.yaml` | 2 Secrets | DB DSNs + master key + SMTP |
| `03-deployment.yaml` | Deployment + SA | 3 réplicas, securityContext hardened, GOMEMLIMIT |
| `04-service-hpa.yaml` | Service + HPA | ClusterIP + autoescalado 3-12 pods CPU/mem |
| `05-networkpolicy.yaml` | 3 NetworkPolicies | Default-deny + ingress + egress |
| `06-migrations-job.yaml` | Job | Aplica migrations al boot |

## Ingress (no incluido)

El Service es ClusterIP. Para exponer externamente añadí un Ingress según
tu controller. Ejemplo con nginx:

```yaml
apiVersion: networking.k8s.io/v1
kind: Ingress
metadata:
  name: domain-api
  namespace: domain
  annotations:
    cert-manager.io/cluster-issuer: letsencrypt-prod
spec:
  ingressClassName: nginx
  tls:
  - hosts: [api.example.com]
    secretName: domain-api-tls
  rules:
  - host: api.example.com
    http:
      paths:
      - path: /
        pathType: Prefix
        backend:
          service:
            name: domain-server
            port:
              number: 80
```

## ServiceMonitor (opcional)

Si tenés Prometheus Operator:

```yaml
apiVersion: monitoring.coreos.com/v1
kind: ServiceMonitor
metadata:
  name: domain-server
  namespace: monitoring
spec:
  namespaceSelector:
    matchNames: [domain]
  selector:
    matchLabels:
      app: domain-server
  endpoints:
  - port: metrics
    interval: 30s
    path: /metrics
```

## Diferencias vs Helm chart

| Aspecto | k8s-examples/ | Helm chart |
|---|---|---|
| Customización | Editar yaml a mano | values.yaml + templates |
| Upgrade flow | kubectl apply | helm upgrade con hooks pre-upgrade |
| Migrations | Job separado a mano | hook pre-upgrade automático |
| Rollback | kubectl rollout undo | helm rollback (con state guardado) |
| Multi-environment | copy-paste o kustomize | values-prod.yaml + values-staging.yaml |
| ConfigMap gen | Inline en env | from .Values |

## Hardening incluido

- `securityContext.runAsNonRoot=true` + `runAsUser=65532` (distroless nonroot UID)
- `readOnlyRootFilesystem=true` + emptyDir para `/tmp`
- `allowPrivilegeEscalation=false`
- `capabilities.drop=[ALL]`
- `seccompProfile=RuntimeDefault`
- `PSS restricted` enforce en namespace
- NetworkPolicy default-deny + allowlist explícita

## Validar antes de aplicar

```bash
# kubeconform: validación schema CRD
kubeconform -strict deploy/k8s-examples/*.yaml

# polaris: best-practices check
polaris audit --audit-path deploy/k8s-examples/

# kyverno: policy enforcement
kyverno apply policies/ --resource deploy/k8s-examples/03-deployment.yaml
```
