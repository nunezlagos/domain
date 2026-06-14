# Tasks: issue-11.1-sandbox-execution

## Backend

- [x] Implementar `SandboxManager` struct con dependencia `DockerClient` interface
- [x] Implementar `ImageManager.Ensure()` con pull lazy y semáforo de concurrencia
- [x] Implementar `ContainerManager.Create()` con ResourceLimits → HostConfig
- [x] Implementar `ContainerManager.CopyFiles()` para escribir código en container
- [x] Implementar `ContainerManager.Start()` con runtime detection por lenguaje
- [x] Implementar `ContainerManager.Attach()` con buffer de stdout/stderr
- [x] Implementar timeout con `context.WithTimeout` + `ContainerKill`
- [x] Implementar NetworkProfile: none, internal, external
- [x] Implementar `ContainerManager.Destroy()` con cleanup forzoso
- [x] Implementar GC loop con label-based sweep cada 5 minutos
- [x] Implementar cleanup on startup (eliminar orphans previos)
- [x] Implementar cleanup on graceful shutdown
- [x] Definir `SandboxConfig` en config.yaml: default image, default limits, network
- [x] Definir `ExecutionRequest` y `ExecutionResult` structs en `internal/models`
- [x] Integrar `SandboxService` con `RunnerService` para steps `code_exec`
- [x] Definir imágenes base Docker: `domain/sandbox-python`, `domain/sandbox-node`, `domain/sandbox-go`
- [x] Escribir Dockerfiles para cada imagen base (runtime + non-root user)

## Frontend

- [x] (No aplica)

## Tests

- [x] Test unitario: `SandboxManager` con mock DockerClient devuelve output correcto
- [x] Test unitario: timeout cancela ejecución y devuelve `ErrTimeout`
- [x] Test unitario: resource limits se mapean correctamente a HostConfig
- [x] Test unitario: network profiles producen flags correctos
- [x] Test unitario: error en código captura stderr y exit code != 0
- [x] Test unitario: GC loop elimina containers con label correcto
- [x] Test unitario: ImageManager.Ensure() no hace pull si imagen existe
- [x] Test unitario: ImageManager.Ensure() hace pull si imagen no existe
- [x] Test unitario: pull semáforo limita concurrencia
- [x] Test unitario: múltiples ejecuciones concurrentes no comparten estado
- [x] Test integración: ciclo completo contra Docker daemon real
- [x] Test integración: timeout real con `sleep` prolongado
- [x] Test integración: network isolation (curl desde network=none)
- [x] Sabotaje: eliminar llamada a Destroy → test debe fallar (orphan detection)
- [x] Sabotaje: no aplicar timeout → test debe fallar (cuelga)

## Cierre

- [x] Verificación manual: ejecutar code_exec step en UI
- [x] Verificación manual: timeout forzoso desde la UI
- [x] Verificación manual: GC cleanup después de crash simulado
- [x] Suite verde: `go test ./internal/runner/... -tags=integration`
- [x] Documentar sandbox config en docs/operations.md
- [x] Documentar imágenes base y cómo extenderlas
