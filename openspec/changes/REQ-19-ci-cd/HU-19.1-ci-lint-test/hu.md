# HU-19.1-ci-lint-test

**Origen:** `REQ-19-ci-cd`
**Persona:** platform-engineer
**Prioridad tentativa:** alta
**Tipo:** infrastructure

## Historia de usuario

**Como** desarrollador del repo Domain
**Quiero** que cada PR/push corra lint + unit + integration tests en GitHub Actions con cache
**Para** detectar regresiones antes de mergear con feedback en <5 min

## Criterios de aceptación

### Escenario 1: Pipeline en cada PR

```gherkin
Dado que abro un PR contra `main`
Cuando GitHub Actions dispara workflow `ci.yml`
Entonces se ejecutan jobs en orden:
  | job          | tool                | timeout |
  | lint         | golangci-lint v1.60 | 3 min   |
  | unit         | go test -race       | 5 min   |
  | integration  | testcontainers + pg | 10 min  |
  | coverage     | gocovsh + comment   | 2 min   |
Y todos pasan en verde antes de permitir merge (branch protection)
```

### Escenario 2: Cache de dependencias y Docker layers

```gherkin
Dado que el workflow usa `actions/setup-go@v5` con cache
Y `docker/setup-buildx-action@v3` con cache
Cuando se ejecuta segunda vez sin cambios en go.mod
Entonces el step de download deps es <10s (cache hit)
```

### Escenario 3: Tests de integración con servicios reales

```gherkin
Dado que `internal/store/postgres_test.go` usa testcontainers
Cuando el job integration corre
Entonces levanta Postgres+pgvector y MinIO en containers
Y los tests pasan sin mocks de la DB
Y se publica resumen del coverage como PR comment
```

### Escenario 4: Required status checks

```gherkin
Dado que main tiene branch protection configurado
Cuando un PR no tiene lint+unit+integration verdes
Entonces el botón merge está deshabilitado
Y el reviewer ve el status check rojo
```

## Análisis breve

- **Qué pide:** workflows GitHub Actions, caching, testcontainers, branch protection
- **Esfuerzo:** S
- **Riesgos:** flakiness de integration tests; minutes de Actions costosos si no se cachea
