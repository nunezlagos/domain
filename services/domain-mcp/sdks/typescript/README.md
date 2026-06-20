# Domain SDK TypeScript

Cliente oficial para [Domain](https://github.com/nunezlagos/domain) — memoria + orquestación AI agents.

## Instalación

```bash
npm install @domain/sdk
# o
pnpm add @domain/sdk
```

Requiere Node 20+ (usa `fetch` built-in) o cualquier runtime con `fetch` global.

## Quick start

```typescript
import { DomainClient } from "@domain/sdk";

const client = new DomainClient({
  apiKey: process.env.DOMAIN_API_KEY!,
  baseUrl: "https://api.domain.sh",
});

// Crear project
const proj = await client.projects.create({
  name: "Demo",
  slug: "demo",
});

// Guardar observación
const obs = await client.observations.save({
  project_slug: "demo",
  content: "Decidimos usar pgvector con embeddings.",
  tags: ["arch", "db"],
});

// Búsqueda global
const results = await client.search.global({
  query: "pgvector",
  limit: 10,
});

for (const r of results) {
  console.log(`[${r.entity_type}] ${r.snippet} (score=${r.score})`);
}

// Ejecutar agent
const run = await client.agents.run("<agent-uuid>", "Revisá este PR");
console.log(run.output);
```

## Configuración

| Opción | Env var | Default |
|--------|---------|---------|
| `apiKey` | `DOMAIN_API_KEY` | requerido |
| `baseUrl` | `DOMAIN_BASE_URL` | `http://localhost:8000` |
| `timeoutMs` | — | `30000` |
| `fetchImpl` | — | `globalThis.fetch` |

## Manejo de errores

```typescript
import {
  AuthError,
  NotFoundError,
  ValidationError,
  QuotaExceededError,
} from "@domain/sdk";

try {
  await client.observations.save({ project_slug: "demo", content: "" });
} catch (e) {
  if (e instanceof ValidationError) {
    for (const d of e.details) {
      console.error(`${d.field}: ${d.message}`);
    }
  } else if (e instanceof QuotaExceededError) {
    console.error(`Quota: ${e.message}`);
  }
}
```

## Resources

- `client.organizations` — orgs CRUD + members
- `client.projects` — projects CRUD
- `client.observations` — save/get/list/delete
- `client.sessions` — start/end/active
- `client.search` — global cross-entity
- `client.skills` — list/create
- `client.agents` — list/get/run + runLogs
- `client.flows` — list/create/run
- `client.knowledge` — save/search

## Edge runtimes

Funciona en Cloudflare Workers, Deno, Bun (cualquier runtime con `fetch` y
`AbortController`). Pasa un `fetchImpl` custom si tu runtime no expone fetch global.

## Testing

```bash
cd sdks/typescript
npm install
npm test
```

## Licencia

Proprietary — ver root LICENSE.
