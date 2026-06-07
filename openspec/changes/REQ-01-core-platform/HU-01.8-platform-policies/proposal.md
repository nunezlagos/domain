# Proposal: HU-01.8-platform-policies

## Intención

Tabla `platform_policies` como source-of-truth de TODAS las policies de la plataforma (DB conventions, API conventions, security, testing, observability, migrations, arquitectura). El binary trae el catalog inicial via seeders (HU-01.7) y la DB es la fuente runtime. CLI export-md regenera archivos para devs. MCP tools accesibles por agentes.

## Scope

**Incluye:**
- Tabla + CRUD + versionado + audit + rollback
- 2 MCP tools: `domain_policy_get`, `domain_policies_search`
- CLI `domain policies import-md|export-md`
- Structured parser (markdown → JSONB) para reglas máquina-leíbles
- Search FTS sobre body_md
- Integration con seeders (carga inicial)

**No incluye:**
- UI editor (futuro)
- ML auto-suggest de policies (futuro)

## Enfoque técnico

1. Body markdown raw + body_structured JSONB parseado
2. Versionado UPDATE → INSERT nueva + flip is_active
3. Markdown→structured parser con sections convention (heading `## Naming`, etc.) parsea tablas
4. CLI usa templates Go para regen md desde structured
5. MCP tool con timeout 2s + cache 5min in-proc (HU-12.6)

## Riesgos

- Drift dev local: documentar workflow — devs hacen edit md → `domain policies import-md` → commit; export-md es para sync inverso
- Structured parser frágil: fallback a body_md raw cuando parsing falla; structured solo populate cuando parser exitoso
- Linters dependen de structured: cuando structured null, linters usan defaults conservadores

## Testing

- CRUD + audit
- Versionado preserva históricas
- Import desde .md crea o updatea
- Export regenera md idempotent
- MCP tools con cache hit/miss
- Structured parser sobre fixtures md
- Rollback a versión previa
- Search FTS
