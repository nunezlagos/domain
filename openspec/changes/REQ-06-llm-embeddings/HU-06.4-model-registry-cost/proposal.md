# Proposal: HU-06.4-model-registry-cost

## Intención

Implementar un registro central de modelos LLM con precios por token (input/output), cálculo automático de costos basado en TokenUsage, y tokenizadores específicos por provider (tiktoken para OpenAI, claude-tokenizer para Anthropic, aproximación para Google).

## Scope

**Incluye:**
- `ModelRegistry` con modelos precargados y capacidad de actualizar precios
- `ModelInfo`: model, provider, input_price_per_1k, output_price_per_1k, context_window, max_output
- `CalculateCost(model, promptTokens, completionTokens) → CostBreakdown`
- `CostBreakdown`: input_cost, output_cost, total_cost, currency (USD)
- `TokenCounter` interface con implementaciones por provider
- `Count(model, text) → int` usando tiktoken (Go binding) y claude-tokenizer (Go port)
- Registro de modelos hardcodeado (actualizable vía DB en el futuro)

**Excluye:**
- Persistencia en DB (los precios se actualizan vía código)
- Cost tracking histórico (HU-15.1)
- Alertas de presupuesto (HU-15.3)

## Enfoque técnico

- ModelRegistry: struct con map[string]ModelInfo. Precargado en init con modelos conocidos.
- TokenCounter: interface `Count(model string, text string) (int, error)`.
- Implementaciones: `TiktokenCounter` (usa `tiktoken-go`), `ClaudeTokenizer` (usa port Go de claude-tokenizer), `ApproxCounter` (fallback: len(text)/4 para otros modelos).
- CalculateCost: operación aritmética simple. Devuelve CostBreakdown.

## Riesgos

- **Precios desactualizados:** Los providers cambian precios. Mitigación: release periódico con precios actualizados, o eventualmente leer desde DB.
- **Dependencia tiktoken-go:** Binding Go no oficial. Mitigación: fallback a ApproxCounter si falla.
- **Claude tokenizer en Go:** No existe binding oficial. Mitigación: implementar basado en la spec de Anthropic o usar aproximación.

## Testing

- **Unitarios:** Cálculo de costos, registry CRUD, contadores de tokens.
- **Sabotaje:** Tokenizer falla → fallback a aproximación.
