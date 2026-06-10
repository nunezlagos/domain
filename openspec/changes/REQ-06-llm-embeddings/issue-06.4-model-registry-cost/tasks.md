# Tasks: issue-06.4-model-registry-cost

## Backend

- [ ] Implementar struct ModelInfo y ModelRegistry
- [ ] Precargar modelos default con precios actualizados
- [ ] Implementar CalculateCost con desglose
- [ ] Implementar interface TokenCounter
- [ ] Implementar TiktokenCounter (OpenAI)
- [ ] Implementar ClaudeTokenizer (Anthropic)
- [ ] Implementar ApproxCounter (fallback genérico)
- [ ] Integrar token counter en los runners para poblar TokenUsage

## Frontend

- [ ] N/A

## Tests

- [ ] Test unitario: GetModel para cada modelo conocido
- [ ] Test unitario: GetModel para modelo inexistente → error
- [ ] Test unitario: CalculateCost con valores exactos
- [ ] Test unitario: CalculateCost con modelo local (costo 0)
- [ ] Test unitario: UpdatePrice funciona
- [ ] Test unitario: ListModels devuelve todos
- [ ] Test unitario: TiktokenCount con texto conocido
- [ ] Test unitario: ClaudeCount con texto conocido
- [ ] Test unitario: ApproxCount como fallback
- [ ] Sabotaje: tokenizer falla → fallback a approx

## Cierre

- [ ] Verificar que todos los modelos usados en la app están registrados
- [ ] Suite verde
