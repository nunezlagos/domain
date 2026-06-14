# HU-28.2-constructors-validation

**Origen:** `REQ-28-arquitectura-interna`
**Prioridad tentativa:** alta
**Tipo:** refactor

## Historia de usuario

**Como** desarrollador de Domain
**Quiero** que cada service se construya con `NewService(pool, audit, ...)` que valida sus dependencias en lugar de struct literals públicos
**Para** que un service sin Pool no compile en vez de crashear en runtime con nil pointer dereference

## Contexto

Hoy la mayoría de los services se construyen así:
```go
&flow.Service{Pool: pools.App, Audit: recorder}
```

Esto tiene 3 problemas:
1. **Sin validación**: si `pools.App` es nil, compila pero crashea en el primer query con nil pointer dereference
2. **Fields públicos y mutables**: nada impide que otro package haga `svc.Pool = nil` después de construir
3. **Sin constructor canónico**: cada lugar que construye el service puede olvidar campos

Esta HU agrega constructores públicos con validación y hace los fields `Pool` privados (o los mantiene públicos con doc `Deprecated` mientras coexiste el código legacy). Se aplica a los mismos 5 services de HU-28.1 más todos los demás que aparecen en `cmd/domain/main.go`.

## Criterios de aceptación

### Escenario 1: Constructor con validación

```gherkin
Dado que existe `flow.NewService(pool, audit, repo)`
Cuando pool es nil
Entonces NewService retorna error (o panic temprano con mensaje claro)
Y el service no se construye en estado inválido
```

### Escenario 2: Backward-compat

```gherkin
Dado que código legacy usa `&flow.Service{Pool: p, Audit: a}`
Cuando la HU introduce `NewService`
Entonces el struct literal legacy sigue compilando (fields se mantienen públicos con doc Deprecated)
```

### Escenario 3: Fields mutables encapsulados

```gherkin
Dado un service construido con NewService
Cuando intento `svc.Pool = nil` desde otro package
Entonces no compila (Pool field es privado o inmutable)
```

## Análisis breve

- **Qué pide:** Funciones `NewService` por cada service en `cmd/domain/main.go` + validación de nil + fields privados
- **Módulos afectados:** Todos los packages `service/*/` + `cmd/domain/main.go` + `cmd/domain-mcp/main.go`
- **Esfuerzo tentativo:** M (2 días)
- **Dependencias:** HU-28.1 (para los 5 services con repository interfaces)
