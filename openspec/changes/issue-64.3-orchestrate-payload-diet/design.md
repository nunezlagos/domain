# issue-64.3 — Design

## Decisions

- **Lazy SystemPrompt simétrico al UserPrompt.** full.go ya deja el UserPrompt
  de los steps 2..N vacío; se aplica el mismo criterio al SystemPrompt en
  `hydrateSystemPrompts`. Consistente con el patrón existente.
- **Reconstrucción desde step.Inputs, no recompute.** El SystemPrompt ya se
  persiste en `step.Inputs["system_prompt"]` (repository.go:619). phase_result
  lo LEE de ahí para el siguiente step, evitando recomputar buildRulesBlock y
  garantizando que el rulesBlock sea el mismo que al crear el plan.
- **Campo nuevo, no romper NextStepPrompt.** Se agrega `NextStepSystemPrompt`
  (o análogo) al PhaseResultResult; `NextStepPrompt` sigue siendo el user prompt.

## Alternatives

- **Recomputar buildRulesBlock en phase_result** — descartada: duplica trabajo y
  puede divergir del plan original si cambian las policies mid-flow. Leer de
  step.Inputs es más fiel y barato.
- **Cambiar NextStepPrompt a estructura {system, user}** — descartada: rompe
  clientes que lo leen como string. Campo aditivo es retrocompatible.
- **Policies por slug** (la otra mitad de R4) — fuera de scope por decisión del
  usuario; mayor superficie de cambio en un solo release.

## Data Flow

1. orchestrate → hydrateSystemPrompts hidrata solo step 0 → exportPlan manda
   step 0 con sys+user, steps 1..N solo id+slug. El sys prompt de todos se
   persiste en step.Inputs (sin cambio).
2. phase_result de la fase K → lee step.Inputs["system_prompt"] del step K+1 →
   devuelve NextStepSystemPrompt + NextStepPrompt (user, ya existente).

## TDD Plan

- **Red**: test de que exportPlan/hydrate deja vacío el SystemPrompt de steps
  2..N en full (hoy viene lleno).
- **Green**: guard `i == 0` en hydrateSystemPrompts.
- **Red**: test de que phase_result devuelve NextStepSystemPrompt no vacío.
- **Green**: leer step.Inputs y poblar el campo.
- **Sabotaje**: hacer que hydrate llene todos → el test de "vacío en 2..N" falla.

## Risk Mitigation

- Leer de step.Inputs (fuente persistida) para fidelidad del rulesBlock.
- Campo aditivo para retrocompat.
