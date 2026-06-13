# Design: issue-35.2-skill-model-decision-record

## Contexto

El modelo de `skills` (issue-05.x) tiene 4 tipos declarados:
- `TypePrompt`: skill = system prompt + opcionalmente tools. ✅
  Implementado, ejecutado server-side.
- `TypeAPI`: skill = HTTP endpoint + schema. ⚠ Declarado, sin
  implementación real (es un stub que dice "ejecutar HTTP").
- `TypeCode`: skill = código Go inline. ⚠ Declarado, sin
  sandboxing, considerado unsafe.
- `TypeMCPTool`: skill = wrapper sobre una MCP tool externa. ⚠
  Declarado, sin implementación real.

Resultado: 4 tipos en el schema, 1 implementado a fondo, 3 son
"shape without substance". Esto confunde al user (¿puedo crear
un TypeCode? ¿qué pasa si lo hago?), infla el código, y nadie
sabe si vale la pena implementar los 3.

La decisión arquitectónica correcta requiere DATOS (issue
35.4 provee: cuántos skills se crearon de cada tipo, cuántos se
ejecutaron, feedback de users).

## Decisión arquitectónica

**Estrategia:** producir un ADR formal basado en datos +
ejecutar la opción ganadora (que puede ser A o B).

**Opción A: Simplificar a `TypePrompt` único**

Matar los 3 stubs. Beneficios:
- ~500 líneas de código menos (estimado).
- Schema más simple (1 enum en vez de 4).
- Documentación honesta (no prometemos algo que no entregamos).
- El user tiene que escribir el código de TypeAPI/Code/MCPTool
  en otros lugares (e.g. un flow), lo cual es la posición
  honesta ("si querés ejecutar un HTTP endpoint, escribí un
  flow con un step `http_request`").

Costos:
- Users que tienen skills de tipo stub existentes se quedan
  sin funcionar. Mitigación: migration que convierte los stubs
  a `TypePrompt` con un mensaje de error (no es drop silencioso).
- Rompe el contrato público (la API ya aceptaba esos types).
  Mitigación: versioning, deprecation warning, 1 release de
  aviso.

**Opción B: Implementar los 3 stubs**

Commit a entregar valor real. Beneficios:
- 4 tipos útiles, no 1.
- Diferenciador vs otros SaaS de skills.
- Cubre casos de uso reales (HTTP, code, MCP wrapping).

Costos:
- ~2-4 semanas de trabajo.
- TypeCode requiere sandboxing (WASM o similar) para no ser
  una vulnerabilidad. Riesgo de seguridad.
- TypeMCPTool requiere entender bien el protocol MCP (vs solo
  el HTTP, que ya dominamos).

**Decisión: PENDIENTE hasta tener datos de 35.4.**

El ADR tendrá:
- Sección "Contexto" con los 4 tipos y su estado.
- Sección "Datos" con los números de 35.4.
- Sección "Opciones" (A y B) con tradeoffs cuantificados.
- Sección "Decisión" (la que gane).
- Sección "Consecuencias" (qué se hace ahora).

## Alternativas descartadas

| Alt | Idea | Por qué se descarta |
|-----|------|---------------------|
| A | Hacer un REQ separado para cada tipo y dejar la decisión para después | Procrastina. El ADR es la decisión. |
| B | Implementar solo TypeAPI (el más pedido) | Decisión parcial. Si vamos a implementar, todos. |
| C | Dejar los 4 tipos como están y pretender que están bien | La deuda técnica crece. Un dev futuro preguntará "¿qué es TypeCode?" y我们会 tener que responder. |
| D | Eliminar el model de skills entero (no es core) | El user ya usa skills (35.4 confirmará). Out of scope. |

## Por qué ADR formal gana

- **Documentado:** futuros devs lo encuentran.
- **Basado en datos:** 35.4 provee los números.
- **Reversible:** si la decisión se revela mala, nuevo ADR.
- **Honesto:** refleja el estado real del código (no un
  "marketing" de 4 tipos cuando solo 1 funciona).

## Detalle de implementación

- `docs/adr/0035-skill-model.md` con el ADR.
- (Si opción A): migration + cleanup + docs update.
- (Si opción B): nuevo REQ-36 con sub-issues.

## Riesgos

- **R1:** Los datos de 35.4 son insuficientes (pocos skills
  creados todavía). **Mitigación:** el ADR es honesto sobre la
  limitación y propone re-evaluar en 6 meses.
- **R2:** El user quiere 4 tipos pero la realidad es 1.
  **Aceptable:** el ADR documenta el gap y propone cómo
  cerrarlo.
- **R3:** La Opción A (simplificar) rompe a users existentes.
  **Mitigación:** deprecation warning + 1 release de aviso.
