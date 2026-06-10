# Design: issue-11.1-sandbox-execution

## Decisión arquitectónica

```
┌─────────────────────────────────────────────────┐
│                 SandboxManager                   │
│  ┌─────────────┐  ┌────────────┐  ┌──────────┐  │
│  │ ImageManager│  │ContainerMgr│  │  GC Loop │  │
│  │ - pull()    │  │ - create() │  │ - sweep() │  │
│  │ - exists()  │  │ - exec()   │  │          │  │
│  │ - cache()   │  │ - attach() │  └──────────┘  │
│  └─────────────┘  │ - destroy()│                 │
│                   └────────────┘                 │
│  ┌──────────────────────────────────────────┐    │
│  │           DockerClient (SDK)             │    │
│  │   github.com/docker/docker/client        │    │
│  └──────────────────────────────────────────┘    │
│                         │                        │
└─────────────────────────┼────────────────────────┘
                          │ /var/run/docker.sock
                 ┌────────┴────────┐
                 │   Docker Daemon │
                 └─────────────────┘
```

**Decisión:** Interface `DockerClient` para testabilidad. Implementación concreta usa SDK oficial. `SandboxManager` coordina el ciclo de vida completo. GC loop independiente con label-based cleanup.

## Alternativas descartadas

| Alternativa | Motivo de descarte |
|---|---|
| Firecracker / microVM | Overkill para ejecución de scripts simples, mayor latencia de startup |
| gVisor | Más seguro pero complejidad operativa alta, no disponible en todos los kernels |
| nsjail+seccomp | Más liviano pero no da las mismas garantías de resource limits y red |
| Docker-in-Docker con volumen | Añade complejidad sin beneficio real, preferimos socket bind mount |
| Kubernetes Jobs | Overkill operativo para un feature de ejecución de código; K8s se考虑 para futura orquestación |

## Diagrama

```
┌──────────┐     Execute(req)     ┌─────────────┐
│  Runner   │────────────────────►│SandboxManager│
│  Service  │                     │              │
│           │◄────────────────────│              │
└──────────┘    Result{Output,    └──────┬───────┘
                 ExitCode, Error}        │
          ┌──────────────────────────────┼──────────────┐
          │                1. Pull/Check │              │
          │                2. Create Ctr │    Docker    │
          │                3. Copy files │    Daemon    │
          │                4. Attach     │              │
          │                5. Start+Wait │              │
          │                6. Read logs  │              │
          │                7. Destroy    │              │
          └──────────────────────────────┘              │
```

**Flujo:**
1. `SandboxManager.Execute()` recibe `ExecutionRequest` con código, lenguaje, timeout, resources, network
2. `ImageManager.Ensure()` verifica si la imagen existe localmente; si no, hace pull con semáforo
3. `ContainerManager.Create()` crea container con config de resources y network
4. `ContainerManager.CopyFiles()` escribe el código dentro del container
5. `ContainerManager.Start()` ejecuta el runtime correspondiente
6. Goroutine escucha timeout: si expira, `ContainerKill` y error
7. `ContainerManager.Attach()` captura stdout/stderr en buffers
8. Container exit: se leen los buffers y exit code
9. `ContainerManager.Destroy()` elimina el container
10. Se devuelve `ExecutionResult`

## TDD plan

1. **Red:** Test que `SandboxManager.Execute()` devuelve output de un `print("hello")`
2. **Green:** Implementar con mock DockerClient que devuelve output simulado
3. **Refactor:** Extraer ImageManager, ContainerManager interfaces
4. **Red:** Test que timeout cancela ejecución y devuelve error
5. **Green:** Implementar context.WithTimeout + kill en el mock
6. **Red:** Test que resource limits se pasan correctamente al HostConfig
7. **Green:** Implementar mapeo de ResourceLimits → Docker HostConfig
8. **Red:** Test que network=none produce `--network none`
9. **Green:** Implementar NetworkProfile → HostConfig.NetworkMode
10. **Sabotaje:** Romper timeout, test debe fallar; restaurar fix
11. **Integration:** Test real contra Docker daemon para validar ciclo completo

## Riesgos y mitigación

- **Container escape:** `--cap-drop ALL`, `--security-opt no-new-privileges`, read-only rootfs, non-root user dentro del container
- **Orphan containers:** Label `domain-sandbox` en todos los containers, GC loop cada 5 minutos, cleanup on graceful shutdown, cleanup on startup
- **Image pull flooding:** Semáforo con max 3 pulls simultáneos, cache local con `ImageExists` check
- **Disk space:** `--storage-opt size=1G` por container, alertas cuando disk usage > 80%
- **Docker socket security:** Nunca exponer el socket a la red, solo acceso local/unix socket
