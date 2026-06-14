# Design: issue-06.1-llm-provider-factory

## Decisión arquitectónica

| Decisión | Opción elegida | Alternativa | Razón |
|----------|---------------|-------------|-------|
| Registry pattern | Package-level singleton | Inyección de dependencia | Simple, accesible desde toda la app |
| Thread-safety | sync.RWMutex | sync.Map | RWMutex permite lecturas concurrentes |
| Config pattern | Struct CompletionOpts | Functional options | Serializable, fácil de pasar por JSON |
| Provider init | Lazy on first Get() | Eager on startup | Evita errores de startup por API keys faltantes |

## Alternativas descartadas

- **Inyección de dependencia:** Correcto pero añade boilerplate. El singleton registry es aceptable para un provider LLM.
- **sync.Map:** No necesitamos las optimizaciones de sync.Map. RWMutex es más predecible.
- **Functional options:** Menos serializable. El struct es más simple y podemos pasarlo por HTTP/JSON.

## Diagrama

```
internal/llm/
├── provider.go          ← Interface + structs
├── factory.go           ← Registry + factory functions
├── opts.go              ← CompletionOpts
└── providers/
    ├── openai.go        ← issue-06.2
    ├── anthropic.go     ← issue-06.2
    ├── google.go        ← issue-06.2
    └── ollama.go        ← issue-06.3
```

### Interfaces

```go
type Provider interface {
    Name() string
    Complete(ctx context.Context, prompt string, opts CompletionOpts) (*Response, error)
    CompleteStream(ctx context.Context, prompt string, opts CompletionOpts) (<-chan StreamChunk, error)
    Models() []string
}

type CompletionOpts struct {
    Model            string
    Temperature      float32
    MaxTokens        int
    TopP             float32
    Stop             []string
    FrequencyPenalty float32
    PresencePenalty  float32
}

type Response struct {
    Content      string
    Model        string
    Usage        TokenUsage
    FinishReason string
}

type StreamChunk struct {
    Content string
    Done    bool
    Usage   TokenUsage  // solo en último chunk
}

type TokenUsage struct {
    PromptTokens     int
    CompletionTokens int
    TotalTokens      int
}
```

### Registry

```go
type ProviderRegistry struct {
    mu       sync.RWMutex
    providers map[string]Provider
    defaultKey string
}

var global = &ProviderRegistry{
    providers: make(map[string]Provider),
}

func Register(name string, p Provider) { ... }
func Get(name string) (Provider, error) { ... }
func GetDefault() (Provider, error) { ... }
func List() []string { ... }
```

## TDD plan

1. **TestRegisterAndGet:** Register mock → Get por nombre → misma instancia
2. **TestGetNoExistente:** Get de no registrado → error
3. **TestGetDefault:** Set DOMAIN_LLM_PROVIDER → GetDefault retorna ese provider
4. **TestGetDefaultSinConfig:** Sin DOMAIN_LLM_PROVIDER → error
5. **TestRegisterSobrescribe:** Register mismo nombre 2 veces → el último gana
6. **TestConcurrencia:** 100 goroutines leyendo Get → sin race conditions
7. **TestCompleteInterface:** Mock implementa Complete → response correcto
8. **TestCompleteStreamInterface:** Mock implementa CompleteStream → chunks correctos
9. **TestSabotaje:** Provider panic en Complete → recovery graceful

## Riesgos y mitigación

- **Race conditions:** RWMutex cubre escritura/lectura. Tests con -race flag.
- **API keys no configuradas:** Provider devuelve error en Complete, no en Register. Lazy init.
- **Interface breaking:** Al agregar métodos, mantener compatibilidad hacia atrás.
