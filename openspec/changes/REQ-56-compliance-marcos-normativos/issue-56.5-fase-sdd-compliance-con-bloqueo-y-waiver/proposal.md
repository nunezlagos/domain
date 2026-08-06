# Proposal: fase `sdd-compliance` con bloqueo y waiver auditado

**REQ padre:** REQ-56-compliance-marcos-normativos
**Depende de:** issue-56.4 (catálogo de marcos y controles)
**Esfuerzo:** M (1-3 días)
**Prioridad:** media

## Intention

Que un cambio que incumple una obligación de un marco vigente declarado por el proyecto no llegue a
escribirse: el flow se detiene en `sdd-compliance`, antes de generar tasks y código, con la
obligación citada por artículo y una vía de waiver que exige razón escrita y queda auditada.

## Scope

**Entra:**
- La fase `sdd-compliance` entre `sdd-design` y `sdd-tasks`, con handler y contrato propios.
- Severidad derivada de `obligatorio` + `vigente_desde` del catálogo.
- Waiver persistido en BD con razón obligatoria, actor y timestamp.
- Handoff de los controles exigidos a R1 vía `PriorOutputs`.
- No-op explícito cuando el proyecto no declaró marcos.
- Declaración del hueco de los modos reducidos.

**No entra:**
- Generar documentación de compliance (RAT, política de privacidad, DPA, EIPD): es estado
  documental, no propiedad de un change. Va por proceso periódico, en otro issue.
- Modificar `sdd-4r` ni `r1_shift_left`: la fase le entrega los controles, R1 conserva su scoping.
- Poblar el catálogo de marcos (es de issue-56.4).
- Hacer correr la fase en `lite` / `express` / `micro`: no tienen `sdd-design`. Queda declarado como
  hueco conocido con sugerencia de subir a `full`.

## Approach

1. Handler nuevo siguiendo el patrón de `phases/sdd_4r.go`, con el no-op al inicio.
2. Inserción en el DAG en tres lugares: `phaseDependencies`, `FullPhases` y `prepPhaseContext`.
3. La derivación de severidad como función pura, sin BD, para que su test no necesite container.
4. El waiver como escritura auditada, expuesta por tool MCP bajo `rlsProyecto`.

## Risks

- **Fatiga de waiver** → solo una ley obligatoria **y vigente** bloquea; la severidad sale del
  catálogo y no de una heurística.
- **Hueco de los modos reducidos** → declarado en REQ-5 con constancia en el reporte. Un hueco
  declarado es deuda; uno silencioso es un bug.
- **Falsa sensación de cumplimiento** → pasar la fase es no violar lo que el catálogo modela, no
  cumplir la ley. El reporte lo dice explícitamente.
- **Olvidar el bump de `agentTemplatesSeedVersion`** → el template no llega a la BD y el síntoma es
  indistinguible del éxito.
