# HU-26.5-circuit-breaker-llm

**Origen:** `REQ-26-horizontal-scalability`
**Persona:** dx-engineer
**Prioridad tentativa:** alta
**Tipo:** hardening

## Historia de usuario

**Como** plataforma con dependencia LLM provider externos
**Quiero** circuit breaker per (provider, model) con fallback configurable
**Para** que outage de OpenAI/Anthropic no tumbe la plataforma entera

## Criterios de aceptación

### Escenario 1: CB tripped

```gherkin
Dado que OpenAI tuvo 5 errores 5xx/timeout consecutivos en 30s
Cuando llega request 6to
Entonces CB para (openai, gpt-4) está OPEN
Y el request fail-fast inmediato con `LLMProviderUnavailable`
Y métrica `domain_llm_circuit_state{provider,model}=1`
Y skill/agent recibe error tipado (HU-05.6 error taxonomy)
```

### Escenario 2: Fallback a modelo alternativo

```gherkin
Dado que agent declara `fallback_models: ["anthropic/claude-sonnet-4-6"]`
Cuando primary provider CB OPEN
Entonces motor intenta con fallback model
Y log warn "primary provider unavailable; using fallback"
Y métrica `domain_llm_fallback_used_total{from,to}`
```

### Escenario 3: Half-open recovery

```gherkin
Dado que CB OPEN hace 60s
Cuando llega request
Entonces se permite 1 probe (half-open)
Y si exitoso → CB CLOSED
Y si falla → CB reset 60s más
```

### Escenario 4: Per provider isolation

```gherkin
Dado que OpenAI CB OPEN
Y Anthropic CB CLOSED
Cuando llega request al Anthropic
Entonces procesa normal (no afectado por OpenAI)
```

### Escenario 5: Embedding fallback

```gherkin
Dado que provider embeddings (OpenAI) caído
Cuando se necesita embed para HU-03.4 knowledge doc
Entonces fallback a Ollama local si configurado
Y si no hay fallback → encolar para retry async (no perder request)
```

## Análisis breve

- **Qué pide:** CB per (provider,model) + fallback declarativo + half-open + embedding queue fallback
- **Esfuerzo:** M
- **Riesgos:** fallback model quality drop sin avisar
