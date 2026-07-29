# issue-54.2 server-side context prep — Tasks

## 1. Contrato de preparación por fase
- [ ] Agregar `PrepToolCalls() []string` a la interfaz de phase handler (separado de
      `RequiredToolCalls` de 54.1).
- [ ] Declarar el mapeo por fase (arrancar con `sdd-apply` = `[policy_list,
      project_skill_list]` como piloto).

## 2. Ejecutor de preparación (crudo)
- [ ] Función `prepareContextRaw(phase, flowRun) (string, error)` que corre las
      `PrepToolCalls` read-only contra los servicios existentes (policy, skill, mem, code).
- [ ] Donde haya >1 tool, evaluar expresarlo como sub-flow del `flowrunner`
      (`steptypes` parallel) en vez de un loop nuevo.

## 3. Capa inteligente (Minimax)
- [ ] Función `refineWithMinimax(raw, phaseHint) (string, error)` con timeout corto
      (config, default 3s).
- [ ] Usar el Factory LLM ya existente; si `LLM == nil` o timeout → devolver `raw`.

## 4. Integración en el orquestador
- [ ] Llamar `prepareContext` antes de entregar el prompt de la fase; inyectar
      `prepared_context` en `PriorOutputs` / el prompt.
- [ ] Asegurar que un fallo de preparación NO bloquea la fase (degradación total a "sin
      prepared_context").

## 5. Config
- [ ] Variable `ORCHESTRATOR_PREP_MINIMAX_TIMEOUT` (o similar) documentada en `.env.example`.

## 6. Tests
- [ ] Los 5 tests del TDD Plan en `context_prep_test.go` (Minimax mockeado).
- [ ] Test de latencia: prep bajo el umbral con Minimax mock rápido.

## 7. Dependencia
- [ ] Verificar que issue-54.3 (key Minimax en env del proceso Go) esté hecho antes de
      activar la capa inteligente en prod; si no, la prep corre en modo crudo.
