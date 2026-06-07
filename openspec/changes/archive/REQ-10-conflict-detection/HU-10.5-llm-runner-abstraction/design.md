# Design: HU-10.5-llm-runner-abstraction

## Decisión arquitectónica

### SemanticRunner interface

```go
// internal/conflict/runner/interface.go
package runner

import "context"

type ConflictVeredict struct {
    Relation       string  `json:"relation"`        // supersedes, conflicts_with, duplicate, unrelated
    Confidence     float64 `json:"confidence"`       // 0.0 - 1.0
    Reasoning      string  `json:"reasoning,omitempty"`
}

type CostEstimate struct {
    Model       string  `json:"model"`
    InputTokens int     `json:"input_tokens"`
    CostUSD     float64 `json:"cost_usd"`
    Currency    string  `json:"currency"` // "USD"
}

type SemanticRunner interface {
    Compare(ctx context.Context, obsA, obsB ObservationInput) (ConflictVeredict, error)
    EstimateCost(inputTokens int) CostEstimate
    ModelName() string
}

type ObservationInput struct {
    ID      int64  `json:"id"`
    Title   string `json:"title"`
    Content string `json:"content"`
    Tags    string `json:"tags,omitempty"`
}
```

### Factory

```go
// internal/conflict/runner/factory.go
const (
    AgentCLIEnv     = "ENGRAM_AGENT_CLI"
    DefaultTimeout  = 30 * time.Second
)

type RunnerConfig struct {
    Timeout    time.Duration
    MaxRetries int
}

func NewRunner(agentCLI string, config RunnerConfig) (SemanticRunner, error) {
    switch strings.ToLower(agentCLI) {
    case "claude":
        return NewClaudeRunner(config), nil
    case "opencode", "":
        return NewOpenCodeRunner(config), nil
    default:
        return nil, fmt.Errorf("unsupported agent CLI: %s", agentCLI)
    }
}
```

### ClaudeRunner

```go
// internal/conflict/runner/claude.go
type ClaudeRunner struct {
    config  RunnerConfig
    client  *http.Client
    apiKey  string
}

func NewClaudeRunner(config RunnerConfig) *ClaudeRunner {
    return &ClaudeRunner{
        config: config,
        client: &http.Client{Timeout: config.Timeout},
        apiKey: os.Getenv("ANTHROPIC_API_KEY"),
    }
}

func (r *ClaudeRunner) Compare(ctx context.Context, obsA, obsB ObservationInput) (ConflictVeredict, error) {
    if r.apiKey == "" {
        return ConflictVeredict{}, errors.New("Claude API key not configured")
    }

    ctx, cancel := context.WithTimeout(ctx, r.config.Timeout)
    defer cancel()

    prompt := BuildComparePrompt(obsA, obsB)
    // POST https://api.anthropic.com/v1/messages
    // Parse response into ConflictVeredict
}

func (r *ClaudeRunner) EstimateCost(tokens int) CostEstimate {
    // Claude Sonnet 4: $3/MTok input, $15/MTok output
    inputCost := float64(tokens) * 3.0 / 1_000_000
    return CostEstimate{
        Model: "claude-sonnet-4", InputTokens: tokens,
        CostUSD: inputCost, Currency: "USD",
    }
}

func (r *ClaudeRunner) ModelName() string { return "claude-sonnet-4" }
```

### OpenCodeRunner

```go
// internal/conflict/runner/opencode.go
type OpenCodeRunner struct {
    config RunnerConfig
}

func NewOpenCodeRunner(config RunnerConfig) *OpenCodeRunner {
    return &OpenCodeRunner{config: config}
}

func (r *OpenCodeRunner) Compare(ctx context.Context, obsA, obsB ObservationInput) (ConflictVeredict, error) {
    ctx, cancel := context.WithTimeout(ctx, r.config.Timeout)
    defer cancel()

    prompt := BuildComparePrompt(obsA, obsB)
    // Exec `opencode --json "prompt"` as subprocess
    // Parse JSON output into ConflictVeredict
}

func (r *OpenCodeRunner) EstimateCost(tokens int) CostEstimate {
    return CostEstimate{
        Model: "opencode-default", InputTokens: tokens,
        CostUSD: 0, Currency: "USD",
    }
}

func (r *OpenCodeRunner) ModelName() string { return "opencode-default" }
```

### PromptBuilder

```go
// internal/conflict/runner/prompt.go
const comparePromptTemplate = `You are a semantic conflict detector for a memory store.
Analyze whether these two observations are in conflict, or if one supersedes the other.

Observation A (ID: %d):
Title: %s
Content: %s

Observation B (ID: %d):
Title: %s
Content: %s

Respond with ONLY a JSON object:
{
  "relation": "supersedes" | "conflicts_with" | "duplicate" | "unrelated",
  "confidence": <0.0-1.0>,
  "reasoning": "<brief justification>"
}`

func BuildComparePrompt(obsA, obsB ObservationInput) string {
    return fmt.Sprintf(comparePromptTemplate,
        obsA.ID, obsA.Title, obsA.Content,
        obsB.ID, obsB.Title, obsB.Content,
    )
}
```

### Cost estimation batch

```go
// internal/conflict/runner/cost.go
func (r *ClaudeRunner) EstimateBatchCost(count, avgTokens int) CostEstimate {
    single := r.EstimateCost(avgTokens)
    return CostEstimate{
        Model: single.Model,
        InputTokens: count * avgTokens,
        CostUSD: single.CostUSD * float64(count),
        Currency: "USD",
    }
}
```

### OpenCode subprocess execution

```go
// internal/conflict/runner/exec.go
func runOpenCode(ctx context.Context, prompt string, timeout time.Duration) ([]byte, error) {
    ctx, cancel := context.WithTimeout(ctx, timeout)
    defer cancel()

    cmd := exec.CommandContext(ctx, "opencode", "--json", prompt)
    output, err := cmd.Output()
    if err != nil {
        if ctx.Err() == context.DeadlineExceeded {
            return nil, fmt.Errorf("OpenCode runner timeout after %v", timeout)
        }
        return nil, fmt.Errorf("OpenCode execution failed: %w", err)
    }
    return output, nil
}
```

## Alternativas descartadas

| Alternativa | Razón de descarte |
|-------------|-------------------|
| Single LLM provider | Vendor lock-in; factory pattern permite cambiar sin modificar judge |
| OpenAI como default | OpenCode es gratuito y ya está en el ecosistema; Claude opcional para más precisión |
| LLM cache en esta HU | Será futura optimización; primero funcionalidad core |
| gRPC para runner communication | Overkill para dos runners locales; CLI y HTTP son suficientes |

## TDD plan

1. **Red:** NewRunner("claude") retorna ClaudeRunner → falla
2. **Green:** Implement factory → pasa
3. **Red:** NewRunner("opencode") retorna OpenCodeRunner → falla
4. **Green:** Implement OpenCodeRunner → pasa
5. **Red:** NewRunner("invalid") retorna error → falla
6. **Green:** Implement validación → pasa
7. **Red:** ClaudeRunner.EstimateCost(1000) retorna > 0 → falla
8. **Green:** Implement cost formula → pasa
9. **Red:** Compare() con timeout cancelado retorna error → falla
10. **Green:** Implement context cancellation check → pasa
11. **Sabotaje:** No chequear API key → ClaudeRunner.Compare() crash en vez de error graceful → test cae → restaurar

## Riesgos y mitigación

| Riesgo | Mitigación |
|--------|------------|
| OpenCode no instalado | Error claro "opencode not found in PATH" |
| Claude API key rotada | Error "Claude API key not configured"; validación en startup |
| Timeout muy agresivo | Default 30s; configurable via RunnerConfig |
| Cost estimation desactualizada | Precios hardcodeados con TODO para actualizar; warning si diff > 10% |
