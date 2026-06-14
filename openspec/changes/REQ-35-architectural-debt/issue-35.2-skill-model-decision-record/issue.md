# issue-35.2-skill-model-decision-record

**Origen:** `REQ-35-architectural-debt`
**Prioridad tentativa:** baja
**Tipo:** ADR (Architectural Decision Record)

## Historia de usuario

**Como** tech lead de domain
**Quiero** tener una decisión arquitectónica formal (ADR) sobre el modelo de skills: ¿simplificamos a `TypePrompt` único, o implementamos los 3 stubs faltantes (`TypeAPI`, `TypeCode`, `TypeMCPTool`)?
**Para** dejar de mantener 4 tipos de los cuales solo 2 están implementados, sin saber si vale la pena el trabajo de implementar los 2 faltantes

## Criterios de aceptación

### Escenario 1: ADR documenta ambas opciones con tradeoffs

```gherkin
Dado que se necesita decidir entre:
  Opción A: Simplificar a TypePrompt único (kill TypeAPI/TypeCode/TypeMCPTool)
  Opción B: Implementar los 3 stubs (commit a entregar valor SaaS real)
Cuando se escribe el ADR
Entonces tiene las secciones estándar:
  - Contexto
  - Decisión (con la opción elegida)
  - Consecuencias (positivas y negativas)
  - Alternativas consideradas
Y los tradeoffs están cuantificados (no "más simple" — cuánto más
simple, en qué métrica)
```

### Escenario 2: Análisis basado en uso real

```gherkin
Dado que tenemos datos de producción de los últimos 30 días (issue-35.4)
Cuando se evalúan las opciones
Entonces el ADR incluye:
  - Cuántos skills de cada tipo se crearon
  - Cuántos se ejecutaron server-side
  - Qué feedback tienen los users
  - Si hay demanda explícita de los tipos faltantes (issues,
    soporte, sales)
Y la decisión se basa en esos datos, no en especulación
```

### Escenario 3: Si simplificación gana → migration plan

```gherkin
Dado que se elige Opción A (simplificar)
Cuando se ejecuta
Entonces:
  - Migration que dropea las columnas vacías (TypeAPI/TypeCode/TypeMCPTool).
  - OpenAPI spec regenerado (32.3) sin esos types.
  - Docs actualizadas.
  - SDK TS regenerado (32.4) sin esos types.
  - Tests que TypeAPI/TypeCode/TypeMCPTool ya no son aceptados en
    create/update.
Y el código se reduce (~500 líneas según estimación).
```

### Escenario 4: Si implementación gana → REQ separado

```gherkin
Dado que se elige Opción B (implementar)
Cuando se ejecuta
Entonces:
  - Se crea un REQ-36 con sub-issues por cada tipo faltante.
  - Cada sub-issue tiene spec completa con Gherkin + design.
  - El REQ-35.2 (este) queda como "decisión tomada" + link al REQ-36.
Y NO se implementa nada en este issue (solo la decisión).
```

### Escenario 5: ADR archivado y consultable

```gherkin
Dado que se toma la decisión
Cuando se commitea
Entonces el archivo está en `docs/adr/0035-skill-model.md` (o
similar, siguiendo la convención de ADRs del repo)
Y está linkeado desde el README
Y futuros devs que tocan el skill model lo encuentran
```

### Escenario 6: Revisión periódica de la decisión

```gherkin
Dado que pasan 6 meses desde la decisión
Cuando se hace review
Entonces el equipo evalúa:
  - ¿La decisión fue correcta?
  - ¿Hay nueva evidencia que la invertiría?
  - Si sí → nuevo ADR que la revierte o ajusta.
```

### Escenario 7: Sabotaje — ADR sin datos de 35.4

```gherkin
Dado que el equipo TIENE presión de tiempo para cerrar la spec
Y el código de "evaluación" (sabotaje) skipea los datos de 35.4
Y escribe el ADR basándose solo en opinión/intuición
Cuando se commitea
Entonces el ADR tiene secciones "Decisión" y "Consecuencias" pero
NO tiene la sección "Datos" o las menciones a "35.4" o
"ejecuciones/mes"
Y el test e2e que assserta "el ADR cita datos cuantitativos de
35.4" DEBE FALLAR
Cuando se restaura la evaluación basada en datos (esperar 35.4,
leer el reporte, citar números en el ADR)
Entonces el test verde
```

## Notas

- Este NO es un issue de implementación. Es un ADR.
- La decisión depende de datos reales (issue 35.4 provee los
  números).
- El output es un archivo markdown + (opcionalmente) una
  migration si la opción A gana.
- Sigue la convention de ADRs del repo (ver
  `.claude/rules/sdd.md` para el formato).
