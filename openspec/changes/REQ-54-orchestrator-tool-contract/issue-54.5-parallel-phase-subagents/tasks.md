# issue-54.5 parallel-phase-subagents — Tasks

## 1. Circuito del plan (mismo patrón que 54.1)
- [ ] Campo `SubagentPlan string` en phases.Output (registry.go).
- [ ] Propagar en PhaseStep (modes/express.go struct) + mapeos en
      express/lite/full.
- [ ] Persistir en step.Inputs (repository.go persistPlan) y reconstruir en
      rebuildOutputFromStepInputs (phase_result.go).
- [ ] Helper `AgentTemplate.SubagentPlan()` leyendo metadata (repository.go,
      patrón RequiredToolCalls).

## 2. Inyección en el prompt (mismo patrón que 54.2)
- [ ] Punto A: hydrateSystemPrompts — anteponer bloque "## Plan de subagentes"
      al UserPrompt cuando el plan efectivo (override ?? default) no es vacío.
- [ ] Punto B: rebuildNextStepPrompt — idem.
- [ ] Orden: prepared_context primero, subagent_plan después.

## 3. Pilotos
- [ ] sdd-explore: plan de N Explore por área (máx 4) + merge con file:line.
- [ ] sdd-verify: lotes de escenarios en paralelo.
- [ ] sdd-judge: panel adversarial 2 jueces ciegos + desempate; actualizar
      handler.Validate para exigir len(judge_verdicts) >= 2.

## 4. Tests
- [ ] Los 5 del TDD plan (subagent_plan_test.go).
- [ ] Suite del orquestador completa sigue verde.

## 5. Seeds
- [ ] Poblar metadata.subagent_plan de los 3 pilotos en el seed de
      agent_templates (override editable desde BD).
