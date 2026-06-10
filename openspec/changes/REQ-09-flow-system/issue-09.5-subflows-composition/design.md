# Design: issue-09.5-subflows-composition

## Decisión arquitectónica

| Decisión | Opción elegida | Alternativas |
|----------|---------------|--------------|
| Ejecución de sub-flow | Nuevo FlowRun con parent_run_id FK | Inline execution en misma goroutine (FK permite trazabilidad) |
| Detección de circularidad | Set de ancestors slugs en FlowContext | Graph global lock (más complejo, set es O(1) lookup) |
| Límite de profundidad | Contador en FlowContext | Navegación recursiva del DAG (contador es más simple) |
| Output de sub-flow | Último step output = flow output | Step específico marcado como output (más configurable pero más complejo) |

## Alternativas descartadas

- **Inline execution**: Ejecutar sub-flow en la misma goroutine del padre sin crear FlowRun separado. No permite trazabilidad ni consulta de estado del sub-flow independientemente.
- **Output step explícito**: Marcar un step con `is_output: true` en el sub-flow. Más flexible pero más complejo; para MVP, el último step output es suficiente.
- **Lock global para circularidad**: Innecesario; set de ancestors por contexto de ejecución es suficiente sin estado compartido.

## Diagrama

```
Flow Padre: "order-confirmation"
┌────────────────────────────────────────────────────┐
│ steps:                                              │
│  [get_order] → [parallel_subflows] → [send_email]  │
└────────────────────────────────────────────────────┘
                       │
                       │ paso sub_flow
                       ▼
              ┌─────────────────┐
              │  SubFlowRunner  │
              │  (step type)    │
              └────────┬────────┘
                       │
              ┌────────▼────────┐
              │ ¿Slug en        │── sí ──► Error: circular
              │  ancestors?     │
              └────────┬────────┘
                       │ no
              ┌────────▼────────┐
              │ ¿Depth > 5?     │── sí ──► Error: too deep
              └────────┬────────┘
                       │ no
              ┌────────▼────────┐
              │ Crear FlowRun   │
              │ parent_run_id= ✓│
              │ ancestors +slug │
              │ input = mapped  │
              └────────┬────────┘
                       │
              ┌────────▼────────┐
              │ FlowRunner      │
              │ (ejecuta sub)   │
              └────────┬────────┘
                       │
              ┌────────▼────────┐
              │ Output del sub  │
              │ → resultado step│
              └─────────────────┘
```

Modelo FlowRun con sub-flow support:
```sql
ALTER TABLE flow_runs ADD COLUMN parent_run_id UUID REFERENCES flow_runs(id);
ALTER TABLE flow_runs ADD COLUMN ancestor_slugs TEXT[] NOT NULL DEFAULT '{}';
ALTER TABLE flow_runs ADD COLUMN depth INT NOT NULL DEFAULT 0;
```

## TDD plan

1. **Red:** Test `TestSubFlowRunner_ExecutesFlow` — sub-flow se ejecuta
2. **Green:** Implementar SubFlowRunner que crea FlowRun hijo
3. **Red:** Test `TestSubFlowRunner_CircularDirect` — A → A
4. **Green:** Detectar slug en ancestors set
5. **Red:** Test `TestSubFlowRunner_CircularIndirect` — A → B → A
6. **Green:** Propagación de ancestors set
7. **Red:** Test `TestSubFlowRunner_DepthLimit` — 6 niveles falla
8. **Green:** Verificar profundidad antes de ejecutar
9. **Red:** Test `TestContextPassing_ParentToChild` — input mapeado correctamente
10. **Green:** Resolver templates del input contra contexto padre
11. **Red:** Test `TestContextPassing_ChildToParent` — output disponible en padre
12. **Green:** Último step output del sub-flow → step result del padre
13. **Red:** Test `TestParallelSubFlows` — 2 sub-flows concurrentes
14. **Green:** ParallelRunner con SubFlowRunner
15. **Red:** Test `TestGetParents` — endpoint parents funciona
16. **Green:** Escanear flows en DB buscando referencias
17. **Sabotaje:** Quitar detección de circularidad → test de ciclo falla

## Riesgos y mitigación

| Riesgo | Probabilidad | Impacto | Mitigación |
|--------|-------------|---------|------------|
| Anidamiento excesivo (>5) | Baja | Medio | Límite de 5 niveles con error claro |
| Sub-flow lento bloquea al padre | Media | Alto | Timeout por step se hereda al sub-flow completo |
| Output del sub-flow muy grande | Baja | Medio | Limitar tamaño de output a 1MB |
| Circularidad en validación estática | Media | Medio | Además de detección en runtime, validar en guardado de flow (verificar que ningún padre eventual tenga al flow como sub-flow) |
