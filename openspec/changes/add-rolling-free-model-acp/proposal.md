# add-rolling-free-model-acp

## Why
El provider ACP hace spawn-per-Complete contra `opencode acp` con un solo modelo. Para aprovechar la capa gratuita de opencode y no chocar el rate-limit por modelo, hay que rotar entre los free, descubiertos automáticamente (no hardcodeados).

## What changes
- `internal/llm/acp/rolling.go`: descubrimiento automático (`opencode models --verbose`, cost==0), round-robin + cooldown, refresh TTL en background, home por modelo.
- `internal/llm/acp/provider.go`: `Complete` elige modelo y setea HOME por modelo.
- `internal/llm/acp/register.go`: cablea el roller (config por env).
- `internal/agentbridge/acp/process.go`: dedup de HOME en `scrubbedEnv` si hace falta.
- `internal/llm/acp/rolling_test.go`: tests rotación/cooldown/fallback con spawn mock.

## Impact
Epic DOMAINSERV-62. Sin cambio de authz ni secrets. Costo ~3-5s/llamada inherente (no baja; distribuye).
