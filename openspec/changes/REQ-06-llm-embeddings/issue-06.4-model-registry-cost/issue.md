# issue-06.4-model-registry-cost

**Origen:** `REQ-06-llm-embeddings`
**Prioridad tentativa:** media
**Tipo:** feature

## Historia de usuario

**Como** administrador de la plataforma
**Quiero** un registro central de modelos LLM que mapee cada modelo a su provider y costo por token (input/output), con cálculo automático de costo por ejecución y conteo de tokens
**Para** tener visibilidad y control sobre los costos de uso de LLM

## Criterios de aceptación

### Escenario 1: Consultar costo de un modelo conocido

```gherkin
Dado el model registry precargado con:
  | model           | provider | input_price_per_1k | output_price_per_1k |
  | gpt-4o          | openai   | 0.005              | 0.015               |
  | gpt-4o-mini     | openai   | 0.00015            | 0.0006              |
  | claude-sonnet-4 | anthropic| 0.003              | 0.015               |
  | claude-haiku    | anthropic| 0.00025            | 0.00125             |
  | gemini-2.0-flash| google   | 0.0001             | 0.0004              |
Cuando consulto `registry.GetModel("gpt-4o")`
Entonces recibo:
  - provider: "openai"
  - input_price_per_1k: 0.005
  - output_price_per_1k: 0.015
```

### Escenario 2: Modelo no registrado

```gherkin
Cuando consulto `registry.GetModel("modelo-desconocido")`
Entonces recibo error "model not found: modelo-desconocido"
```

### Escenario 3: Calcular costo de una ejecución

```gherkin
Dado que usé gpt-4o con PromptTokens=500 y CompletionTokens=200
Cuando llamo `registry.CalculateCost("gpt-4o", 500, 200)`
Entonces el costo es (500 * 0.005 / 1000) + (200 * 0.015 / 1000)
Y el resultado es 0.0055 USD
```

### Escenario 4: Calcular costo con TokenUsage

```gherkin
Dado un TokenUsage{PromptTokens: 1000, CompletionTokens: 500, TotalTokens: 1500}
Cuando calculo costo para "claude-sonnet-4"
Entonces input_cost = 1000 * 0.003 / 1000 = 0.003
Y output_cost = 500 * 0.015 / 1000 = 0.0075
Y total_cost = 0.0105 USD
```

### Escenario 5: Contar tokens con tiktoken (OpenAI)

```gherkin
Dado un texto "Hello, world!"
Cuando llamo `tokenCounter.Count("gpt-4o", "Hello, world!")`
Entonces retorna 4 tokens (según tiktoken)
```

### Escenario 6: Contar tokens con claude tokenizer (Anthropic)

```gherkin
Dado un texto "Hello, world!"
Cuando llamo `tokenCounter.Count("claude-sonnet-4", "Hello, world!")`
Entonces retorna el conteo según el tokenizer de Anthropic
```

### Escenario 7: Registry precargado por defecto

```gherkin
Cuando el sistema se inicializa
Entonces el model registry contiene todos los modelos soportados
Y cada modelo tiene provider, input_price, output_price válidos
```

### Escenario 8: Actualizar precio de un modelo

```gherkin
Cuando `registry.UpdatePrice("gpt-4o", 0.004, 0.012)`
Entonces `registry.GetModel("gpt-4o").input_price_per_1k` es 0.004
Y `registry.GetModel("gpt-4o").output_price_per_1k` es 0.012
```

### Escenario 9: Listar todos los modelos

```gherkin
Cuando `registry.ListModels()`
Entonces retorna todos los modelos registrados
Y cada entrada tiene: model, provider, input_price, output_price
```

## Análisis breve

- **Qué pide realmente:** Registro central de modelos con precios, cálculo de costos por uso, y utilidades de conteo de tokens con librerías específicas por provider.
- **Módulos sospechados:** `internal/llm/registry.go`, `internal/llm/tokenizer.go`, `internal/llm/cost.go`
- **Riesgos / dependencias:** Los precios cambian periódicamente. Los tokenizers son específicos por provider (tiktoken para OpenAI, claude-tokenizer para Anthropic).
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
