# Design: issue-22.1-sdk-python

## Decisión arquitectónica

**Generator:** `openapi-python-client` v0.21+ (Jinja2 templating, pydantic v2)
**HTTP:** httpx (sync + async)
**Lock OpenAPI:** spec committed bajo `openapi/v1.yaml`

## Estructura repo

```
sdks/python/
  pyproject.toml
  README.md
  domain_sdk/
    __init__.py
    client.py          # DomainClient, AsyncDomainClient
    exceptions.py
    pagination.py
    _generated/        # all auto-generated
      models/...
      api/...
  tests/
```

## Comando generación

```bash
make sdk-python  # invoca openapi-python-client generate -p openapi/v1.yaml -o sdks/python --custom-template ...
```

## TDD plan

1. Spec fixture → generated client compila
2. Auth header se incluye
3. 401 → AuthenticationError
4. Retry 503 → ok después de N
5. Iter pagination consume todas las páginas
6. mypy strict pass
