# Tasks: issue-09.5-cloud-autosync

## Backend

- [ ] **B1: Crear paquete `internal/cloud/autosync/`**
      - `manager.go` — AutosyncManager struct + Start/Stop
      - `state.go` — SyncPhase, ReasonCode enums
      - `backoff.go` — backoff schedule + nextDelay
      - `errors.go` — classifyError

- [ ] **B2: Implementar state machine**
      - SyncPhase enum con 7 estados
      - Transiciones válidas validadas

- [ ] **B3: Implementar AutosyncManager**
      - Start(ctx) lanza goroutine loop
      - Stop() cierra done channel
      - loop con select: ctx.Done, m.done, ticker.C

- [ ] **B4: Implementar runCycle**
      - pushing → puller.Push → pulling → puller.Pull → healthy
      - Error path: failed → backoff → retry

- [ ] **B5: Implementar backoff schedule**
      - Slice defaultBackoff
      - nextDelay() retorna duración actual y avanza índice
      - Reset en healthy

- [ ] **B6: Implementar classifyError**
      - Mapear errores a ReasonCode
      - network_error, auth_error, server_error, timeout, rate_limited

- [ ] **B7: Implementar GET /api/cloud/sync-status**
      - Handler que expone SyncStatus con RLock
      - Incluir en router

- [ ] **B8: Integrar ENGRAM_CLOUD_AUTOSYNC env var**
      - Parsear al inicio ("1"/"true" → enabled)
      - Si no está seteado o es "0"/"false" → PhaseDisabled

- [ ] **B9: Implementar SyncPusher y SyncPuller interfaces**
      - `SyncPusher { Push(ctx) error }`
      - `SyncPuller { Pull(ctx) error }`
      - Implementaciones concretas que llaman cloud server API

## Tests

- [ ] **T1: Ciclo exitoso idle→pushing→pulling→healthy**
      ```go
      func TestCycleSuccess(t *testing.T) {
          m := NewAutosyncManager(WithPusher(mockSuccessPusher), WithPuller(mockSuccessPuller))
          m.Start(context.Background())
          defer m.Stop()
          assert.Eventually(t, func() bool {
              return m.Status().Phase == PhaseHealthy
          }, 2*time.Second, 100*time.Millisecond)
      }
      ```

- [ ] **T2: Push error → failed → backoff**
- [ ] **T3: Backoff schedule respeta secuencia**
      ```go
      func TestBackoffSchedule(t *testing.T) {
          b := newBackoff()
          assert.Equal(t, 30*time.Second, b.nextDelay())
          assert.Equal(t, 60*time.Second, b.nextDelay())
          assert.Equal(t, 120*time.Second, b.nextDelay())
          assert.Equal(t, 240*time.Second, b.nextDelay())
          assert.Equal(t, 300*time.Second, b.nextDelay()) // max
      }
      ```

- [ ] **T4: Disabled no ejecuta ciclos**
- [ ] **T5: classifyError mapea correctamente**
- [ ] **T6: Status endpoint retorna JSON correcto**
- [ ] **T7: Sabotaje — no resetear backoffIdx en healthy → backoff sigue creciendo → test cae → restaurar**

## Cierre

- [ ] `go build ./...` sin errores
- [ ] `go vet ./...` sin warnings
- [ ] `go test ./internal/cloud/autosync/... -v`
- [ ] Commit: `feat: background autosync manager with state machine and backoff`
