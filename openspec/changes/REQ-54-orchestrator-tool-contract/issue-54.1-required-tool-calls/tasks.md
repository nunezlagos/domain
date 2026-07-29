# issue-54.1 required-tool-calls — Tasks

## 1. Schema
- [ ] Migración `NNNNNN_agent_templates_required_tool_calls.up.sql`: agregar
      `required_tool_calls JSONB NOT NULL DEFAULT '[]'` a `agent_templates`.
- [ ] `.down.sql` correspondiente.

## 2. Contrato en handlers
- [ ] Agregar método `RequiredToolCalls() []string` a la interfaz de phase handler.
- [ ] Implementar default por fase en `orchestrator/phases/sdd_*.go` (empezar VACÍO en
      todas salvo `sdd-verify`, que declara `domain_verify_start/_update_item/_complete`
      como primera fase piloto).

## 3. Hidratación del override
- [ ] En `service.go` (junto a `hydrateSystemPrompts`), leer
      `agent_templates.required_tool_calls` y, si no está vacío, usarlo sobre el default.

## 4. Payload del phase_result
- [ ] Agregar campo `tool_calls []string` al input de `domain_orchestrate_phase_result`
      (`orchestrate_tools.go` + tipo en `orchestrator/types.go`).
- [ ] Documentar el campo en la descripción del tool MCP.

## 5. Validación server-side
- [ ] En `phase_result.go`, función `validateRequiredToolCalls(required, reported []string)
      (missing []string)`.
- [ ] Integrar en el flujo de `phase_result` ANTES de `MarkStepCompleted`: si hay missing,
      devolver `{ok:false, missing_tool_calls}` sin completar el step.
- [ ] Asegurar que el rechazo NO marca `failed` (reintentable).

## 6. Tests
- [ ] Los 5 tests del TDD Plan en `phase_result_required_tools_test.go`.
- [ ] Verificar que `confirm_integration_test.go` y `exec_mode_test.go` siguen verdes.

## 7. Seed / activación
- [ ] Poblar `required_tool_calls` de `sdd-verify` en el seed de `agent_templates`.
- [ ] Documentar en el proposal cómo activar el resto de las fases incrementalmente.
