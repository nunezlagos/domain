# Tasks: issue-06.4-model-registry-cost

## Backend

- [x] Implementar struct ModelInfo y ModelRegistry
- [x] Precargar modelos default con precios actualizados
- [x] Implementar CalculateCost con desglose
- [x] Implementar interface TokenCounter
- [x] Implementar TiktokenCounter (OpenAI)
- [x] Implementar ClaudeTokenizer (Anthropic)
- [x] Implementar ApproxCounter (fallback genérico)
- [x] Integrar token counter en los runners para poblar TokenUsage

## Frontend

- [x] N/A

## Tests

- [x] Test unitario: GetModel para cada modelo conocido
- [x] Test unitario: GetModel para modelo inexistente → error
- [x] Test unitario: CalculateCost con valores exactos
- [x] Test unitario: CalculateCost con modelo local (costo 0)
- [x] Test unitario: UpdatePrice funciona
- [x] Test unitario: ListModels devuelve todos
- [x] Test unitario: TiktokenCount con texto conocido
- [x] Test unitario: ClaudeCount con texto conocido
- [x] Test unitario: ApproxCount como fallback
- [x] Sabotaje: tokenizer falla → fallback a approx

## Cierre

- [x] Verificar que todos los modelos usados en la app están registrados
- [x] Suite verde
