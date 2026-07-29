# issue-54.3 async worker + fixes — Tasks

## Fix 1 — Async worker (sustancia)
- [ ] `repo.ClaimNextPendingAsyncFlow(ctx)` con lock (SELECT ... FOR UPDATE SKIP LOCKED).
- [ ] `asyncWorker(ctx)` goroutine: loop de polling → claim → `ProcessAsyncFlowRun`.
- [ ] Lanzarla al arranque del server SOLO si `LLM != nil`; loguear si no arranca.
- [ ] Límite de concurrencia configurable (`ASYNC_WORKER_CONCURRENCY`).
- [ ] Tests 1-4 del TDD Plan.

## Fix 2 — Key LLM en el proceso Go (trivial)
- [ ] Agregar al `environment` de `domain-mcp` en `services/domain-mcp/docker-compose.yml`:
      `LLM_PROVIDER`, `LLM_API_KEY`, `LLM_MODEL` (+ aliases `MINIMAX_*`), vía `${VAR}`.
- [ ] Confirmar que `services/.env` tiene las vars (ya las tiene).
- [ ] Smoke: arrancar el container y verificar que `domain_health` reporta el LLM activo.

## Fix 3 — Wiring de binarios
- [ ] Extraer el armado del orquestador (con Spec/Tasks/IssueSvc) a una función compartida.
- [ ] Usarla en `cmd/domain/server_services.go` y `cmd/domain-mcp/main.go`.
- [ ] Test de wiring: el orquestador construido tiene Spec/Tasks/IssueSvc no-nil.
- [ ] Si `cmd/domain-mcp` es legacy: documentar deprecación en su README/comentario.

## Fix 4 — Código muerto y nombres
- [ ] Confirmar cero call-sites de `agent/orchestration/*` (grep).
- [ ] Eliminar el paquete (o documentar plan de uso concreto si se conserva).
- [ ] Documentar la distinción `agents` (ejecución, domain_agent_run) vs `agent_templates`
      (system_prompts de fases) en un comentario/README del orquestador.

## Orden sugerido de commits
1. Fix 2 (compose) — enciende Minimax en el server, prerequisito de todo lo demás.
2. Fix 3 (wiring) — base sana.
3. Fix 1 (worker) — la sustancia.
4. Fix 4 (limpieza) — al final, reversible.
