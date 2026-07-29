# add-openai-compat-provider — Proposal

## Problema

Cada endpoint que habla el dialecto OpenAI (vLLM, Groq, Together, LM Studio) exige código/wiring propio porque `openai.New` hardcodea `api.openai.com` y no hay override de `base_url`. Ata el sistema a un endpoint fijo y bloquea el norte del epic DOMAINSERV-55 (no depender de un modelo).

## Intención

Agregar un provider OpenAI-compatible configurable por env que reusa el dialecto openai (`Complete`/`CompleteStream`) con `base_url` override, replicando el patrón MiniMax, para enchufar endpoints nuevos sin código.

## Scope in

- `internal/llm/openai/provider.go`: `NewWithBaseURL(apiKey, baseURL, model)`.
- `internal/llm/openai/compat.go` (nuevo): `RegisterOpenAICompat(factory, wrap)`.
- `cmd/domain/server_services.go`: invocar en `buildLLMFactory`.
- `internal/llm/openai/compat_test.go` (nuevo): tests httptest.

## Scope out

- roles config-driven (DOMAINSERV-57), failover (DOMAINSERV-59), embeddings (DOMAINSERV-60), migrar providers existentes.

## Riesgos

- Romper el registro de providers existentes → no tocar bloques previos; test de regresión.
- Fuga de `api_key` en logs → loguear solo name/base_url/model; test sabotaje.
- JSON malformado en env → parse tolerante (warning sin key + skip), sin abortar boot.
