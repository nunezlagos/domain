# HU-07.2-cross-session-stitch

**Origen:** `REQ-07-context-cache`
**Prioridad tentativa:** alta
**Tipo:** feature

## Historia de usuario
**Como** agente de memoria ejecutando una nueva sesión
**Quiero** recibir automáticamente un resumen cosido de sesiones anteriores (decisiones clave, items abiertos, contexto relevante) sin duplicar información repetida
**Para** mantener continuidad contextual entre sesiones y evitar preguntar/presentar lo mismo dos veces

## Criterios de aceptación

### Scenario 1: Stitching básico
**Given** 3 sesiones previas con resúmenes almacenados
**When** se inicia una nueva sesión y se solicita el stitching
**Then** el stitch producer combina los 3 resúmenes en un único bloque
**And** el bloque incluye secciones: "Decisiones tomadas", "Items abiertos", "Contexto recurrente"

### Scenario 2: Dedup de memorias repetidas
**Given** la sesión S1 registró "decidimos usar Go" como decisión
**And** la sesión S2 también registró "decidimos usar Go" (misma semantic key)
**When** se ejecuta el stitch
**Then** "decidimos usar Go" aparece solo una vez en el output
**And** se marca como "[confirmado en S1, S2]"

### Scenario 3: Sin sesiones previas
**Given** no hay sesiones anteriores (primera ejecución)
**When** se solicita el stitching
**Then** retorna un bloque vacío
**And** no lanza error

### Scenario 4: Límite de sesiones a stitch
**Given** 20 sesiones previas pero el límite configurado es 5
**When** se ejecuta el stitch
**Then** incluye solo las 5 sesiones más recientes
**And** indica cuántas sesiones quedaron fuera

## Análisis breve

- **Qué pide realmente:** Un mecanismo que, al iniciar una sesión, consulte resúmenes de sesiones anteriores y los fusione inteligentemente, eliminando duplicados semánticos y priorizando lo reciente.
- **Módulos sospechados:** `internal/session/`, `internal/context/`, `internal/memory/`
- **Riesgos / dependencias:** Depende de session summaries (HU-03.2), del sistema de memoria (REQ-03), y de dedup engine (HU-03.6). La dedup semántica requiere embeddings (HU-06.5).
- **Esfuerzo tentativo:** L**
