# Kustomize Overlays — Domain (HU-24.2)

Reutiliza los manifests canónicos de `deploy/k8s-examples/` (HU-24.3) como
`base` y aplica `overlays` por entorno. Alternativa a `deploy/helm/domain/`
(HU-24.1) para equipos que prefieren Kustomize plano sobre Helm templates.

## Estructura

```
deploy/kustomize/
├── base/
│   └── kustomization.yaml              # Importa los 6 manifests k8s-examples
└── overlays/
    ├── dev/
    │   └── kustomization.yaml          # 1 réplica, log debug
    ├── staging/
    │   └── kustomization.yaml          # 2 réplicas, HPA 2-6
    └── prod/
        ├── kustomization.yaml          # 3 réplicas, HPA 3-12, json logs
        └── pdb.yaml                    # PodDisruptionBudget minAvailable=2
```

## Aplicar

```bash
# Preview render sin aplicar
kubectl kustomize deploy/kustomize/overlays/prod

# Apply dev
kubectl apply -k deploy/kustomize/overlays/dev

# Apply prod (después de revisar diff)
kubectl apply -k deploy/kustomize/overlays/prod
```

## Diferencias vs Helm chart (HU-24.1)

| Aspecto | Kustomize | Helm |
|---|---|---|
| Versionado | git ref + commit | chart version OCI |
| Templating | strategic merge patches | Go templates |
| Configuración | overlays explícitos | values.yaml |
| Curva | menor | mayor pero más potente |

Ambos son válidos. Los manifests subyacentes son idénticos.

## Validación pre-apply

```bash
kustomize build deploy/kustomize/overlays/prod | kubeconform -strict -
```
