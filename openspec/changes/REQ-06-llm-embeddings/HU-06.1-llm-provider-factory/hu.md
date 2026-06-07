# HU-06.1-llm-provider-factory

**Origen:** `REQ-06-llm-embeddings`
**Prioridad tentativa:** alta
**Tipo:** feature

## Historia de usuario

**Como** desarrollador de la plataforma
**Quiero** una fábrica de proveedores LLM con una interfaz común `Complete(ctx, prompt, opts) → Response` y un registro central de providers
**Para** poder cambiar entre OpenAI, Anthropic, Google, Ollama sin modificar el código que consume LLMs

## Criterios de aceptación

### Escenario 1: Obtener provider por nombre desde el registry

```gherkin
Dado que los providers OpenAI, Anthropic y Google están registrados
Cuando invoco `factory.Get("openai")`
Entonces recibo una instancia de Provider configurada
Y `Provider.Name()` es "openai"
```

### Escenario 2: Provider no registrado retorna error

```gherkin
Dado que "claude-unknown" no está registrado
Cuando invoco `factory.Get("claude-unknown")`
Entonces recibo un error "provider not found: claude-unknown"
```

### Escenario 3: Provider por defecto via config

```gherkin
Dado que `DOMAIN_LLM_PROVIDER=openai` está configurada
Cuando invoco `factory.GetDefault()`
Entonces recibo el provider OpenAI
```

### Escenario 4: Interfaz Complete con opciones

```gherkin
Dado un provider P implementando la interfaz Provider
Cuando llamo `P.Complete(ctx, "Hola mundo", opts)`
Donde opts incluye:
  - Model: "gpt-4o"
  - Temperature: 0.7
  - MaxTokens: 1000
Entonces recibo un Response con:
  - Content: string no vacío
  - Model: string
  - Usage: { PromptTokens, CompletionTokens, TotalTokens }
  - FinishReason: "stop" | "length"
```

### Escenario 5: Interfaz CompleteStream para streaming

```gherkin
Dado un provider P
Cuando llamo `P.CompleteStream(ctx, "Cuenta una historia", opts)`
Entonces recibo un channel de StreamChunk
Y cada chunk tiene: Content (string), Done (bool)
Y el último chunk tiene Done=true
Y `Usage` está disponible al final del stream
```

### Escenario 6: Register de nuevo provider

```gherkin
Dado un provider personalizado "my-custom-provider"
Cuando invoco `factory.Register("my-custom-provider", myProvider)`
Entonces `factory.Get("my-custom-provider")` retorna ese provider
```

### Escenario 7: Registry es thread-safe

```gherkin
Dado que 10 goroutines intentan leer `factory.Get("openai")` concurrentemente
Cuando todas se ejecutan en paralelo
Entonces ninguna retorna error
Y todas reciben la misma instancia del provider
```

### Escenario 8: Configuración vía env vars

```gherkin
Dado el archivo .env con:
  DOMAIN_LLM_PROVIDER=openai
  DOMAIN_OPENAI_KEY=sk-...
  DOMAIN_OPENAI_ORG=org-...
Cuando factory se inicializa
Entonces el provider OpenAI está configurado con esa API key y org
```

## Análisis breve

- **Qué pide realmente:** Factory pattern + Provider interface + thread-safe registry. Config via env vars. Soporte para chat completion y streaming.
- **Módulos sospechados:** `internal/llm/factory.go`, `internal/llm/provider.go`, `internal/config/`
- **Riesgos / dependencias:** Ninguno external. Es la base para HU-06.2, HU-06.3, HU-06.5.
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
