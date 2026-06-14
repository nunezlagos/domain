# Design: issue-24.2-kustomize-overlays

## Estructura

```
deploy/kustomize/
  base/
    kustomization.yaml
    deployment.yaml
    service.yaml
    configmap.yaml
    serviceaccount.yaml
    networkpolicy.yaml
  overlays/
    dev/
      kustomization.yaml
      patch-replicas.yaml
      configmap.env
    staging/
      kustomization.yaml
      patch-image-tag.yaml
      external-secret.yaml
    prod/
      kustomization.yaml
      patch-resources.yaml
      patch-hpa.yaml
      external-secret-aws.yaml
      pdb.yaml
```

## Base kustomization.yaml

```yaml
apiVersion: kustomize.config.k8s.io/v1beta1
kind: Kustomization
commonLabels:
  app.kubernetes.io/name: domain
  app.kubernetes.io/part-of: domain-platform
resources:
  - deployment.yaml
  - service.yaml
  - configmap.yaml
  - serviceaccount.yaml
  - networkpolicy.yaml
```

## TDD plan

1. `kustomize build base` clean
2. `kustomize build overlays/dev` clean
3. kubeconform sobre output
4. Apply en kind, deployment ready
