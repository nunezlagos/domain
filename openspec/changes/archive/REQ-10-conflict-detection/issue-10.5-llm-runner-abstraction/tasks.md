# Tasks: issue-10.5-llm-runner-abstraction

## Backend

- [ ] **B1: Definir SemanticRunner interface y tipos**
      - `internal/conflict/runner/interface.go`
      - ConflictVeredict, CostEstimate, ObservationInput, SemanticRunner

- [ ] **B2: Implementar Factory**
      - `internal/conflict/runner/factory.go`
      - NewRunner(agentCLI, config) con switch case
      - Default a OpenCodeRunner si ENGRAM_AGENT_CLI vacío
      - Error si CLI inválido

- [ ] **B3: Implementar ClaudeRunner**
      - `internal/conflict/runner/claude.go`
      - HTTP client a Anthropic Messages API
      - Parseo de respuesta JSON
      - Validación de ANTHROPIC_API_KEY

- [ ] **B4: Implementar OpenCodeRunner**
      - `internal/conflict/runner/opencode.go`
      - Subprocess exec con exec.CommandContext
      - Parseo de stdout JSON
      - Manejo de timeout vía context

- [ ] **B5: Implementar PromptBuilder**
      - `internal/conflict/runner/prompt.go`
      - BuildComparePrompt() con template estructurado
      - Schema de respuesta esperada en el prompt

- [ ] **B6: Implementar CostEstimator**
      - `internal/conflict/runner/cost.go`
      - EstimateCost() por modelo
      - EstimateBatchCost() para N operaciones
      - Precios hardcodeados con comentario de fuente

- [ ] **B7: Implementar RunnerConfig con timeout default**
      - `internal/conflict/runner/config.go`
      - DefaultTimeout = 30s
      - MaxRetries = 0 (por ahora, sin retry)

## Tests

- [ ] **T1: Factory crea ClaudeRunner con "claude"**
- [ ] **T2: Factory crea OpenCodeRunner con "opencode"**
- [ ] **T3: Factory crea OpenCodeRunner con string vacío**
- [ ] **T4: Factory retorna error con CLI inválido**
- [ ] **T5: ClaudeRunner.EstimateCost(1000) > 0 (Claude cuesta)**
- [ ] **T6: OpenCodeRunner.EstimateCost(1000) == 0 (gratuito)**
- [ ] **T7: ClaudeRunner.ModelName() == "claude-sonnet-4"**
- [ ] **T8: OpenCodeRunner.ModelName() == "opencode-default"**
- [ ] **T9: Compare() con contexto cancelado retorna error de timeout**
- [ ] **T10: ClaudeRunner sin API key retorna error claro**
- [ ] **T11: PromptBuilder genera output con obsA y obsB**
- [ ] **T12: Sabotaje — no validar API key → panic en vez de error → test cae → restaurar**

## Cierre

- [ ] `go build ./...` sin errores
- [ ] `go vet ./...` sin warnings
- [ ] `go test ./internal/conflict/runner/... -v`
- [ ] Commit: `feat: LLM runner abstraction with factory pattern`
