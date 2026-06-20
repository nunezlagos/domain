# RFC 0005: SDD (REQ-04) vs Knowledge Docs (REQ-03) Relationship

**Status:** accepted
**Date:** 2026-06-07
**Related:** REQ-03 Memory System (issue-03.4 knowledge-documents), REQ-04 OPSX SDD

## Contexto

REQ-04 (OPSX SDD) modela artifacts: REQs, HUs (con Gherkin), specs, designs (ADRs), tasks.
REQ-03 issue-03.4 modela knowledge_docs: markdown documents con chunking, embeddings, búsqueda híbrida.

Conceptualmente, los artifacts SDD **son** knowledge documents especializados con schema fijo. Riesgo de duplicación de:
- Storage layer
- Búsqueda
- Versionado
- Embeddings
- Tagging

## Decisión

**SDD artifacts son una capa especializada SOBRE knowledge_docs**, no tablas paralelas.

### Modelo unificado

```
knowledge_docs (base storage)
   ├─ kind = "free"          → markdown libre (notas, runbooks, docs)
   ├─ kind = "sdd_req"       → schema REQ
   ├─ kind = "sdd_hu"        → schema HU con Gherkin
   ├─ kind = "sdd_spec"      → schema spec
   ├─ kind = "sdd_design"    → schema ADR
   ├─ kind = "sdd_tasks"     → schema tasks
   └─ kind = "sdd_state"     → schema state machine
```

### Schema

```sql
ALTER TABLE knowledge_docs
  ADD COLUMN kind VARCHAR(50) NOT NULL DEFAULT 'free',
  ADD COLUMN structured_body JSONB,    -- para sdd_* kinds, body validado
  ADD COLUMN parent_doc_id UUID REFERENCES knowledge_docs(id),
  ADD COLUMN sdd_slug VARCHAR(100);    -- "REQ-01", "issue-01.1"

CREATE INDEX ON knowledge_docs (kind, sdd_slug);
CREATE INDEX ON knowledge_docs (parent_doc_id);
```

### Jerarquía SDD via parent_doc_id

```
REQ-01 (kind=sdd_req)
  └─ issue-01.1 (kind=sdd_hu, parent=REQ-01)
      ├─ issue-01.1 / hu.md      (kind=sdd_hu, structured)
      ├─ issue-01.1 / proposal   (kind=sdd_spec)
      ├─ issue-01.1 / design     (kind=sdd_design)
      ├─ issue-01.1 / tasks      (kind=sdd_tasks)
      └─ issue-01.1 / state      (kind=sdd_state)
```

### Validación per kind

- `kind = "sdd_req"`: schema JSON con `slug, name, criterios_exito, hus_hijas[]`
- `kind = "sdd_hu"`: schema con `slug, origen, prioridad, tipo, escenarios_gherkin[]`
- etc.

Schemas en `internal/sdd/schemas/` versionados como migraciones de schema.

### Búsqueda

Una sola búsqueda híbrida (issue-03.7) cubre todos:

```sql
GET /api/v1/search?q=migration&kind=sdd_hu        -- solo HUs
GET /api/v1/search?q=migration                    -- todo incluido SDD
GET /api/v1/search?q=migration&kind=free          -- excluir SDD
```

### Versionado

- knowledge_docs ya tiene versioning (issue-03.4 implícito, formalizar si no está)
- SDD artifacts heredan versioning
- Pública: `GET /sdd/req/:slug/versions`

### Embeddings

- Computados para todos los kinds (incluso SDD)
- SDD docs son highly embedded en context windows de agentes que diseñan features

### Endpoints específicos SDD

- `GET /api/v1/sdd/reqs` (lista, filtros por state)
- `GET /api/v1/sdd/reqs/:slug`
- `GET /api/v1/sdd/reqs/:slug/hus`
- `POST /api/v1/sdd/reqs` (crea kind=sdd_req con validation)
- `GET /api/v1/sdd/hus/:slug/tasks`
- `PATCH /api/v1/sdd/hus/:slug/state` (transition)

Internamente delegan a knowledge_docs storage.

## Alternativas consideradas

### Alternativa A: tablas separadas para SDD (rechazada)

`requirements`, `issues`, `specs`, `designs`, `tasks_table`, `state_machines`.

Rechazada:
- Duplica búsqueda y embeddings
- 6 tablas con schemas similares
- Cross-search (free docs + SDD) imposible sin UNION

### Alternativa B: SDD como capa externa que no usa knowledge_docs (rechazada)

Filesystem-based (como `openspec/changes/`) sin DB.

Rechazada:
- No search server-side
- No multi-user collaboration
- No API
- Pierde reuse de RBAC/auth/audit

### Alternativa C: NoSQL / document store dedicado (rechazada)

MongoDB / DocumentDB para SDD.

Rechazada: rompe Postgres-only.

## Migración filesystem → DB

Hoy `openspec/changes/REQ-*/issue-*/*.md` viven en disco. Estrategia de migración:

1. Importer one-shot lee filesystem y crea knowledge_docs con `kind=sdd_*`
2. Hook git en repo (opcional): cambios en `openspec/` se sincronizan a DB
3. Endpoint `/sdd/sync` para resync manual
4. Filesystem queda como "source of truth para repo Domain mismo"; DB es source para usuarios finales de la plataforma

## Validación de cambios cross-document

Algunas validaciones requieren chequear varios documentos (ej: HU declara origen REQ-X que debe existir):

- Validación en service layer al crear/editar
- Endpoint `/sdd/validate` que recorre todo y reporta inconsistencies
- Linter test en CI valida filesystem version

## Consecuencias

**Positivas:**
- Una sola search (búsqueda global cubre todo)
- Reuse de embeddings, versioning, RBAC
- Storage layer único
- Cross-references SDD ↔ knowledge libre

**Negativas:**
- `knowledge_docs` table tiene schema más complejo (kind, structured_body)
- Validation per-kind aumenta complejidad service layer

## Implementación

- Agregar columnas a knowledge_docs en migración (issue-03.4 ya base)
- Definir schemas JSON per kind en `internal/sdd/schemas/`
- Service `internal/service/sdd/` con CRUD validation
- Handlers /api/v1/sdd/* delegando a knowledge_docs
- issue-03.7 global search ya cubre todos los kinds

## Open questions

- ¿"State machine" como kind separado o columna en sdd_hu? — Yo voto columna en sdd_hu para evitar fragmentar
- ¿Adjuntos S3 (issue-04.6) atados a knowledge_docs.id genérico? Sí, polimórfico funciona
- ¿Versionado de SDD requiere approval workflow? No en MVP; futuro como state machine adicional
