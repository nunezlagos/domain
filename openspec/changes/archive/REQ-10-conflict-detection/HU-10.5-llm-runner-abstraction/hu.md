# HU-10.5-llm-runner-abstraction

**Origen:** `REQ-10-conflict-detection`
**Prioridad:** media
**Tipo:** feature

## Historia de usuario

**Como** desarrollador de memoria
**Quiero** una abstracción de LLM runner que soporte múltiples proveedores (Claude, OpenCode) mediante factory pattern
**Para** que el semantic conflict judge pueda funcionar con cualquier LLM sin acoplarse a una implementación concreta

**Como** desarrollador
**Quiero** que cada runner implemente `Compare()` con contexto y timeout configurables
**Para** garantizar que las comparaciones semánticas no bloqueen el sistema indefinidamente

**Como** administrador
**Quiero** estimación de costo por modelo antes de ejecutar comparaciones masivas
**Para** decidir si usar Claude (más preciso, más caro) u OpenCode (gratuito, menor calidad)

## Criterios de aceptación

```gherkin
Scenario: Claude runner implementa SemanticRunner
  Given ENGRAM_AGENT_CLI=claude
  When se crea un runner via factory
  Then el runner es de tipo ClaudeRunner
  And implementa SemanticRunner interface

Scenario: OpenCode runner implementa SemanticRunner
  Given ENGRAM_AGENT_CLI=opencode
  When se crea un runner via factory
  Then el runner es de tipo OpenCodeRunner
  And implementa SemanticRunner interface

Scenario: Compare() ejecuta comparación semántica con contexto
  Given dos textos para comparar
  When se llama a runner.Compare(ctx, textA, textB)
  Then retorna ConflictVeredict con relation y confidence
  And la operación respeta el context.Context (cancelación, deadline)

Scenario: Compare() respeta timeout configurable
  Given runner con timeout de 5s
  When la comparación toma más de 5s
  Then la operación se cancela con error de timeout
  And no quedan procesos colgados

Scenario: Cost estimation por modelo
  Given un runner asociado a un modelo
  When se llama a runner.EstimateCost(inputTokens)
  Then retorna estimación en USD
  And Claude es más caro que OpenCode para el mismo input

Scenario: ENGRAM_AGENT_CLI inválido produce error
  Given ENGRAM_AGENT_CLI=invalid-runner
  When se intenta crear un runner via factory
  Then retorna error "unsupported agent CLI: invalid-runner"

Scenario: Prompt builder genera template estructurado
  Given dos observaciones en conflicto
  When se llama a BuildComparePrompt(obsA, obsB)
  Then retorna un prompt con: contexto, obs A, obs B, instrucciones de análisis
  And el prompt incluye schema de respuesta esperada

Scenario: EmptyAgentCLI usa default
  Given ENGRAM_AGENT_CLI no está seteado
  When se crea un runner via factory
  Then se usa el runner default (OpenCodeRunner)

Scenario: Cost estimation para batch
  Given N comparaciones a ejecutar
  When se llama a runner.EstimateBatchCost(N, avgTokens)
  Then retorna costo total estimado y por operación

Scenario: Runner reporta modelo usado
  Given un runner creado
  When se llama a runner.ModelName()
  Then retorna string identificando el modelo (e.g. "claude-sonnet-4", "opencode-default")

Scenario: ClaudeRunner falla elegantemente si API key no está
  Given ANTHROPIC_API_KEY no está seteada
  When se ejecuta Compare()
  Then retorna error "Claude API key not configured"
  And no crash
```

## Análisis breve

- **Qué pide realmente:** Interfaz SemanticRunner con factory, implementaciones ClaudeRunner y OpenCodeRunner, cost estimation, prompt builder, manejo de timeouts y contexto
- **Módulos sospechados:** `internal/conflict/runner/` — nueva carpeta con interface.go, factory.go, claude.go, opencode.go, cost.go, prompt.go
- **Riesgos / dependencias:** Timeout mal configurado puede cancelar comparaciones válidas; cost estimation depende de precios actualizados de Anthropic; OpenCode runner puede no ser determinista
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
