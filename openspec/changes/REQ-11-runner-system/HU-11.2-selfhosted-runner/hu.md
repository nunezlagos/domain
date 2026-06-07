# HU-11.2-selfhosted-runner

**Origen:** `REQ-11-runner-system`
**Persona:** platform-engineer
**Prioridad tentativa:** alta
**Tipo:** feature

## Historia de usuario

**Como** usuario con restricciones de infraestructura
**Quiero** ejecutar un agente runner self-hosted que se conecte al servidor Domain y ejecute tareas localmente
**Para** no depender del sandbox cloud y poder correr cargas de trabajo en mi propia infraestructura

## Criterios de aceptación

### Escenario 1: Runner se registra con token válido

```gherkin
Dado un token de registro válido generado desde la UI de administración
Cuando inicio el runner con `domain-runner --token=tk_xxxx --server=wss://domain.example.com`
Entonces el runner establece conexión WebSocket con el servidor
Y envía un mensaje `register` con su token y metadatos (hostname, version, capabilities)
Y el servidor valida el token
Y el servidor responde `registered` con un runner_id asignado
Y el runner queda en estado `idle` esperando tareas
```

### Escenario 2: Token inválido es rechazado

```gherkin
Dado un token inválido o expirado
Cuando el runner intenta registrarse
Entonces el servidor responde con error `invalid_token`
Y el runner muestra "Registration failed: invalid or expired token"
Y el runner termina con código de salida 1
```

### Escenario 3: Runner recibe y ejecuta tarea

```gherkin
Dado un runner registrado en estado `idle`
Cuando el servidor envía un mensaje `execute` con:
  | task_id   | "tk_abc123"               |
  | type      | "code_exec"               |
  | payload   | {"language":"python","code":"print(42)"} |
  | timeout   | 30                        |
Entonces el runner ejecuta el código localmente
Y captura stdout, stderr y exit code
Y envía un mensaje `result` con:
  | task_id   | "tk_abc123" |
  | status    | "success"   |
  | stdout    | "42\n"      |
  | stderr    | ""          |
  | exit_code | 0           |
  | duration  | 0.123       |
```

### Escenario 4: Tarea falla con timeout

```gherkin
Dado un runner ejecutando una tarea con timeout de 5s
Cuando el código ejecuta `sleep(60)`
Entonces el runner cancela la tarea después de 5s
Y envía `result` con status `timeout`
Y el runner vuelve a estado `idle`
```

### Escenario 5: Desconexión y reconexión

```gherkin
Dado un runner conectado y registrado
Cuando la conexión WebSocket se pierde
Entonces el runner intenta reconectar con backoff exponencial (1s, 2s, 4s, 8s, max 30s)
Y envía `reconnect` con su runner_id y token
Y el servidor reasigna las tareas pendientes
Y si después de 5 minutos no reconecta, el servidor marca el runner como `offline`
```

### Escenario 6: Heartbeat mantiene el runner vivo

```gherkin
Dado un runner registrado en estado `idle`
Cuando pasan 30 segundos sin actividad
Entonces el runner envía un heartbeat
Y el servidor responde confirmando
Y el runner permanece en estado `idle`
Si el servidor no recibe heartbeat por 90 segundos
Entonces marca el runner como `offline`
```

### Escenario 7: Capacidades del runner

```gherkin
Dado un runner con Python 3.11 y Node.js 18 instalados
Cuando se registra con `capabilities: {runtimes: ["python3","node"]}`
Entonces el servidor solo asigna tareas compatibles con esos runtimes
Y rechaza tareas que requieran Go si el runner no lo soporta
```

### Escenario 8: Runner tiene sandbox opcional

```gherkin
Dado un runner configurado con `sandbox: docker`
Cuando recibe una tarea code_exec
Entonces ejecuta dentro de un contenedor Docker (reusa HU-11.1)
Y aplica resource limits y network isolation

Dado un runner configurado con `sandbox: none`
Cuando recibe una tarea code_exec
Entonces ejecuta directamente en el host
Y el usuario es responsable del aislamiento
```

## Análisis breve

- **Qué pide realmente:** Un agente Go ligero que se conecta vía WebSocket/gRPC al servidor, recibe tareas, las ejecuta localmente y reporta resultados. Ideal para entornos air-gapped o con requisitos de data residency.
- **Módulos sospechados:** `cmd/domain-runner/`, `internal/runner/agent/`, `internal/runner/connection/`
- **Riesgos / dependencias:** Conexión WebSocket requiere estabilidad de red. El runner ejecuta código arbitrario en el host del usuario. Token security.
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
