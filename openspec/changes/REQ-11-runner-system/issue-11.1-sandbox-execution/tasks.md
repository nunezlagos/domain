# Tasks: issue-11.1-sandbox-execution

## Backend

- [ ] Implementar `SandboxManager` struct con dependencia `DockerClient` interface
- [ ] Implementar `ImageManager.Ensure()` con pull lazy y semáforo de concurrencia
- [ ] Implementar `ContainerManager.Create()` con ResourceLimits → HostConfig
- [ ] Implementar `ContainerManager.CopyFiles()` para escribir código en container
- [ ] Implementar `ContainerManager.Start()` con runtime detection por lenguaje
- [ ] Implementar `ContainerManager.Attach()` con buffer de stdout/stderr
- [ ] Implementar timeout con `context.WithTimeout` + `ContainerKill`
- [ ] Implementar NetworkProfile: none, internal, external
- [ ] Implementar `ContainerManager.Destroy()` con cleanup forzoso
- [ ] Implementar GC loop con label-based sweep cada 5 minutos
- [ ] Implementar cleanup on startup (eliminar orphans previos)
- [ ] Implementar cleanup on graceful shutdown
- [ ] Definir `SandboxConfig` en config.yaml: default image, default limits, network
- [ ] Definir `ExecutionRequest` y `ExecutionResult` structs en `internal/models`
- [ ] Integrar `SandboxService` con `RunnerService` para steps `code_exec`
- [ ] Definir imágenes base Docker: `domain/sandbox-python`, `domain/sandbox-node`, `domain/sandbox-go`
- [ ] Escribir Dockerfiles para cada imagen base (runtime + non-root user)

## Frontend

- [ ] (No aplica)

## Tests

- [ ] Test unitario: `SandboxManager` con mock DockerClient devuelve output correcto
- [ ] Test unitario: timeout cancela ejecución y devuelve `ErrTimeout`
- [ ] Test unitario: resource limits se mapean correctamente a HostConfig
- [ ] Test unitario: network profiles producen flags correctos
- [ ] Test unitario: error en código captura stderr y exit code != 0
- [ ] Test unitario: GC loop elimina containers con label correcto
- [ ] Test unitario: ImageManager.Ensure() no hace pull si imagen existe
- [ ] Test unitario: ImageManager.Ensure() hace pull si imagen no existe
- [ ] Test unitario: pull semáforo limita concurrencia
- [ ] Test unitario: múltiples ejecuciones concurrentes no comparten estado
- [ ] Test integración: ciclo completo contra Docker daemon real
- [ ] Test integración: timeout real con `sleep` prolongado
- [ ] Test integración: network isolation (curl desde network=none)
- [ ] Sabotaje: eliminar llamada a Destroy → test debe fallar (orphan detection)
- [ ] Sabotaje: no aplicar timeout → test debe fallar (cuelga)

## Cierre

- [ ] Verificación manual: ejecutar code_exec step en UI
- [ ] Verificación manual: timeout forzoso desde la UI
- [ ] Verificación manual: GC cleanup después de crash simulado
- [ ] Suite verde: `go test ./internal/runner/... -tags=integration`
- [ ] Documentar sandbox config en docs/operations.md
- [ ] Documentar imágenes base y cómo extenderlas
