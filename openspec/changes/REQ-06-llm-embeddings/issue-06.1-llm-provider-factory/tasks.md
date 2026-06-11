# Tasks: issue-06.1-llm-provider-factory

## Backend

- [x] Definir interface `Provider` con métodos Complete, CompleteStream, Name, Models
- [x] Definir structs: CompletionOpts, Response, StreamChunk, TokenUsage
- [x] Implementar ProviderRegistry con Register, Get, GetDefault, List
- [x] Implementar inicialización de provider desde env vars
- [x] Implementar lazy initialization de providers
- [x] Implementar thread-safety con sync.RWMutex
- [x] Escribir .env.example con vars de configuración

## Frontend

- [x] N/A

## Tests

- [x] Test unitario: Register + Get de mock provider
- [x] Test unitario: Get de provider no registrado → error
- [x] Test unitario: GetDefault lee DOMAIN_LLM_PROVIDER
- [x] Test unitario: concurrencia con -race flag
- [x] Test unitario: Complete interface funciona con mock
- [x] Test unitario: CompleteStream interface funciona con mock
- [x] Sabotaje: panic en provider no propaga

## Cierre

- [x] Verificación con -race flag
- [x] Suite verde
