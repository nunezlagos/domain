# Design: HU-05.6-agent-skill-contract

## Decisión arquitectónica

**JSON Schema:** Draft 2020-12 (estándar moderno, compatible con OpenAI/Anthropic tools).
**Validator:** `santhosh-tekuri/jsonschema/v5` (rápido, mantenido).
**Errores:** type-safe Go errors con `errors.As` para extracción.
**Timeout:** `context.WithTimeout` propagado.

## Schema diff

```sql
ALTER TABLE skills
  ADD COLUMN input_schema JSONB NOT NULL DEFAULT '{}',
  ADD COLUMN output_schema JSONB NOT NULL DEFAULT '{}',
  ADD COLUMN timeout_seconds INT DEFAULT 30 CHECK (timeout_seconds BETWEEN 1 AND 600),
  ADD COLUMN idempotent BOOLEAN DEFAULT false,
  ADD COLUMN depends_on TEXT[] DEFAULT '{}';

ALTER TABLE skill_versions
  ADD COLUMN input_schema JSONB,
  ADD COLUMN output_schema JSONB;
```

## Error taxonomy (Go)

```go
type SkillErrorCode string
const (
  ErrInvalidInput     SkillErrorCode = "InvalidInput"
  ErrInvalidOutput    SkillErrorCode = "InvalidOutput"
  ErrNotAuthorized    SkillErrorCode = "NotAuthorized"
  ErrNotFound         SkillErrorCode = "NotFound"
  ErrRateLimited      SkillErrorCode = "RateLimited"
  ErrTimeout          SkillErrorCode = "Timeout"
  ErrDependencyFailed SkillErrorCode = "DependencyFailed"
  ErrInternalError    SkillErrorCode = "InternalError"
)

type SkillError struct {
  Code     SkillErrorCode
  Message  string
  Retry    bool
  Cause    error
}
```

## Translation layer

```
internal/llm/toolcalling/
  anthropic.go    # ToAnthropicTool(skill) anthropic.Tool
  openai.go       # ToOpenAITool(skill) openai.Tool
  gemini.go       # ToGeminiTool(skill) gemini.Tool
  ollama.go       # ToOllamaTool(skill) ollama.Tool
```

## Skill execution context

```go
type ExecContext interface {
  context.Context
  AgentRunID() string
  ChainDepth() int
  ChainID() string
  InvokeSkill(slug string, input map[string]any) (map[string]any, error)
  Logger() *slog.Logger
}
```

## TDD plan

1. Schema válido pasa register; inválido 422
2. Defaults aplicados antes de validate
3. Anthropic/OpenAI/Gemini translation fixtures
4. Cada error code se mapea correctamente
5. Retry only para Retry=true
6. Skill A → invoke B → respeta auth + chain audit
7. Max depth 5 → error
8. Timeout: skill loop infinito → cancelled en N seg
9. Sabotaje: schema con `$ref` externo → reject (security)
