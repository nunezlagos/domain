# Tasks: HU-11.2-memory-protocol

## Documento

- [ ] **D1: Crear MEMORY_PROTOCOL.md**
      - `docs/MEMORY_PROTOCOL.md`
      - Header con version y last updated
      - 6 secciones principales con subsecciones
      - Ejemplos de código en cada sección

- [ ] **D2: Integrar embed en binary**
      - `//go:embed MEMORY_PROTOCOL.md` en `internal/setup/protocol.go`

- [ ] **D3: Implementar `engram protocol` CLI**
      - `engram protocol` → imprime protocolo completo
      - `engram protocol --section when-to-save` → sección específica

- [ ] **D4: Referenciar protocolo desde templates de setup**
      - CLAUDE.md template incluye referencia
      - AGENTS.md section incluye referencia

## Tests

- [ ] **T1: `engram protocol` output incluye todas las secciones**
- [ ] **T2: `engram protocol --section` filtra correctamente**
- [ ] **T3: Embed funciona en build**
- [ ] **T4: Setup claude-code incluye referencia al protocolo**

## Cierre

- [ ] `go build ./...` sin errores
- [ ] `go vet ./...` sin warnings
- [ ] `engram protocol` output correcto
- [ ] Commit: `docs: add memory protocol for agent integration`
