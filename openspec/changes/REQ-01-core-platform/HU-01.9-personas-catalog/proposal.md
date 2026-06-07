# Proposal: HU-01.9-personas-catalog

## Intención

Catálogo formal de 10 personas (actores) que sirve Domain, persistido en BD como source of truth (igual que platform_policies HU-01.8), expuesto via MCP tools para agentes IA, y referenciado por TODAS las HUs en su header.

## Scope

**Incluye:**
- Tabla `personas` con schema completo
- 10 personas seedeadas via HU-01.7 desde `seeds/personas/*.yaml`
- MCP tools `domain_persona_get(slug)` y `domain_persona_list()`
- CLI `domain personas import-md|export-md`
- Endpoint admin REST CRUD
- Linter extensión: cada hu.md debe declarar `**Persona:** <slug>` (uno o más)
- Cross-reference endpoint: persona → HUs
- Retrofit policy con baseline file

**No incluye:**
- Custom personas per-org (futuro, Enterprise)
- Persona journey maps detallados (futuro, design docs separados)
- Asociar usuarios reales (users tabla) con personas (eso es RBAC + behavior tracking, futuro)

## Enfoque técnico

1. Schema similar a platform_policies (body_md + structured)
2. Linter validation: parser hu.md detecta `**Persona:** xxx`, valida slug existe en BD
3. Retrofit: script Go que infiere persona desde keywords en hu.md content y agrega field
4. CLI export-md genera doc Markdown bonito desde BD (templates Go)

## Riesgos

- Persona inferida mal en retrofit: spot-check 10% + manual review
- Drift BD vs md: same que HU-01.8, workflow documentado
- Cap proliferación: max 30 custom personas per-org

## Testing

- CRUD persona
- Seed inicial 10 personas
- MCP tool get + list (con cache HU-12.6)
- Export md round-trip idempotent
- Import md parser
- Linter: HU sin field → fail; slug inexistente → fail
- Cross-reference endpoint
- Retrofit script muestras
