# HU-22.1-sdk-python

**Origen:** `REQ-22-sdk-clients`
**Prioridad tentativa:** media
**Tipo:** feature

## Historia de usuario

**Como** developer Python integrando Domain
**Quiero** un SDK Python oficial generado desde OpenAPI con typing completo
**Para** consumir el API sin escribir HTTP a mano

## Criterios de aceptación

### Escenario 1: Generación desde OpenAPI

```gherkin
Dado que existe `openapi.yaml` versionado generado desde el código Go
Cuando ejecuto `make sdk-python`
Entonces se genera `sdks/python/domain_sdk/` con typings (pydantic v2) y clases cliente
Y el código generado es committeado para revisión
```

### Escenario 2: Async + sync client

```gherkin
Dado que el SDK genera ambos
Cuando importo
  ```python
  from domain_sdk import DomainClient, AsyncDomainClient
  ```
Entonces ambos clientes existen
Y comparten misma API (mismo método ej: `client.observations.create(...)`)
```

### Escenario 3: Autenticación API key

```gherkin
Dado que existe `DomainClient(base_url="https://api.domain.sh", api_key="sk_...")`
Cuando hago `client.observations.list()`
Entonces el header `Authorization: Bearer sk_...` se incluye
Y errores 401 levantan `domain_sdk.exceptions.AuthenticationError`
```

### Escenario 4: Pagination helpers

```gherkin
Dado que endpoint list paginado devuelve 100 items por página
Cuando hago `for obs in client.observations.iter_all(): ...`
Entonces el iterator pagina transparentemente hasta agotar
```

### Escenario 5: Retry + backoff

```gherkin
Dado que el server responde 503 transitorio
Cuando llamo método del SDK
Entonces se reintenta con backoff exponencial (3 attempts default)
Y errores 4xx no se reintentan
```

### Escenario 6: Publicación PyPI

```gherkin
Dado que el API publica release v1.2.3
Cuando se ejecuta CI release
Entonces se publica `domain-sdk==1.2.3` en PyPI
Y se actualiza `docs.domain.sh/sdk/python`
```

## Análisis breve

- **Qué pide:** OpenAPI generator + pydantic v2 + httpx + publish PyPI
- **Esfuerzo:** M
- **Riesgos:** OpenAPI spec incompleta → SDK incompleto; mantener compat entre versiones
