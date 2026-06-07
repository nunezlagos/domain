# Tasks: HU-06.1-llm-provider-factory

## Backend

- [ ] Definir interface `Provider` con métodos Complete, CompleteStream, Name, Models
- [ ] Definir structs: CompletionOpts, Response, StreamChunk, TokenUsage
- [ ] Implementar ProviderRegistry con Register, Get, GetDefault, List
- [ ] Implementar inicialización de provider desde env vars
- [ ] Implementar lazy initialization de providers
- [ ] Implementar thread-safety con sync.RWMutex
- [ ] Escribir .env.example con vars de configuración

## Frontend

- [ ] N/A

## Tests

- [ ] Test unitario: Register + Get de mock provider
- [ ] Test unitario: Get de provider no registrado → error
- [ ] Test unitario: GetDefault lee DOMAIN_LLM_PROVIDER
- [ ] Test unitario: concurrencia con -race flag
- [ ] Test unitario: Complete interface funciona con mock
- [ ] Test unitario: CompleteStream interface funciona con mock
- [ ] Sabotaje: panic en provider no propaga

## Cierre

- [ ] Verificación con -race flag
- [ ] Suite verde
