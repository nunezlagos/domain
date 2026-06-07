# HU-10.2-conflict-semantic-judge

**Origen:** `REQ-10-conflict-detection`
**Prioridad:** alta
**Tipo:** feature

## Historia de usuario

**Como** usuario de memoria
**Quiero** que un juez semántico (LLM) evalúe los candidates léxicos
**Para** determinar si realmente son conflictos/duplicados y reducir falsos positivos

**Como** desarrollador
**Quiero** configurar el agente LLM con ENGRAM_AGENT_CLI (claude/opencode)
**Para** elegir qué CLI de IA usar para el juicio semántico

## Criterios de aceptación

```gherkin
Scenario: JudgeBySemantic evalúa un candidate via LLM
  Given un candidate en memory_relations con judgment_status="pending"
  When se ejecuta JudgeBySemantic(candidate)
  Then el LLM recibe: source content + target content
  And retorna un veredicto: "supersedes", "conflicts_with", "duplicate", "unrelated"

Scenario: Veredicto persiste en memory_relations
  Given JudgeBySemantic retorna "supersedes"
  When se actualiza memory_relations
  Then judgment_status cambia a "judged"
  And relation se actualiza al veredicto
  And confidence se actualiza con el valor del LLM
  And marked_by_kind = "llm"
  And marked_by_model incluye el modelo usado

Scenario: ENGRAM_AGENT_CLI=claude usa Claude CLI
  Given ENGRAM_AGENT_CLI=claude
  When se ejecuta JudgeBySemantic
  Then ejecuta "claude" como subproceso con el prompt

Scenario: ENGRAM_AGENT_CLI=opencode usa OpenCode
  Given ENGRAM_AGENT_CLI=opencode
  When se ejecuta JudgeBySemantic
  Then ejecuta "opencode" como subproceso con el prompt

Scenario: Concurrency control ejecuta N juicios en paralelo
  Given hay 50 candidates pendientes
  When se ejecuta JudgePending(concurrency=5)
  Then máximo 5 juicios ocurren simultáneamente

Scenario: Timeout aborta juicio lento
  Given un LLM tarda más del timeout configurado (default 30s)
  When se ejecuta JudgeBySemantic
  Then el juicio se aborta
  And judgment_status = "error" con reason "timeout"

Scenario: --max-semantic limita juicios por ejecución
  Given hay 100 candidates pendientes
  When se ejecuta JudgePending con --max-semantic=10
  Then solo 10 candidates son juzgados

Scenario: LLM response malformed se maneja graceful
  Given el LLM retorna texto que no es un veredicto válido
  When se procesa la respuesta
  Then judgment_status = "error" con reason "invalid_verdict"
  And la respuesta raw se guarda en evidence

Scenario: Candidate ya juzgado no se re-evalúa
  Given un candidate con judgment_status="judged"
  When se ejecuta JudgeBySemantic sobre él
  Then se skipea con log "already judged"

Scenario: Sin candidates pendientes no hace nada
  Given memory_relations no tiene candidates "pending"
  When se ejecuta JudgePending()
  Then reporta "no pending candidates to judge"
```

## Análisis breve

- **Qué pide realmente:** LLM-based judge que ejecuta CLI externa (claude/opencode) para evaluar candidates, con concurrencia controlada, timeout, max-semantic limit
- **Módulos sospechados:** `internal/conflict/semantic.go` con JudgeBySemantic, JudgePending
- **Riesgos / dependencias:** CLI externa puede no estar instalada; respuesta LLM no estructurada; subprocesos concurrentes
- **Esfuerzo tentativo:** L

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
- **Agente:** —
- **Evidencia:** —
- **Acción derivada:** —
