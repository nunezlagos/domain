# Design: issue-06.3-ollama-runner

## Decisión arquitectónica

| Decisión | Opción elegida | Alternativa | Razón |
|----------|---------------|-------------|-------|
| API endpoint | /api/generate | /api/chat | Generate es más simple, Chat requiere historial |
| Pull automático | Opt-in via env var | Siempre | Evita descargas inesperadas |
| Streaming | SSE via channel | Polling | Consistente con otros runners |
| Timeout default | 120s | 30s | Modelos locales son más lentos |

## Alternativas descartadas

- **/api/chat:** Requiere formatear historial de mensajes. Generate acepta prompt directo. Más simple para el caso de uso interno.
- **Pull siempre:** Descargar modelos sin consentimiento es mala UX. Opt-in obligatorio.

## Diagrama

```
OllamaRunner.Complete:
  POST http://localhost:11434/api/generate
  {
    "model": "llama3.2",
    "prompt": "Hola",
    "stream": false,
    "options": { "temperature": 0.7 }
  }
  ← 200 { "response": "¡Hola!", "done": true, "context": [...] }

OllamaRunner.CompleteStream:
  POST http://localhost:11434/api/generate
  { "model": "llama3.2", "prompt": "Cuento", "stream": true }
  ← SSE: { "response": "Érase", "done": false }
  ← SSE: { "response": " una vez", "done": false }
  ← SSE: { "response": "", "done": true }
```

### Config

```go
type OllamaConfig struct {
    BaseURL   string  // default http://localhost:11434
    AutoPull  bool    // default false
    Timeout   time.Duration // default 120s
}
```

## TDD plan

1. **TestOllamaComplete:** Mock /api/generate → response correcto
2. **TestOllamaStream:** Mock SSE streaming → chunks
3. **TestOllamaModeloNoEncontrado:** Mock 404 → error "model not found"
4. **TestOllamaNoDisponible:** Connection refused → error
5. **TestOllamaURLPersonalizada:** Config URL distinta → usa esa URL
6. **TestOllamaAutoPull:** Mock /api/pull + /api/generate → success
7. **TestOllamaTimeout:** Context timeout → error
8. **TestSabotaje:** Response JSON inválido → error graceful

## Riesgos y mitigación

- **Ollama no instalado:** Error en Complete, no en startup. Lazy error.
- **Pull muy lento:** Timeout separado para pull (5 min). Documentar.
- **Modelo sin GPU:** CPU inference es lento. Timeout generoso.
