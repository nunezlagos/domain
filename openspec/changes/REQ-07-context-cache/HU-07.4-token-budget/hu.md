# HU-07.4-token-budget

**Origen:** `REQ-07-context-cache`
**Prioridad tentativa:** alta
**Tipo:** feature

## Historia de usuario
**Como** administrador del sistema de agentes
**Quiero** configurar budgets de tokens por agente/flow con límites hard y soft, tracking durante streaming, y manejo de agotamiento (truncamiento graceful vs error)
**Para** controlar costos, evitar timeouts del modelo y tener visibilidad del consumo en tiempo real

## Criterios de aceptación

### Scenario 1: Hard y soft limits
**Given** un agente con hard_limit=4096 y soft_limit=3072
**When** se ejecuta un run que consume 3500 tokens
**Then** se dispara una advertencia de soft limit (log + callback)
**And** el run continúa normalmente (no se interrumpe)

### Scenario 2: Hard limit alcanzado
**Given** un agente con hard_limit=4096
**When** se ejecuta un run que alcanza 4096 tokens
**Then** la ejecución se interrumpe (streaming se corta)
**And** se registra un error "token_budget_exceeded"
**And** el output parcial se conserva con flag `truncated: true`

### Scenario 3: Budget por modelo
**Given** el modelo gpt-4 tiene un límite de 8192 tokens según el registry
**When** se configura un agente con ese modelo
**Then** el hard_limit máximo permitido es 8192
**And** si se intenta configurar un hard_limit > 8192, se rechaza con error

### Scenario 4: Tracking durante streaming
**Given** un run en streaming que ya consumió 2000 tokens
**When** se consulta el consumo actual
**Then** retorna tokens_usados=2000, budget_restante=2096 (si hard=4096)
**And** retorna porcentaje=48.8%

## Análisis breve

- **Qué pide realmente:** Un sistema de token budget que imponga límites configurables (hard/soft), que conozca los límites por modelo desde el registry, y que trackee el consumo en tiempo real durante ejecuciones streaming.
- **Módulos sospechados:** `internal/budget/`, `internal/llm/`, `internal/model/`, `internal/runner/`
- **Riesgos / dependencias:** Depende de model registry (HU-06.4) para límites por modelo, de token counter (HU-06.6) para medición streaming, y del runner (REQ-11) para corte de ejecución.
- **Esfuerzo tentativo:** M**
