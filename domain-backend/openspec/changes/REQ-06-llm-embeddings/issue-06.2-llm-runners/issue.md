# issue-06.2-llm-runners

**Origen:** `REQ-06-llm-embeddings`
**Prioridad tentativa:** alta
**Tipo:** feature

## Historia de usuario

**Como** desarrollador de la plataforma
**Quiero** runners concretos para OpenAI (gpt-4o, gpt-4o-mini), Anthropic (claude-sonnet-4, claude-haiku) y Google (gemini-2.0-flash) que implementen la interfaz Provider
**Para** poder ejecutar prompts contra cualquier proveedor LLM de forma transparente

## Criterios de aceptación

### Escenario 1: OpenAI runner - completion básico

```gherkin
Dado que DOMAIN_OPENAI_KEY está configurada
Cuando llamo a `openaiRunner.Complete(ctx, "Hola", CompletionOpts{Model: "gpt-4o"})`
Entonces recibo un Response con Content no vacío
Y `Model` es "gpt-4o"
Y `Usage.PromptTokens` > 0
Y `Usage.CompletionTokens` > 0
Y `FinishReason` es "stop"
```

### Escenario 2: OpenAI runner con streaming

```gherkin
Dado el runner de OpenAI
Cuando llamo a `CompleteStream(ctx, "Cuenta un cuento", opts)`
Entonces recibo chunks progresivamente
Y el content completo es la concatenación de todos los chunks
Y el último chunk tiene `Done=true` y `Usage` poblado
```

### Escenario 3: Anthropic runner - claude-sonnet-4

```gherkin
Dado que DOMAIN_ANTHROPIC_KEY está configurada
Cuando llamo a `anthropicRunner.Complete(ctx, "Hola", CompletionOpts{Model: "claude-sonnet-4"})`
Entonces recibo un Response con Content no vacío
Y `Model` es "claude-sonnet-4"
```

### Escenario 4: Anthropic runner - claude-haiku

```gherkin
Cuando llamo a `anthropicRunner.Complete(ctx, "Resume esto", CompletionOpts{Model: "claude-haiku"})`
Entonces recibo response exitoso
Y el modelo en response es "claude-haiku"
```

### Escenario 5: Google runner - gemini-2.0-flash

```gherkin
Dado que DOMAIN_GOOGLE_KEY está configurada
Cuando llamo a `googleRunner.Complete(ctx, "Hola", CompletionOpts{Model: "gemini-2.0-flash"})`
Entonces recibo Response exitoso
Y `Model` es "gemini-2.0-flash"
```

### Escenario 6: Retry automático en errores transitorios

```gherkin
Dado que el API de OpenAI retorna 429 Too Many Requests
Cuando llamo a `Complete`
Entonces el runner reintenta automáticamente hasta 3 veces con backoff exponencial
Y eventualmente retorna response exitoso o error definitivo
```

### Escenario 7: Error por API key inválida

```gherkin
Dado que DOMAIN_OPENAI_KEY es inválida
Cuando llamo a `Complete`
Entonces recibo error "authentication failed: 401 Invalid API Key"
```

### Escenario 8: Rate limiting con backoff

```gherkin
Dado que el provider retorna 429 consecutivamente 3 veces
Cuando llamo a `Complete`
Entonces después del tercer reintento retorna error "rate limit exceeded after 3 retries"
```

### Escenario 9: Timeout en llamada al provider

```gherkin
Dado que el provider tarda > 30s en responder
Cuando llamo a `Complete` con contexto con timeout de 5s
Entonces el contexto se cancela
Y retorna error "context deadline exceeded"
```

### Escenario 10: Runners expuestos via factory

```gherkin
Dado que los tres runners están registrados en la factory
Cuando `factory.Get("openai")`
Entonces retorna un *OpenAIRunner
Cuando `factory.Get("anthropic")`
Entonces retorna un *AnthropicRunner
Cuando `factory.Get("google")`
Entonces retorna un *GoogleRunner
```

## Análisis breve

- **Qué pide realmente:** Implementaciones concretas de la interfaz Provider para OpenAI, Anthropic y Google. Cada runner maneja autenticación, retries, rate limits, y streaming.
- **Módulos sospechados:** `internal/llm/providers/openai.go`, `internal/llm/providers/anthropic.go`, `internal/llm/providers/google.go`
- **Riesgos / dependencias:** Depende de issue-06.1 (interfaz Provider). APIs externas pueden cambiar. Costos reales de API.
- **Esfuerzo tentativo:** L

## Verificación previa

- [ ] Revisar codebase (grep)
- [ ] Revisar memorias engram (domain_mem_search)
- [ ] Revisar git log
- [ ] Probar en ambiente correcto
- [ ] Reproducir con perfil correcto
- [ ] Verificar caché / build
- [ ] Verificar feature flag / config

### Resultado de verificación

- **Estado:** pendiente
- **Evidencia:**
- **Acción derivada:**
