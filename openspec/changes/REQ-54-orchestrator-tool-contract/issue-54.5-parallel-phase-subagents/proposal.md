# Como agente cliente ejecutando una fase del SDD, recibo en el prompt de la fase un plan de subagentes paralelos (roles, contexto por subagente, estrategia de merge) para fanear MIS subagentes en las fases que se benefician (explore/verify/judge), de modo que las fases dejen de ser monolíticas sin cambiar el modelo económico client-driven.

## Why

Las fases del SDD hoy son monolíticas: un solo hilo de atención del agente por
fase. Explore pierde cobertura (un solo recorrido del codebase), judge pierde
rigor (un solo juez = sin adversarialidad). El cliente (Claude Code) YA tiene
subagentes paralelos nativos — el patrón judgment-day del propio usuario lo
prueba — pero el orquestador no los aprovecha: nada le dice al agente "acá
faneá N".

La arquitectura correcta ya está decidida: "Server has NO LLM — fan-out via
client subagents". Esta issue implementa exactamente eso: el server DIRIGE el
fan-out (declara el plan), el cliente lo EJECUTA con su LLM potente.

## Scope

- `SubagentPlan` por fase: default en el handler Go (campo en Output, mismo
  patrón que RequiredToolCalls de 54.1) + override editable en
  `agent_templates.metadata.subagent_plan`.
- Inyección en el user_prompt de la fase (ambos puntos: plan inicial + lazy
  rebuild, misma dualidad que prepared_context de 54.2).
- El prepared_context de 54.2 se referencia en el plan para que cada subagente
  reciba el contexto pertinente.
- Pilotos: sdd-explore (N Explore por área detectada), sdd-verify (escenarios
  Gherkin en paralelo), sdd-judge (panel adversarial 2+ jueces ciegos).

Fuera de alcance (fase 2, issue aparte si se quiere): fan-out server-side de
fases read-only en modo async vía el steptype `parallel` del flowrunner con
MiniMax.

## Approach

Todo declarativo y por texto inyectado: el plan es una instrucción estructurada
en el prompt, el cliente decide los detalles de ejecución (cuántos subagentes
según su límite, qué tipo). El server no puede forzar el fan-out — puede
verificar indicios vía el contrato de 54.1 (p.ej. judge reporta N verdicts).

## Risks

- El agente ignora el plan → mitigación: el Output de la fase puede exigir
  shape que solo sale del fan-out (ej: `judge_verdicts: [..] con length>=2`,
  validado por handler.Validate — teeth reales).
- Costo: más subagentes = más tokens del cliente → el plan declara N sugerido
  y el cliente puede reducirlo; nunca es obligatorio superar su presupuesto.

## Testing

Unit: SubagentPlan fluye handler→PhaseStep→Inputs→prompt (ambos puntos);
override de template gana; fases sin plan = prompt sin bloque (no-op).
Validate de judge exige >=2 verdicts.
