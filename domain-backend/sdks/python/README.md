# Domain SDK Python

Cliente oficial para [Domain](https://github.com/nunezlagos/domain) — plataforma
de memoria + orquestación de agents IA.

## Instalación

```bash
pip install domain-sdk
```

Requiere Python 3.10+.

## Quick start

```python
import asyncio
from domain_sdk import DomainClient

async def main():
    async with DomainClient(api_key="domk_live_...") as client:
        # Crear project
        proj = await client.projects.create(name="Demo", slug="demo")

        # Guardar observación
        obs = await client.observations.save(
            project_slug="demo",
            content="Decidimos usar pgvector con embeddings.",
            tags=["arch", "db"],
        )

        # Búsqueda global
        results = await client.search.global_(query="pgvector")
        for r in results:
            print(f"[{r.entity_type}] {r.snippet} (score={r.score})")

        # Ejecutar agent
        run = await client.agents.run(
            agent_id="<agent-uuid>",
            input="Revisá este PR",
        )
        print(f"Agent says: {run.output}")

asyncio.run(main())
```

## Configuración

| Arg | Env var | Default |
|-----|---------|---------|
| `api_key` | `DOMAIN_API_KEY` | requerido |
| `base_url` | `DOMAIN_BASE_URL` | `http://localhost:8000` |

## Manejo de errores

```python
from domain_sdk import AuthError, NotFoundError, ValidationError, QuotaExceededError

try:
    await client.observations.save(project_slug="x", content="")
except ValidationError as e:
    for d in e.details:
        print(f"{d['field']}: {d['message']}")
except QuotaExceededError as e:
    print(f"Quota: {e.message}")
```

## Resources disponibles

- `client.organizations` — orgs CRUD + members
- `client.projects` — projects CRUD
- `client.observations` — save/get/list/delete + búsqueda
- `client.sessions` — start/end/active
- `client.search` — global cross-entity
- `client.skills` — list/create
- `client.agents` — list/get/run + logs
- `client.flows` — list/run
- `client.knowledge` — save/search

## Testing

```bash
cd sdks/python
pip install -e ".[dev]"
pytest
```

## Licencia

Proprietary — ver root LICENSE.
