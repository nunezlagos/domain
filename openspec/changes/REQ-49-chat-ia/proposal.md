# REQ-49 — Chat IA estilo NotebookLM sobre datos de domain-admin

> Diseñamos esta HU en 3 sub-HUs incrementales. Cada sub-HU es deployable
> independientemente y se valida contra su HU anterior.

## Motivacion

Los operadores del admin de total-domain hoy navegan entre 11+ mantenedores
(agentes, skills, flows, prompts, projects, etc.) para responder preguntas
tipo "¿cuantos agentes tenemos con provider anthropic?", "¿que skills usan
los proyectos del cliente X?", "¿cual fue el costo en tokens del modelo
claude-haiku la semana pasada?". Cada respuesta requiere abrir varias
pantallas + cruzar mentalmente la info.

Queremos una pestaña **Chat IA** estilo NotebookLM: el operador hace una
pregunta en lenguaje natural y el sistema responde citando las fuentes
(fila concreta de la tabla, con link al detalle). Stack:

- **LLM**: MiniMax (Claude-compatible, via API Anthropic-compatible).
- **RAG**: embeddings sobre las tablas reales del admin, almacenados en pgvector.
- **UI**: split view NotebookLM (sidebar conversaciones + main panel con burbujas).

## Decisiones de diseno

### 1. ¿Por que MiniMax (Anthropic-compatible) y no OpenAI?

- El proyecto ya usa `provider="anthropic"` como default en todos los agentes
  seed (ver `maintainers/agents/factories.py`).
- La SDK oficial `anthropic` permite custom `base_url` con un set minimisimo
  de cambios. Mismo patron que en `saargo_curriculum/backend/app/Services/LlmFactory.php`
  que ya tiene `AnthropicProvider` + `GeminiProvider`.
- Si en el futuro queremos cambiar de provider, solo tocamos una env var.

### 2. ¿Por que las conversaciones viven en domain-mcp (Go) y no en domain-admin (Django)?

Consistencia con el resto del proyecto:
- TODAS las tablas del dominio viven en domain-mcp (Go + Postgres). Django
  solo las lee/escribe via ORM `managed=False`.
- Si las conversaciones vivieran en domain-admin, rompiamos el patron y
  generabamos 2 conexiones a DB distintas.
- Asi, las migraciones DDL siguen en Go (golang-migrate) y Django solo
  agrega los models admin (read-only + write via ORM).

### 3. ¿Por que split view en lugar de chat modal?

- NotebookLM es el reference design que el usuario pidio explicitamente.
- Split view permite comparar conversaciones pasadas mientras trabajas.
- Sidebar persistente da acceso rapido a consultas recurrentes.

### 4. ¿Por que polling cada 1.5s y no SSE/streaming en MVP?

- Es la peticion explicita del usuario: "MVP funcional estilo NotebookLM".
- Polling es robusto (sin conexiones long-lived, sin problemas con Caddy
  buffering), facil de testear y suficiente para textos de <2K tokens.
- Streaming lo dejamos para una HU-49.4 futura cuando midamos latencia
  real.

### 5. ¿Por que vanilla JS en lugar de HTMX o React?

- El admin es Django server-side rendered. Meter React rompe el modelo
  mental (y mete npm/bundler).
- HTMX seria valido pero requiere partials + complejidad extra.
- Vanilla JS con `fetch` + `marked.js` (CDN) cubre el 100% del scope del MVP.
- Coherente con todos los demas JS del proyecto (`csrf.js`, `modals.js`,
  `sidebar.js`, etc).

## Arquitectura (alto nivel)

```
+-------------------+      +---------------------+      +------------------+
|   Browser (HTML)  | ---> | domain-admin (Djan) | ---> | domain-mcp (Go)  |
|  - chat.js        |      | - /chat/api/*       |      | - tables         |
|  - polling 1.5s   |      | - ChatService       |      | - conversations  |
|  - marked.js MD   |      | - RetrievalService  |      | - messages       |
+-------------------+      | - LlmProvider       |      | - embeddings     |
                           +----------+----------+      +------------------+
                                      |
                                      v
                            +------------------+
                            | MiniMax API      |
                            | (Claude-comp)    |
                            +------------------+
```

## Scope del MVP (NO exhaustivo)

**Incluye:**
- Provider MiniMax con SDK Anthropic + base_url custom (HU-49.1)
- Tablas conversations, messages, document_embeddings en domain-mcp (HU-49.2)
- RAG basico: cosine similarity >= 0.7, top 8 chunks (HU-49.2)
- Endpoints REST de chat estilo curriculum (HU-49.2)
- UI split view con sidebar + burbujas + source cards (HU-49.3)
- Polling 1.5s (HU-49.3)
- Markdown basico (HU-49.3)

**NO incluye (futuro):**
- Streaming SSE (HU-49.4)
- Tools (generar PDF, exportar Excel) — el proyecto no necesita CVs
- Multi-tenant: las conversaciones son por usuario (email) pero no hay RLS
  granular todavia (TODO HU-49.5)
- Compartir conversaciones por link
- Historial >30 dias (auto-cleanup en HU-49.5)

## Modelo de datos (resumen)

```sql
-- domain-mcp (Go) crea via golang-migrate:

CREATE TABLE conversations (
    id          UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    user_email  TEXT NOT NULL,
    title       TEXT NOT NULL DEFAULT '',
    created_at  TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at  TIMESTAMPTZ NOT NULL DEFAULT now(),
    deleted_at  TIMESTAMPTZ
);
CREATE INDEX idx_conversations_user ON conversations(user_email) WHERE deleted_at IS NULL;

CREATE TABLE messages (
    id              BIGSERIAL PRIMARY KEY,
    conversation_id UUID NOT NULL REFERENCES conversations(id) ON DELETE CASCADE,
    role            TEXT NOT NULL CHECK (role IN ('user', 'assistant')),
    content         TEXT,
    content_partial TEXT,
    status          TEXT NOT NULL DEFAULT 'pending'
                     CHECK (status IN ('pending', 'processing', 'completed', 'error')),
    sources         JSONB NOT NULL DEFAULT '[]'::jsonb,
    tokens_in       INTEGER NOT NULL DEFAULT 0,
    tokens_out      INTEGER NOT NULL DEFAULT 0,
    model           TEXT NOT NULL DEFAULT '',
    duration_ms     INTEGER NOT NULL DEFAULT 0,
    created_at      TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE INDEX idx_messages_conv ON messages(conversation_id, created_at DESC);

CREATE TABLE document_embeddings (
    id            BIGSERIAL PRIMARY KEY,
    source_table  TEXT NOT NULL,
    source_id     UUID NOT NULL,
    source_url    TEXT NOT NULL DEFAULT '',
    chunk_text    TEXT NOT NULL,
    chunk_index   INTEGER NOT NULL DEFAULT 0,
    embedding     vector(1536) NOT NULL,
    created_at    TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE INDEX idx_doc_emb_source ON document_embeddings(source_table, source_id);
CREATE INDEX idx_doc_emb_vec ON document_embeddings USING ivfflat (embedding vector_cosine_ops);
```

## Archivos a tocar (resumen)

| HU | Capa | Archivos |
|----|------|----------|
| 49.1 | provider | `app/core/llm/{provider,minimax_provider,anthropic_provider,factory,types}.py`, `requirements.txt`, `.env.example` |
| 49.2 | backend | `app/chat/{views,services,retrieval,prompts,models,urls,apps}.py`, `app/chat/migrations/0001_initial.py`, `config/urls.py`, `docker-compose.yml` |
| 49.3 | frontend | `app/templates/chat/*`, `app/static/{css/chat.css, js/chat.js}`, `components/_sidebar.html` |

## Riesgos

| Riesgo | Mitigacion |
|--------|------------|
| MiniMax cambia la API Anthropic | Atrasamos con SDK Anthropic oficial, solo cambia base_url |
| Embeddings de baja calidad | Filtramos por score >= 0.7, top-K configurable |
| Polling agresivo satura DB | Intervalo 1.5s solo mientras status sea pending/processing, polling se apaga solo |
| Credenciales en sources | Privacy guard en retrieval: blacklist explicita de columnas |
| Sin multi-tenant RLS | MVP single-org; HU-49.5 sumara FORCE RLS sobre conversaciones |

## Plan de implementacion

1. **HU-49.1** (provider): ~4 archivos Python + tests. Sin tocar DB ni templates.
2. **HU-49.2** (RAG + endpoints): ~10 archivos. Migraciones en domain-mcp (Go).
3. **HU-49.3** (UI): ~12 archivos templates/CSS/JS. Sin backend nuevo.

## Testing

- HU-49.1: tests unitarios del provider con mocks de transport HTTP
- HU-49.2: tests de integracion con DB real (postgres testcontainers)
- HU-49.3: tests manuales via navegador (este proyecto no tiene playwright)