# Proposal: HU-06.1-llm-provider-factory

## Intención

Implementar la abstracción base del sistema LLM: una interfaz `Provider` común para todos los proveedores, un registry thread-safe para registrar/obtener providers por nombre, y una factory que lee la configuración desde env vars para inicializar el provider por defecto.

## Scope

**Incluye:**
- Interface `Provider` con métodos: `Complete`, `CompleteStream`, `Name`, `Models`
- Struct `CompletionOpts`: Model, Temperature, MaxTokens, TopP, Stop, FrequencyPenalty, PresencePenalty
- Struct `Response`: Content, Model, Usage {PromptTokens, CompletionTokens, TotalTokens}, FinishReason
- Struct `StreamChunk`: Content, Done, Usage (en último chunk)
- Struct `ProviderRegistry` con métodos: Register, Get, GetDefault, List
- Inicialización de providers desde env vars
- Thread-safety con sync.RWMutex
- Config: DOMAIN_LLM_PROVIDER, DOMAIN_OPENAI_KEY, DOMAIN_ANTHROPIC_KEY, DOMAIN_GOOGLE_KEY

**Excluye:**
- Implementaciones concretas (HU-06.2, HU-06.3)
- Embedding interface (HU-06.5)
- Cost tracking (HU-06.4)
- Token counting (HU-06.6)

## Enfoque técnico

- Interface `Provider` minimalista pero extensible. Usar opts pattern (functional options o struct) para evitar breaking changes al agregar params.
- Registry como singleton package-level con `sync.RWMutex`.
- Factory `NewProvider(name string, cfg Config) (Provider, error)` crea la instancia concreta.
- `GetDefault()` lee `DOMAIN_LLM_PROVIDER` e inicializa el provider correspondiente.
- Las API keys se leen de env vars al inicializar, no se almacenan en DB.

## Riesgos

- Provider interface demasiado acoplada a chat completion. Si necesitamos embedding, necesitamos otro método. Mitigación: mantener interface separada para embeddings (HU-06.5).
- API keys en env vars: Seguro para entorno server. Documentar en .env.example.

## Testing

- **Unitarios:** Registry thread-safety, Get/Register/GetDefault, configuración desde env vars mock.
- **Integración:** Mock provider que implementa la interfaz, verificar ciclo completo.
- **Sabotaje:** Registry concurrente con 100 goroutines no pierde datos.
