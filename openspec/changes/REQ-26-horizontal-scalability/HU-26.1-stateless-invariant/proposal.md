# Proposal: HU-26.1-stateless-invariant

## Intención

Linter Go que detecta state crítico in-memory + whitelist explícita + tests multi-pod confirman comportamiento.

## Scope

- AST scan paquetes `internal/`, `cmd/`
- Detección: globals mutables, maps sin sync, sync.Map sin comment
- `.stateless-allowed.yaml` con reason por cada exception
- CI fail si no-whitelisted
- Tests integration 2 pods stateless

## Riesgos

- Falsos positivos: whitelist explícita
- Patterns legítimos (logger global): documentar en whitelist

## Testing

- Linter detecta var counter global → fail
- Whitelist con reason válida → pass
- 2 pods test: request A llega a pod 1 ok, request B llega a pod 2 ok, ambos consistentes
