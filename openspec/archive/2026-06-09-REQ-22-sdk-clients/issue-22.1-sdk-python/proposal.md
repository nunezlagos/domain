# Proposal: issue-22.1-sdk-python

## Intención

SDK Python oficial generado desde OpenAPI 3.1 spec del API HTTP Domain, con clientes async y sync, typing pydantic v2, retries, pagination helpers, publicado a PyPI desde CI.

## Scope

**Incluye:**
- OpenAPI 3.1 generator (`openapi-python-client` o custom Jinja templates sobre `openapi-generator`)
- Cliente async (httpx.AsyncClient) y sync (httpx.Client)
- Pydantic v2 models
- Exceptions tipadas por código de error
- Retry con tenacity (exp backoff)
- Iter pagination helper
- `pyproject.toml` con metadata, classifiers, py>=3.10
- CI publish a PyPI on tag

**No incluye:**
- CLI Python (foco Go CLI)
- ORM-like abstractions; mantener thin

## Enfoque técnico

1. `openapi-python-client` v0.21+ con custom templates Jinja
2. Spec generada en Go: `swag init` o `huma`/`fuego` que generan OpenAPI desde tipos Go
3. Lock SDK version a major del API
4. CI matrix Python 3.10–3.12

## Riesgos

- OpenAPI spec parcial → SDK no cubre todo → policy: spec es source of truth
- Breaking changes Python: minor versions del SDK reflejan minor del API
- Tamaño SDK generado grande → tree-shake con `__all__` explícito

## Testing

- pytest sobre client contra `httpx.MockTransport`
- Integration contra Domain dev compose
- Mypy strict over generated code
