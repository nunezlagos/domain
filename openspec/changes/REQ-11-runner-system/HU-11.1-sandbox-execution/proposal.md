# Proposal: HU-11.1-sandbox-execution

## Intención

Implementar un sandbox Docker para ejecución segura y aislada de código arbitrario en steps `code_exec` y ejecución de skills. Cada ejecución recibe un contenedor efímero con recursos limitados (CPU, memoria, disco), timeout forzoso, control de red y limpieza automática post-ejecución.

## Scope

**Incluye:**
- Docker manager: pull de imágenes, creación/destrucción de contenedores, copia de archivos
- Resource limits configurables por ejecución (CPU, memory, disk)
- Timeout enforcement con kill del proceso y cleanup
- Network profiles: none (aislado total), internal (solo red interna), external (con proxy de salida)
- Captura de stdout/stderr, exit code, duración
- Soporte multi-lenguaje vía imágenes base: python, node, go
- Image cache: skip pull si ya existe localmente
- Limpieza de contenedores huérfanos (heartbeat/GC)
- API interna: `SandboxService` con método `Execute(ctx, req) -> (*Result, error)`
- Configuración vía `sandbox:` sección en config.yaml

**No incluye:**
- Self-hosted runner (HU-11.2)
- Execution streaming (HU-11.3)
- GPU support
- Volúmenes persistentes entre ejecuciones
- Windows containers

## Enfoque técnico

1. Usar `docker/docker/client` (SDK oficial Go) para interactuar con Docker daemon vía socket
2. `SandboxManager` struct que maneja el pool de Docker client
3. `SandboxConfig` con ResourceLimits, Timeout, NetworkProfile, Image
4. Cada ejecución crea un container con `container.Config` + `container.HostConfig`
5. Timeout implementado con `context.WithTimeout` + `ContainerKill` si expira
6. Captura de output con `ContainerAttach` (stdout+stderr)
7. Network profiles: `none` → `--network none`, `internal` → `--network domain-internal`, `external` → `--network bridge`
8. GC goroutine cada 5 minutos que elimina contenedores con label `domain-sandbox` más viejos de 1 hora
9. Las imágenes base se definen en config y pueden extenderse vía Dockerfile custom

## Riesgos

- **Docker socket expuesto:** Quien acceda al socket puede escapar del sandbox. Mitigación: el servicio solo escucha en internal network, autenticación requerida.
- **Docker-in-Docker:** Si Domain corre dentro de Docker, mount del socket del host. Mitigación: documentado como requisito, socket bind mount.
- **Container escape:** Kernel exploits. Mitigación: `--security-opt no-new-privileges`, `--cap-drop ALL`, read-only rootfs.
- **Orphan containers:** Si Domain crashea, los contenedores quedan vivos. Mitigación: GC con label, cleanup on startup.
- **Image pull flooding:** Muchas ejecuciones concurrentes pueden saturar el pull. Mitigación: image cache y pull semáforo.

## Testing

- Unit tests con Docker mock (interfaz `DockerClient` interface)
- Integration tests contra Docker real (tag: `integration`)
- Timeout: código que hace sleep largo y verificar cancelación
- Resource limits: verificar con `docker inspect` que los flags se aplicaron
- Network isolation: intentar curl a external desde container `none`
- Concurrente: 20 ejecuciones en paralelo, verificar no interferencia
- GC: crear container manual, esperar GC cycle, verificar eliminación
