# add-openai-compat-provider — Design

## ADR-1 — Reusar el dialecto openai con base_url override

Agregar `NewWithBaseURL(apiKey, baseURL, model)` a `openai.Provider` + `RegisterOpenAICompat(factory, wrap)` en `internal/llm/openai/compat.go`. NO paquete `openaicompat` separado.

- Alternativas: paquete separado (duplica `Complete`/`CompleteStream`/`doRequest`); variadic options en `openai.New` (cambia firma pública).
- Tradeoff: reusar `openai.Provider` evita duplicar la lógica HTTP (DRY + yagni); el paquete openai pasa a servir doble propósito, aceptable porque hablan el mismo dialecto. Espejo de `anthropic/minimax.go`.
- Pattern: Adapter / reuse.

## ADR-2 — Config por env JSON

`DOMAIN_OPENAI_COMPAT_PROVIDERS=[{"name","base_url","api_key_env","model"}]`. El `api_key` se referencia por NOMBRE de env (`api_key_env`), nunca el valor.

- Alternativas: archivo declarativo (sin precedente, repo 100% env-driven); envs numeradas (frágil).
- Tradeoff: consistente con el repo, cero deps; parse tolerante (item inválido → warning sin key + skip, sin abortar boot).
- Seguridad (shift-left, acotado al cambio): secret fuera del config visible, nunca logueado (cubre REQ-4 / policy `secrets-redaction`). No introduce authz ni exposición cross-tenant.
- Pattern: Configuration via environment.

## Plan TDD

1. `TestOpenAICompat_Complete_CustomBaseURL_HitsConfiguredEndpoint` — AC1/AC3. Sabotaje: ignorar `baseURL` en `NewWithBaseURL`.
2. `TestRegisterOpenAICompat_ValidConfig_RegistersProviders` — AC2. Sabotaje: nombre fijo en vez de `cfg.Name`.
3. `TestRegisterOpenAICompat_ExistingProvidersUntouched` — AC4. Sabotaje: pisar un `Register` existente.
4. `TestRegisterOpenAICompat_MalformedJSON_SkipsWithoutPanic` — parse tolerante. Sabotaje: quitar el guard de `json.Unmarshal`.
5. `TestRegisterOpenAICompat_ApiKeyNotLogged` — AC5. Sabotaje: loguear config con key.
