# Design: HU-06.4-model-registry-cost

## Decisión arquitectónica

| Decisión | Opción elegida | Alternativa | Razón |
|----------|---------------|-------------|-------|
| Tokenizer | Interface + impl por provider | Una lib para todos | Cada provider usa algoritmo distinto |
| Precios | Hardcodeados en Go | DB/Config | Simple, cambios son poco frecuentes |
| Cost breakdown | Struct con desglose | Float único | Transparencia, debug |
| Fallback tokenizer | len(text)/4 ≈ tokens | N/A | Aproximación standard para modelos sin tokenizer |

## Alternativas descartadas

- **Precios en DB:** Overkill para valores que cambian cada meses. Hardcode + release es suficiente.
- **Una librería de tokenización:** No existe una que funcione para todos los providers.

## Diagrama

```
internal/llm/
├── registry.go
│   └── ModelRegistry
│       ├── GetModel(name) → ModelInfo
│       ├── UpdatePrice(name, input, output)
│       ├── ListModels() → []ModelInfo
│       └── CalculateCost(name, promptTokens, completionTokens) → CostBreakdown
│
├── tokenizer.go
│   └── TokenCounter interface
│       └── Count(model, text) → (int, error)
│
└── tokenizers/
    ├── tiktoken.go      → TiktokenCounter (OpenAI)
    ├── claude.go         → ClaudeTokenizer (Anthropic)
    └── approx.go         → ApproxCounter (fallback)
```

### Modelos precargados

```go
var defaultModels = map[string]ModelInfo{
    "gpt-4o":          {Provider: "openai",   InputPricePer1K: 0.005,   OutputPricePer1K: 0.015,   ContextWindow: 128000},
    "gpt-4o-mini":     {Provider: "openai",   InputPricePer1K: 0.00015, OutputPricePer1K: 0.0006,  ContextWindow: 128000},
    "claude-sonnet-4": {Provider: "anthropic",InputPricePer1K: 0.003,   OutputPricePer1K: 0.015,   ContextWindow: 200000},
    "claude-haiku":    {Provider: "anthropic",InputPricePer1K: 0.00025, OutputPricePer1K: 0.00125, ContextWindow: 200000},
    "gemini-2.0-flash":{Provider: "google",   InputPricePer1K: 0.0001,  OutputPricePer1K: 0.0004,  ContextWindow: 1000000},
    // Modelos locales cuestan 0
    "llama3.2":        {Provider: "ollama",   InputPricePer1K: 0,      OutputPricePer1K: 0,       ContextWindow: 128000},
}
```

## TDD plan

1. **TestGetModelExistente:** Modelo conocido → ModelInfo correcta
2. **TestGetModelInexistente:** Error
3. **TestCalculateCost:** 500 prompt + 200 completion en gpt-4o → 0.0055
4. **TestCalculateCostCero:** Modelo local → 0
5. **TestUpdatePrice:** Update → Get refleja nuevo precio
6. **TestListModels:** Lista completa con modelos esperados
7. **TestTiktokenCount:** Texto conocido → tokens exactos
8. **TestClaudeCount:** Texto conocido → tokens exactos
9. **TestApproxCount:** Fallback → len/4
10. **TestSabotaje:** Tiktoken no disponible → fallback a approx

## Riesgos y mitigación

- **Precios cambian:** Proceso de release trimestral para actualizar. Config file JSON en el futuro.
- **Tiktoken Go binding:** Si deja de funcionar, fallback automático a ApproxCounter.
- **Claude tokenizer:** Implementación propia basada en la spec pública de Anthropic.
