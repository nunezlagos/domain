# Como plataforma domain, arreglo los cuatro puntos que hoy aparentan funcionar y no funcionan (async sin worker, key LLM que no llega al proceso Go, wiring divergente entre binarios, código muerto de orquestación) para que el estado del sistema sea honesto y el trabajo desatendido con Minimax sea posible.

## Why

La auditoría identificó cuatro cosas que dan falsa sensación de completitud. Ninguna es un
rediseño; son gaps de integración que erosionan la confianza en el sistema y bloquean el
único caso donde Minimax server-side vale la pena: trabajo desatendido (sin cliente
esperando). El flujo interactivo NO usa esto — el cliente ya avanza solo con su propio LLM.

## Scope

Cuatro fixes independientes:

1. **Async worker.** `ProcessAsyncFlowRun` (`async.go:69`) está completo pero **no lo llama
   nadie** en prod. Agregar el disparador (goroutine post-`runAsync` o worker que haga tail
   de flow_runs `pending` en modo async). Sin esto, los flows async quedan `pending` para
   siempre.

2. **Key LLM en el proceso Go.** `services/.env` tiene `MINIMAX_API_KEY` real, pero el
   bloque `environment` de `domain-mcp` en su `docker-compose.yml` NO la pasa (sí llega a
   Django). Agregar `LLM_API_KEY`/aliases al compose. Trivial pero enciende rerank,
   inferencia de aristas, judge y (con 54.2) la preparación inteligente en el server Go.

3. **Wiring divergente entre binarios.** `cmd/domain-mcp/main.go` no recibe Spec/Tasks/
   IssueSvc, así que `persistOpenspec` es no-op silencioso ahí; `cmd/domain` sí materializa.
   Unificar el wiring o documentar/eliminar el binario secundario.

4. **Código muerto de orquestación.** `agent/orchestration/*` (supervisor, sequential,
   parallel, handoff) tiene tests pero cero call-sites. Integrar o eliminar; y aclarar la
   confusión de nombres `agents` vs `agent_templates`.

## Approach

Cambios quirúrgicos, uno por fix, cada uno commiteable por separado. El fix #1 (worker) es
el de más sustancia; #2 es una línea de compose; #3 y #4 son limpieza.

## Risks

- Async worker mal acotado podría procesar flows que no debía → arrancar con opt-in
  explícito (solo flows creados con `Mode: async`) y un límite de concurrencia.
- Encender Minimax en el proceso Go consume la key (costo) → ya está pagada y en uso por
  Django; el consumo del server Go es de tools baratas.

## Testing

Worker: flow async → procesado hasta `completed`/`failed`, no queda `pending`. Compose:
verificar que el proceso ve la key (test de arranque / health). Wiring: `persistOpenspec`
materializa en el binario que se despliega.
