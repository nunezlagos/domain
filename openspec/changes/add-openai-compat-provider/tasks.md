# add-openai-compat-provider — Tasks

## Implementación

- [ ] code: agregar `NewWithBaseURL(apiKey, baseURL, model)` a `openai/provider.go` (override BaseURL/Model si no vacío) — debe pasar TestOpenAICompat_Complete_CustomBaseURL
- [ ] code: crear `openai/compat.go` con struct de config + `RegisterOpenAICompat(factory, wrap)` (parse env JSON, resolver api_key_env, registrar con wrap, parse tolerante)
- [ ] code: invocar `RegisterOpenAICompat(factory, wrapLLM)` en `buildLLMFactory` (server_services.go), sin tocar bloques existentes

## Tests

- [ ] tests: escribir los 5 tests en `openai/compat_test.go` (AC1–AC5)

## Sabotage

- [ ] sabotage: aplicar los 5 sabotajes del design y confirmar que cada test los atrapa; restaurar

## Verify

- [ ] verify: auditar change — ningún archivo nuevo >150 líneas, sin secrets hardcodeados, api_key no logueada, providers existentes intactos, `go test ./internal/llm/...` verde
