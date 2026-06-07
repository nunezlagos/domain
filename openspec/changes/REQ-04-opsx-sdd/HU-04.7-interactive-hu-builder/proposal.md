# Proposal: HU-04.7-interactive-hu-builder

## Intención

Wizard MCP-driven que clarifica el alcance de una HU/REQ/RFC antes de generarla, con 5 modos (feature/bug-fix/refactor/doc/rfc), preguntas adaptativas, validación de audience/REQ contra BD, y state persistente en `hu_drafts` para auditoría + reanudación.

## Scope

**Incluye:**
- Tabla `hu_drafts` + `hu_draft_steps_log`
- 6 MCP tools: start/answer/preview/commit/abandon/list
- Step flow declarativo per mode (8/6/5/3/7 pasos respectivamente)
- Validación audience contra audience text libre
- Validación REQ contra openspec/changes filesystem y BD (HU-04.1)
- Path inference + slug auto-numbering
- Gherkin scenarios skeleton generator (3-5 sugeridos)
- Pending clarifications flow para ambigüedades
- Cron purge drafts >7 días + expired >24h

**No incluye:**
- UI Web (postponer Fase 6 si REQ-16 lo cubre)
- Generación full LLM-driven sin preguntas (eso es el flujo libre actual; este wizard es el opuesto: dirigido)
- Edición de HUs existentes (solo creación; updates van por PRs normales)

## Enfoque técnico

1. State machine per mode con steps declarados en código
2. Cada step retorna `{prompt, options, validator_fn}`
3. Storage Postgres con JSONB para flexibilidad de answers
4. MCP tools delgados sobre `internal/sdd/wizard` service
5. Preview renderiza templates Go (hu.md.tmpl, etc.)
6. Commit escribe filesystem (los archivos siguen siendo source of truth para git) + audit_log

## Riesgos

- Wizard rígido frustra a power users: tener escape `domain hu create --skip-wizard` para flujo libre
- Drafts acumulándose: cron purge + max 5 drafts/user concurrentes
- Llamadas MCP fragmentadas (1 por step) → state cache vital
- Audience mal sugerida por filtro de REQ → permite "other audience" override

## Testing

- Crear draft mode=feature → recibe primera pregunta
- Respuestas avanzan steps secuenciales
- Validación slug audience inválido → error
- Path inference correcto basado en REQ
- Preview genera 5 archivos coherentes
- Commit escribe filesystem + marca status committed
- Draft expirado → 410
- Sabotaje: respuesta inválida → wizard NO avanza step
