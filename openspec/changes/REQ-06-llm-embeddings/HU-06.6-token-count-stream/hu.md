# HU-06.6-token-count-stream

**Origen:** `REQ-06-llm-embeddings`
**Prioridad tentativa:** media
**Tipo:** feature

## Historia de usuario

**Como** desarrollador de la plataforma
**Quiero** una librería de conteo de tokens que funcione con streaming, que pueda trackear el presupuesto de tokens en tiempo real durante llamadas LLM streaming, y que entregue output fragmentado (chunked) para respuestas largas
**Para** tener control granular sobre el consumo de tokens y poder interrumpir ejecuciones que excedan el presupuesto

## Criterios de aceptación

### Escenario 1: Contar tokens de un texto completo

```gherkin
Dado un texto de ejemplo
Cuando llamo a `tokenCounter.Count("gpt-4o", texto)`
Entonces recibo un entero positivo con la cantidad de tokens
Y coincide con el conteo de tiktoken
```

### Escenario 2: Streaming con conteo de tokens en tiempo real

```gherkin
Dado una llamada streaming a un LLM
Cuando recibo chunks del stream
Entonces cada chunk incluye `cumulative_tokens` (total acumulado hasta ese chunk)
Y el último chunk tiene `total_tokens` final
```

### Escenario 3: Token budget tracking durante streaming

```gherkin
Dado un presupuesto máximo de 1000 tokens
Cuando inicio un stream con `TokenBudget{MaxTokens: 1000}`
Y después de varios chunks el cumulative_tokens llega a 950
Entonces el próximo chunk que excede 1000 debe ser truncado
Y el stream se cierra con `finish_reason = "token_limit"`
```

### Escenario 4: Chunked output para respuestas largas

```gherkin
Dado que una respuesta LLM es de 5000 tokens
Cuando la configuro con chunk_size=1000
Entonces recibo 5 chunks de ~1000 tokens cada uno
Y cada chunk tiene metadata: chunk_index (0..4), is_last (bool), cumulative_tokens
```

### Escenario 5: Token budget con timeout

```gherkin
Dado un TokenBudget con MaxTokens=500 y MaxSeconds=30
Cuando el stream supera los 30 segundos
Entonces el stream se cierra con `finish_reason = "timeout"`
Y `cumulative_tokens` refleja los tokens hasta el momento
```

### Escenario 6: Presupuesto compartido entre múltiples llamadas

```gherkin
Dado un TokenBudget compartido con MaxTokens=2000
Cuando hago 3 llamadas LLM en secuencia que consumen 800, 700 y 600 tokens respectivamente
Entonces las primeras 2 llamadas completan exitosamente
Y la tercera llamada se rechaza porque excede el presupuesto restante (500 < 600)
```

### Escenario 7: Wrap de provider streaming con token counter

```gherkin
Dado un provider P con CompleteStream
Cuando lo envuelvo con `NewTokenCountingProvider(P, budget)`
Entonces los chunks incluyen información de tokens acumulados
Y si el presupuesto se excede, el stream se corta automáticamente
```

### Escenario 8: Chunks para WebSocket response

```gherkin
Dado una respuesta streaming con chunk_size=500
Cuando el chunk está listo
Entonces se envía via WebSocket al cliente
Y el cliente puede renderizar el texto progresivamente
```

## Análisis breve

- **Qué pide realmente:** Librería de conteo de tokens con soporte streaming, token budget tracking (cortar cuando se excede), chunked output para respuestas largas, y wrapper para providers existentes.
- **Módulos sospechados:** `internal/llm/tokenbudget.go`, `internal/llm/stream.go`, `internal/llm/chunker.go`
- **Riesgos / dependencias:** Depende de HU-06.4 (token counters) y HU-06.2 (providers streaming). El chunking debe respetar límites de tokens, no de caracteres.
- **Esfuerzo tentativo:** M

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
