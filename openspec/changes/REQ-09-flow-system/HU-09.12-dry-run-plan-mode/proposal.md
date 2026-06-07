# Proposal: HU-09.12-dry-run-plan-mode

## Intención

Endpoint `dry-run` que analiza estáticamente un flow + inputs, devuelve plan con steps que ejecutaría, estimación de tokens y costo, sin side-effects.

## Scope

**Incluye:**
- Static analyzer del DAG
- Cost estimator usando model_registry (REQ-06)
- Template renderer para LLM prompts
- Side-effects detection
- Conditionals resueltos donde sea posible estáticamente
- Endpoint POST /flows/:id/dry-run

**No incluye:**
- Simulación real con LLM stub (futuro)
- Replay vs original results comparison (HU-09.11 cubre eso)

## Enfoque técnico

1. Walker del DAG aplica inputs y resuelve template strings
2. Conditionals: si toda la expresión es de inputs conocidos → resuelve; sino marca depends_on_runtime
3. Token counter usa `pkoukk/tiktoken-go` o equivalente por provider
4. Pricing del model_registry (HU-06.4)

## Riesgos

- Estimación imprecisa → comunicar margen ±20%
- Conditionals con loops/recursión → cap max nodes

## Testing

- Plan simple
- Conditional resuelto estático
- Conditional dinámico marcado
- Cost matchea calculadora manual
- Side-effects skipped warning
