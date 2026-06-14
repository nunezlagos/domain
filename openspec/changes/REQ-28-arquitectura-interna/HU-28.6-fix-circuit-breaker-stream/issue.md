# HU-28.6-fix-circuit-breaker-stream

**Origen:** `REQ-28-arquitectura-interna`
**Prioridad tentativa:** media
**Tipo:** fix

## Historia de usuario

**Como** operador de Domain
**Quiero** que el circuit breaker registre como fallo cuando un stream de LLM se corta a mitad de camino con error
**Para** que el breaker se abra cuando el provider está dando respuestas parciales consistentemente, no solo cuando falla el handshake inicial

## Contexto

En `internal/llm/circuitbreaker/breaker.go:159-172`, el método `CompleteStream` ejecuta una goroutine que lee del channel de streaming. La variable `sawError` se setea cuando hay un error mid-stream, pero en la línea 171 se encuentra `_ = sawError` — la variable se lee (para silenciar el warning de compilador) y se descarta. El breaker nunca llama a `recordFailure()`.

Esto significa que si un provider de LLM empieza bien pero falla a mitad de un response largo (ej: timeout parcial, inestabilidad), el breaker no lo detecta. Solo se detectan fallos en el handshake inicial (que pasan por `Complete`).

## Criterios de aceptación

### Escenario 1: Stream error registra fallo

```gherkin
Dado un provider que responde 3 chunks bien y el 4to falla
Cuando el breaker recibe el error
Entonces llama a recordFailure()
Y el breaker incrementa el contador de fallos
```

### Escenario 2: Stream exitoso no registra falso fallo

```gherkin
Dado un provider que responde todos los chunks correctamente
Cuando el stream termina sin error
Entonces el breaker NO llama recordFailure()
Y el contador de fallos no se incrementa
```

### Escenario 3: Sabotaje — stream con error deliberado

```gherkin
Dado un test que inyecta un error en el channel de stream
Cuando CompleteStream procesa el stream
Entonces el breaker entra en estado "open" después de N errores
```

## Análisis breve

- **Qué pide:** Reemplazar `_ = sawError` por `if sawError { cb.recordFailure() }`
- **Módulos afectados:** `internal/llm/circuitbreaker/breaker.go`
- **Esfuerzo tentativo:** XS (4 horas)
- **Dependencias:** Ninguna
