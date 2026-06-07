# Tasks: HU-06.3-ollama-runner

## Backend

- [ ] Implementar OllamaRunner con interfaz Provider
- [ ] Implementar llamada a /api/generate (completion)
- [ ] Implementar streaming via SSE
- [ ] Implementar configuración via env vars (DOMAIN_OLLAMA_URL)
- [ ] Implementar pull automático de modelo (opt-in)
- [ ] Implementar timeout de 120s para generación local
- [ ] Registrar OllamaRunner en factory como "ollama"

## Frontend

- [ ] N/A

## Tests

- [ ] Test unitario: completion básico con mock HTTP
- [ ] Test unitario: streaming con chunks mock
- [ ] Test unitario: modelo no encontrado → error
- [ ] Test unitario: conexión rechazada → error
- [ ] Test unitario: URL personalizada se usa correctamente
- [ ] Test unitario: timeout del contexto
- [ ] Sabotaje: respuesta inválida → error graceful

## Cierre

- [ ] Verificación manual con Ollama local
- [ ] Suite verde
