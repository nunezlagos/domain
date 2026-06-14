# Proposal: issue-06.3-ollama-runner

## Intención

Implementar el runner para Ollama, permitiendo usar modelos LLM locales a través de la API REST de Ollama. Esto es crítico para desarrollo offline, testing sin costos, y entornos air-gapped.

## Scope

**Incluye:**
- `OllamaRunner` implementando interfaz `Provider`
- Configuración via `DOMAIN_OLLAMA_URL` (default `http://localhost:11434`)
- Soporte para cualquier modelo descargado en Ollama
- API calls a `/api/generate` (completion) y `/api/generate` con stream=true
- Pull automático de modelo (opt-in via `OLLAMA_AUTO_PULL=true`)
- Timeout configurable (modelos locales pueden ser lentos)
- Registro en factory como "ollama"

**Excluye:**
- Gestión de modelos (pull/delete/list via CLI)
- Soporte multi-gpu o configuración avanzada de Ollama
- Embeddings via Ollama (futuro)

## Enfoque técnico

- API de Ollama es simple REST: POST `/api/generate` con `{ model, prompt, stream, options }`.
- Response incluye `response` (texto), `done` (bool), `context` (opcional).
- Streaming via SSE similar a OpenAI.
- Timeout default 120s para generación local.
- Pull automático: POST `/api/pull` con `{ name: model }`, esperar a que termine.

## Riesgos

- **Rendimiento local:** Modelos grandes pueden tardar minutos. Timeout generoso.
- **Ollama no instalado:** Error claro y graceful. No bloquear startup.
- **Pull automático:** Puede descargar GBs de datos. Opt-in explícito.

## Testing

- **Unitarios:** Mock HTTP de Ollama API.
- **Integración (opcional):** Con Ollama real en CI.
- **Sabotaje:** Ollama no disponible → error graceful.
