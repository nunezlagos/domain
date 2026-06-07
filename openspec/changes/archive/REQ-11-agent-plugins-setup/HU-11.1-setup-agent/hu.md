# HU-11.1-setup-agent

**Origen:** `REQ-11-agent-plugins-setup`
**Prioridad:** alta
**Tipo:** feature

## Historia de usuario

**Como** usuario de Claude Code
**Quiero** ejecutar `engram setup` para configurar la integración de memoria con mi CLI de IA
**Para** que el agente pueda acceder automáticamente a mis memorias

**Como** usuario de OpenCode, Gemini CLI, Codex, PI, o VS Code
**Quiero** que `engram setup` detecte mi agente y configure los plugins correspondientes
**Para** no tener que hacerlo manualmente

## Criterios de aceptación

```gherkin
Scenario: Setup detecta agente instalado automáticamente
  Given claude está instalado en el PATH
  When se ejecuta `engram setup --detect`
  Then detecta "claude-code" como agente disponible

Scenario: Setup específico para claude-code
  Given se ejecuta `engram setup --agent claude-code`
  When se configura la integración
  Then escribe CLAUDE.md con las instrucciones de memoria
  And configura el hook o plugin correspondiente

Scenario: Setup para opencode escribe AGENTS.md
  Given se ejecuta `engram setup --agent opencode`
  When se configura la integración
  Then escribe/actualiza AGENTS.md en el proyecto
  And agrega las reglas de memoria al archivo

Scenario: Setup para gemini-cli configura gemini
  Given se ejecuta `engram setup --agent gemini-cli`
  When se configura la integración
  Then escribe el manifest de plugin para gemini-cli
  And configura el prompt de sistema

Scenario: Setup para codex configura Codex CLI
  Given se ejecuta `engram setup --agent codex`
  When se configura
  Then escribe .codex/setup.json con las referencias a memoria

Scenario: Setup para pi configura Pi package
  Given se ejecuta `engram setup --agent pi`
  When se configura
  Then escribe pi.config.json con las referencias a memoria

Scenario: Setup para vs-code configura extensión
  Given se ejecuta `engram setup --agent vs-code`
  When se configura
  Then escribe .vscode/settings.json con las referencias a memoria

Scenario: Setup es idempotente
  Given ya se ejecutó `engram setup` previamente
  When se ejecuta nuevamente
  Then los archivos existentes no se duplican
  And se actualizan solo las secciones relevantes

Scenario: Setup informa qué archivos creó/modificó
  Given se ejecuta `engram setup --agent claude-code`
  When termina la configuración
  Then mustra lista de archivos creados/modificados con sus rutas

Scenario: Setup --dry-run mustra lo que haría sin escribir
  Given se ejecuta `engram setup --agent opencode --dry-run`
  When termina
  Then mustra qué archivos modificaría
  And no escribe ningún archivo
```

## Análisis breve

- **Qué pide realmente:** Comando `engram setup` que configura la integración con 6 agentes/CLIs, escribe config files/plugin manifests, detección automática, idempotente
- **Módulos sospechados:** `internal/setup/` — `setup.go`, `agents.go`, `writemanifest.go`
- **Riesgos / dependencias:** Conocer la estructura exacta de config de cada agente; puede necesitar actualizaciones cuando los agentes cambien
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
- **Evidencia:** —
- **Acción derivada:** —
