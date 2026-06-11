# Tasks: issue-05.4-auto-skill-engine

> **Cierre por absorción 2026-06-11 (decisión MCP-first):** la capacidad
> core — "dado un contexto, recomendar skills relevantes" — la entrega
> `skill.Service.SearchHybrid` (BM25 + pgvector cosine + RRF fusion,
> issue-05.2) expuesta como MCP tool `domain_skill_search` y
> GET /api/v1/skills/search. El agente pasa su contexto como query y recibe
> skills rankeados por relevancia semántica. El orchestrator (issue-08.10)
> además recomienda skills por fase (orchestrator/skills.go).
> Un endpoint /recommend dedicado con batch, cache LRU de embeddings,
> truncado 512-tokens y threshold tuning queda DIFERIDO hasta demanda real.

## Backend

- [x] Recomendación por contexto → SearchHybrid RRF vía domain_skill_search + GET /skills/search
- [ ] POST /api/skills/recommend dedicado → DIFERIDO (search semántico cubre el caso de uso)
- [ ] POST /recommend/batch → DIFERIDO
- [x] Embedding del contexto → Embedder inyectado en skill.Service (FakeEmbedder en tests, providers reales en prod)
- [x] Query pgvector → cosine con scoping org (migration skills)
- [ ] Cache LRU embeddings → DIFERIDO (volumen actual no lo justifica)
- [ ] Truncado 512 tokens → DIFERIDO con el endpoint dedicado
- [x] Validaciones → q requerido, limit cap en handler/tool
- [x] Scores en respuesta → SearchResult con score RRF

## Tests

- [x] Search híbrido → TestSkill_SearchHybrid (datos seed + ranking)
- [ ] Cache LRU / batch / threshold → diferidos con sus features
- [x] Filtros org-scoped → suite skill service
- [ ] Sabotaje embedding timeout → diferido con endpoint dedicado (el tool MCP hereda retry/CB del ResilientWrapper issue-12.6)

## Cierre

- [x] Verificación → cubierta por tests + tool MCP (mismo código)
- [x] Suite verde → 2026-06-11
