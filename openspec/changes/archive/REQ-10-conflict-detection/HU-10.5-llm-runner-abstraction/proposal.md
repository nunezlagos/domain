# Proposal: HU-10.5-llm-runner-abstraction

## Intención

Crear una abstracción de LLM runner con factory pattern que permita al semantic conflict judge (HU-10.2) usar Claude o OpenCode intercambiablemente. Incluye cost estimation, prompt builder y manejo de timeouts.

## Scope

**Incluye:**
- `SemanticRunner` interface con `Compare()`, `EstimateCost()`, `ModelName()`
- Factory `NewRunner(agentCLI string)` basada en `ENGRAM_AGENT_CLI`
- `ClaudeRunner` — ejecuta comparaciones via API de Anthropic
- `OpenCodeRunner` — ejecuta comparaciones via CLI `opencode`
- `PromptBuilder` — genera prompts estructurados para comparación semántica
- `CostEstimator` — estimación por modelo y por batch
- Timeout configurable por operación
- Validación de ENGRAM_AGENT_CLI inválido

**No incluye:**
- El semantic conflict judge en sí (HU-10.2)
- Cache de respuestas LLM (futuro)
- Soporte para otros proveedores (OpenAI, Gemini) — extensible via interface

## Enfoque técnico

| Aspecto | Decisión |
|---------|----------|
| Interface | `SemanticRunner { Compare(ctx, obsA, obsB) → ConflictVeredict, error; EstimateCost(tokens) → CostEstimate; ModelName() → string }` |
| Factory | `NewRunner(agentCLI string) (SemanticRunner, error)` — switch case simple |
| ClaudeRunner | HTTP client a Anthropic API; requiere ANTHROPIC_API_KEY |
| OpenCodeRunner | Ejecuta `opencode ...` como subprocess; parsea stdout |
| Prompt | Template: "Compare these two observations for semantic conflict. Observation A: ... Observation B: ..." |
| Timeout | `context.WithTimeout(ctx, runner.timeout)` en cada Compare() |
