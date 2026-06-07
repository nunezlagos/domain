# HU-07.1-context-optimizer

**Origen:** `REQ-07-context-cache`
**Persona:** dx-engineer
**Prioridad tentativa:** alta
**Tipo:** feature

## Historia de usuario
**Como** sistema de contexto de memoria
**Quiero** optimizar dinámicamente la ventana de contexto seleccionando los fragmentos más relevantes dentro de un token budget dado
**Para** maximizar la utilidad de la información presentada al LLM sin exceder los límites de contexto del modelo

## Criterios de aceptación

### Scenario 1: Selección por prioridad (recent > relevant > structured)
**Given** un token budget de 4000 tokens
**And** una pool de contexto con: 3 memorias recientes (1500t), 5 memorias relevantes (3000t), 2 documentos estructurados (1000t)
**When** el optimizer ejecuta la selección
**Then** incluye las 3 memorias recientes primero
**And** luego las memorias relevantes hasta agotar el budget
**And** omite documentos estructurados si no queda presupuesto
**And** retorna un payload de <= 4000 tokens

### Scenario 2: Truncamiento por overflow
**Given** una entrada de 5000 tokens y un budget máximo de 4096 tokens
**When** se aplica la estrategia de truncamiento
**Then** trunca desde el medio del contexto preservando inicio y fin
**And** el resultado no excede 4096 tokens
**And** incluye un marcador `[TRUNCATED ...]` en el punto de corte

### Scenario 3: Budget exacto sin necesidad de truncar
**Given** una entrada de 3000 tokens y un budget de 4096 tokens
**When** se aplica la optimización
**Then** retorna la entrada completa sin modificaciones

### Scenario 4: Pool de contexto vacía
**Given** una pool de contexto vacía
**When** se ejecuta la selección
**Then** retorna un contexto vacío
**And** no lanza error

### Scenario 5: Prioridad "recent" con timestamps iguales
**Given** dos observaciones con el mismo timestamp
**When** se ejecuta la selección
**Then** usa el ID de observación como tiebreaker (mayor ID = más reciente)

## Análisis breve

- **Qué pide realmente:** Un motor de selección contextual que priorice información reciente, luego relevante, luego estructurada, con truncamiento inteligente cuando se excede el budget.
- **Módulos sospechados:** `internal/context/`, `internal/memory/`, `pkg/llm/tokenizer/`
- **Riesgos / dependencias:** Depende de token counter (HU-06.6), embedding similarity (HU-06.5), y del sistema de memoria (REQ-03). El scoring semántico requiere pgvector.
- **Esfuerzo tentativo:** L**
