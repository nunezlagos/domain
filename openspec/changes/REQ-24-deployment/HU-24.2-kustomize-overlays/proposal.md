# Proposal: HU-24.2-kustomize-overlays

## Intención

Kustomize base + overlays dev/staging/prod listos para `kubectl apply -k`, con secrets via External Secrets Operator, CI validation, docs para elegir entre Helm y Kustomize.

## Scope

**Incluye:**
- `deploy/kustomize/base/` con manifests core
- Overlays dev/staging/prod con patches específicos
- Integración ESO opcional (External Secrets Operator)
- CI workflow validate
- Docs comparativo Helm vs Kustomize

**No incluye:**
- Helmfile (mezclar Helm + Kustomize → confusión)
- ArgoCD/Flux specific (cualquier GitOps puede consumirlo)

## Enfoque técnico

1. Base con labels comunes app=domain
2. Overlays usan strategicMerge patches + namePrefix opcional
3. ESO templates con CRD ExternalSecret
4. CI ejecuta build + kubeconform por overlay

## Riesgos

- Drift Helm vs Kustomize: docs claros + un único source of truth para values core
- ESO not available: fallback documentación a sealed-secrets

## Testing

- kustomize build cada overlay
- kubeconform clean
- Apply en kind cluster smoke
