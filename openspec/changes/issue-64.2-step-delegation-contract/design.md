# issue-64.2 — Design

## Decisions

- **A — campos aditivos en PhaseStepSummary.** `RequiredToolCalls []string` y
  `OutputSchema map[string]any`, ambos con `omitempty`. No rompen clientes que
  no los leen.
- **B — validación agregada sin cortar.** Se reemplaza el "return al primer
  error" por un acumulador que corre las tres validaciones (output fields,
  required_saves, tool_calls) y junta los resultados en una respuesta única.
- **Retrocompat de la respuesta.** Se conserva `validation_error` (string) como
  resumen agregado y se agregan listas estructuradas (`missing_fields`,
  `missing_required_saves`, `missing_tool_calls`) para que el cliente ramifique.

## Alternatives

- **A alternativa descartada:** un endpoint separado `domain_orchestrate_contract`
  para consultar el contrato. Descartada: agrega un round-trip; mejor incluirlo
  en el plan que el cliente ya recibe.
- **B alternativa descartada:** dejar la validación de a uno pero cachear los
  errores ya vistos. Descartada: no resuelve el problema, solo lo maquilla.

## Data Flow

1. Definición de fase (registry) → `exportPlan` copia `RequiredToolCalls` +
   `OutputSchema` a cada `PhaseStepSummary` → el cliente los ve en el plan.
2. Cliente reporta fase → validación agregada corre output+saves+tools en una
   pasada → si hay faltantes, devuelve las 3 listas juntas + step queda running.

## TDD Plan

- **Red**: test que reporta una fase con 3 campos faltantes y espera los 3 en la
  respuesta (hoy devuelve 1).
- **Green**: acumulador de validación.
- **Refactor**: extraer la validación agregada a una función testeable.
- **Sabotaje**: hacer que el acumulador corte al primer error → el test de
  "múltiples juntos" debe fallar.
- Análogo para A: test de que exportPlan incluye required_tool_calls/output_schema.

## Risk Mitigation

- Campos aditivos con omitempty.
- Conservar `validation_error` string para clientes viejos.
