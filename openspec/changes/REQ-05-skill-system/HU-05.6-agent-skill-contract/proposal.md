# Proposal: HU-05.6-agent-skill-contract

## Intención

Formalizar el contrato entre Agent y Skill: JSON Schema para inputs/outputs, traducción a tool-calling format de cada provider LLM, taxonomía de errores tipados, timeouts, idempotency hints y skill-to-skill invocation.

## Scope

**Incluye:**
- Schemas input/output Draft 2020-12 validados al register
- Adapters tool-calling para Anthropic, OpenAI, Gemini, Ollama (function calling)
- Error taxonomy 8 categorías + retry policy por categoría
- Defaults aplicados antes de validar
- Timeout configurable por skill
- Flag `idempotent` por skill
- `ctx.InvokeSkill(slug, input)` para skill-to-skill
- Audit trail de invocation chain

**No incluye:**
- Auto-generación de schemas desde código (futuro)
- Schema migration entre versiones de skill (HU-05.3 cubre versionado)
- Marketplace de skills (HU futura)

## Enfoque técnico

1. Lib `github.com/santhosh-tekuri/jsonschema/v5` para validation
2. Translation layer en `internal/llm/toolcalling/` por provider
3. Error wrapping con `errors.As` para extraer code
4. Timeout via `context.WithTimeout` propagado a la goroutine de skill
5. InvokeSkill helper que respeta auth + audit del agent padre

## Riesgos

- Schemas malformados rompen agents en runtime → validate al register
- Provider format drift (cambios upstream) → tests con fixtures reales
- Skill chain infinito → max depth 5 + detección por chain_id

## Testing

- Schema válido/inválido en register
- Defaults aplicados pre-validation
- Translation por provider con fixture LLM responses
- 8 error categories propagadas correctamente
- Retry sólo cuando aplica
- Skill-to-skill respeta auth + chain
- Timeout cancela goroutine
- Max depth chain → error
