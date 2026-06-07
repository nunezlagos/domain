# HU-13.8-api-versioning-policy

**Origen:** `REQ-13-http-api`
**Persona:** integrator
**Prioridad tentativa:** media
**Tipo:** policy + feature

## Historia de usuario

**Como** owner del producto
**Quiero** una política clara y técnicamente soportada de versionado de API
**Para** evolucionar sin romper clientes en producción

## Política propuesta

- **URL versioning**: `/api/v1/`, `/api/v2/` (visible, explícito)
- **Breaking changes** solo en major version (v1→v2)
- **Additive changes** OK en patch (campos nuevos opcionales, endpoints nuevos)
- **Deprecation period**: 12 meses mínimo desde anuncio antes de retirar v anterior
- **Sunset header** RFC 8594 desde 6 meses antes
- **Deprecation header** RFC 8594 desde anuncio
- **Changelog público** en `docs/api/changelog.md`

## Criterios de aceptación

### Escenario 1: Header Deprecation activo

```gherkin
Dado que v1 fue marcada deprecated el 2026-06-01 (sunset 2027-06-01)
Cuando cliente llama cualquier endpoint /api/v1/*
Entonces response incluye:
  `Deprecation: @1717200000`
  `Sunset: Wed, 01 Jun 2027 00:00:00 GMT`
  `Link: <https://docs.domain.sh/api/migrate-v2>; rel="deprecation"`
```

### Escenario 2: Después de sunset

```gherkin
Dado que la fecha sunset pasó
Cuando cliente llama /api/v1/*
Entonces 410 Gone con body explicando migración
Y header `Link: <...migrate-v2>`
```

### Escenario 3: Versioning support matrix

```gherkin
Dado que existe `docs/api/support-matrix.md`
Entonces declara qué versions están active, deprecated, sunset
Y se actualiza con cada release
```

### Escenario 4: Endpoint /api/version

```gherkin
Dado que GET /api/version
Entonces devuelve `{api_versions: ["v1","v2"], current: "v2", deprecated: ["v1"], sunset: {v1: "2027-06-01"}}`
```

### Escenario 5: SDK auto-detecta version

```gherkin
Dado que SDK Python/TS/Go está pinned a v2
Cuando server upgrade
Entonces SDK sigue funcionando si v2 sigue active
Y si v2 deprecated, SDK loguea warning al import
```

## Análisis breve

- **Qué pide:** policy documentada + headers + sunset enforcement + matrix + version endpoint
- **Esfuerzo:** S
- **Riesgos:** mantenimiento doble durante deprecation period; sunset enforcement breaking clientes lazy
