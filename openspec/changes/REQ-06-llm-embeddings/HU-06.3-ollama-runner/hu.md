# HU-06.3-ollama-runner

**Origen:** `REQ-06-llm-embeddings`
**Prioridad tentativa:** media
**Tipo:** feature

## Historia de usuario

**Como** desarrollador que trabaja offline o en desarrollo
**Quiero** un runner para Ollama que me permita usar modelos LLM locales configurando solo `DOMAIN_OLLAMA_URL`
**Para** poder desarrollar y testear sin depender de APIs externas ni generar costos

## Criterios de aceptación

### Escenario 1: Ollama runner - completion básico

```gherkin
Dado que DOMAIN_OLLAMA_URL=http://localhost:11434 está configurada
Y que Ollama está corriendo localmente con el modelo "llama3.2"
Cuando llamo a `ollamaRunner.Complete(ctx, "Hola", CompletionOpts{Model: "llama3.2"})`
Entonces recibo un Response con Content no vacío
Y `Model` es "llama3.2"
Y `FinishReason` es "stop"
```

### Escenario 2: Ollama runner con streaming

```gherkin
Cuando llamo a `CompleteStream(ctx, "Cuenta un cuento", opts)`
Entonces recibo chunks progresivamente
Y el último chunk tiene `Done=true`
```

### Escenario 3: Modelo no descargado

```gherkin
Dado que el modelo "modelo-inexistente" no está descargado en Ollama
Cuando llamo a `Complete` con ese modelo
Entonces recibo error "model not found: modelo-inexistente"
```

### Escenario 4: Ollama no disponible

```gherkin
Dado que Ollama no está corriendo en localhost:11434
Cuando llamo a `Complete`
Entonces recibo error "connection refused" o timeout
```

### Escenario 5: URL personalizada

```gherkin
Dado que DOMAIN_OLLAMA_URL=http://ollama.internal:11434
Cuando el runner se inicializa
Entonces las llamadas van a http://ollama.internal:11434/api/generate
```

### Escenario 6: Pull automático de modelo (opcional)

```gherkin
Dado que el modelo "llama3.2" no está descargado
Cuando se configura `OLLAMA_AUTO_PULL=true`
Y llamo a `Complete`
Entonces el runner intenta hacer pull del modelo automáticamente
Y luego ejecuta el completion
```

### Escenario 7: Registro en factory

```gherkin
Dado que el runner de Ollama está registrado
Cuando `factory.Get("ollama")`
Entonces retorna un *OllamaRunner
```

### Escenario 8: Timeout en generación local

```gherkin
Dado que el modelo local tarda > 120s en generar
Cuando llamo a `Complete` con timeout de 10s
Entonces recibo error "context deadline exceeded"
```

## Análisis breve

- **Qué pide realmente:** Runner para Ollama que implementa la interfaz Provider. Se conecta a la API REST de Ollama en `localhost:11434` por defecto. Soporta cualquier modelo descargado localmente.
- **Módulos sospechados:** `internal/llm/providers/ollama.go`
- **Riesgos / dependencias:** Depende de Ollama instalado en el entorno. La URL debe ser configurable. Pull automático puede tomar varios minutos.
- **Esfuerzo tentativo:** S

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
