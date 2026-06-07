# HU-11.1-sandbox-execution

**Origen:** `REQ-11-runner-system`
**Persona:** platform-engineer
**Prioridad tentativa:** alta
**Tipo:** feature

## Historia de usuario

**Como** administrador de la plataforma
**Quiero** ejecutar código no confiable en un sandbox Docker aislado con recursos limitados y timeout
**Para** garantizar que skills y steps de código arbitrario no comprometan el host ni agoten recursos

## Criterios de aceptación

### Escenario 1: Sandbox ejecuta código Python simple

```gherkin
Dado un sandbox Docker configurado con imagen base `domain/sandbox-python:3.11`
Cuando envío un step de tipo `code_exec` con:
  | language | python         |
  | code     | print("hello") |
  | timeout  | 30s            |
Entonces el sandbox se crea desde la imagen
Y el código se copia al contenedor
Y se ejecuta con el runtime correspondiente
Y el output capturado es "hello\n"
Y el contenedor se destruye inmediatamente después
```

### Escenario 2: Timeout mata el proceso

```gherkin
Dado un sandbox con timeout de 5 segundos
Cuando envío código que incluye `time.sleep(60)`
Entonces la ejecución se cancela después de 5 segundos
Y el output contiene "Execution timed out after 5s"
Y el contenedor se destruye
Y el step se marca como `failed` con error `timeout`
```

### Escenario 3: Resource limits se aplican correctamente

```gherkin
Dado una solicitud de ejecución con:
  | cpu_limit    | 0.5   |
  | memory_limit | 256MB |
  | disk_limit   | 1GB   |
Cuando se crea el contenedor sandbox
Entonces el contenedor tiene `--cpus=0.5`
Y el contenedor tiene `--memory=256m`
Y el contenedor tiene `--storage-opt size=1G`
```

### Escenario 4: Network access control

```gherkin
Dado un step con `network: none`
Cuando se crea el contenedor sandbox
Entonces el contenedor se ejecuta con `--network none`
Y cualquier intento de conexión externa falla

Dado un step con `network: internal`
Cuando se crea el contenedor sandbox
Entonces el contenedor se conecta a la red `domain-internal`
Y puede acceder a servicios internos (DB, API)
Y NO puede acceder a Internet

Dado un step con `network: external`
Cuando se crea el contenedor sandbox
Entonces el contenedor tiene acceso a Internet
Y se aplica un proxy de salida para auditoría
```

### Escenario 5: Múltiples ejecuciones concurrentes no interfieren

```gherkin
Dado que envío 10 ejecuciones simultáneas con `sleep(2)` cada una
Cuando todas se ejecutan en paralelo
Entonces cada una se ejecuta en su propio contenedor
Y cada una completa en ~2 segundos
Y los outputs no se mezclan entre ejecuciones
```

### Escenario 6: Error en código se captura correctamente

```gherkin
Dado un sandbox listo para ejecutar
Cuando envío código Python con `raise ValueError("boom")`
Entonces el stderr capturado contiene "ValueError: boom"
Y el step se marca como `failed`
Y el exit code es distinto de 0
Y el contenedor se destruye
```

### Escenario 7: Pull de imagen lazy con caché

```gherkin
Dado que la imagen `domain/sandbox-python:3.11` ya existe localmente
Cuando se solicita una ejecución con esa imagen
Entonces no se ejecuta `docker pull`
Y el contenedor se crea inmediatamente
```

## Análisis breve

- **Qué pide realmente:** Sistema de sandboxing Docker para ejecución aislada de código arbitrario con recursos controlados, timeout y limpieza automática.
- **Módulos sospechados:** `internal/runner/sandbox/`, `internal/runner/docker/`, `internal/models/execution.go`
- **Riesgos / dependencias:** Docker daemon requerido en el host. Imágenes base deben estar disponibles. Riesgo de Docker-in-Docker. Limpieza de contenedores huérfanos en caso de crash.
- **Esfuerzo tentativo:** L

## Verificación previa

- [ ] Revisar codebase (grep)
- [ ] Revisar memorias engram (domain_mem_search)
- [ ] Revisar git log
- [ ] Probar en ambiente correcto
- [ ] Reproducir con perfil correcto
- [ ] Verificar caché / build
- [ ] Verificar feature flag / config

### Resultado de verificación

- **Estado:** pendiente
- **Evidencia:**
- **Acción derivada:**
