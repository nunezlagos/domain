# HU-09.11-reproducibility-snapshots

**Origen:** `REQ-09-flow-system`
**Prioridad tentativa:** media
**Tipo:** feature

## Historia de usuario

**Como** developer debuggeando un run failed en producción
**Quiero** capturar snapshot determinístico de inputs (incluyendo random seeds, timestamps mockables)
**Para** poder replay exacto local sin "no se puede reproducir"

## Criterios de aceptación

### Escenario 1: Snapshot al iniciar run

```gherkin
Dado que se inicia un flow_run
Cuando el motor inicializa el contexto
Entonces se captura snapshot inmutable:
  - flow_version_id congelado
  - inputs (vars + payload + triggered_by)
  - random_seed (uint64 generado o pasado externo)
  - frozen_time (timestamp inicial inmutable)
  - env_vars whitelist (LLM model versions, feature flags)
  - skill_versions de cada skill referenciada
Y se persiste como `flow_runs.snapshot JSONB`
```

### Escenario 2: Replay desde snapshot

```gherkin
Dado que existe run failed con snapshot completo
Cuando POST /api/v1/runs/:id/replay con `{mode:"deterministic"}`
Entonces se crea nuevo run con MISMO snapshot
Y `Math.random` (o equivalente Go) usa seed
Y `time.Now()` retorna frozen_time + offset relativo
Y LLM calls reusan model_version del snapshot
Y outputs (en happy path) coinciden con originales
```

### Escenario 3: Replay con override

```gherkin
Dado que quiero replay con cambio
Cuando POST replay con `{mode:"replay_with_overrides", overrides:{inputs:{...}}}`
Entonces se hace fork del snapshot con overrides aplicados
Y el resto stays determinístico
```

### Escenario 4: LLM no-determinismo controlado

```gherkin
Dado que `llm_call` step se replay
Cuando el motor invoca LLM
Entonces incluye `temperature: snapshot.llm_temperature`
Y si snapshot.cached_responses tiene match, devuelve cached (no llama LLM)
Y si no, llama LLM (warning: replay puede divergir)
```

### Escenario 5: Snapshot opt-out

```gherkin
Dado que `flow.deterministic_replay: false`
Cuando se ejecuta
Entonces snapshot mínimo (solo inputs)
Y replay NO garantiza determinismo
Y storage es menor
```

## Análisis breve

- **Qué pide:** snapshot JSONB + frozen time + seed + LLM cache + replay endpoint
- **Esfuerzo:** L (impacto cross-cutting)
- **Riesgos:** LLM stateful entre versiones del modelo; snapshot bloat
