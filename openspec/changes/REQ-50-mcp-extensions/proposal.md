# REQ-50 — Extensiones del MCP (4 features)

> 4 HU incrementales que llevan al MCP al siguiente nivel:
> goals/met sistema, auto-mejora de skills, adjuntos MinIO, y policies
> con scope (globales + proyecto + skill).

## Motivacion

Hoy el MCP sabe ejecutar skills, clasificar issues, y persistir memories,
pero le falta:
1. **Vision de largo plazo** (no tiene goals / objetivos)
2. **Auto-mejora** (los skills se degradan sin supervision)
3. **Adjuntos** (los usuarios no pueden subir PDFs/imagenes via API)
4. **Policies granulares** (faltan policies a nivel skill)

## Decisiones de diseno

### 1. Goals primero porque es transversal
Los goals son la abstraccion de nivel mas alto. Vinculan requirements
(alcance), observations (conocimiento), metrics (progreso) e intake
(feedback externo). Sin goals, el sistema solo reacciona; con goals,
puede priorizar trabajo.

### 2. Auto-mejora con human-in-the-loop
Ninguna sugerencia se aplica automaticamente. El LLM sugiere, el humano
aprueba via UI. Esto preserva el control del operador y permite iterar
sin miedo.

### 3. Adjuntos via API, no como feature hardcoded
Los adjuntos son una capacidad transversal (cualquier entity puede tenerlos).
Por eso la tabla tiene entity_type + entity_id en vez de FK a una tabla
especifica. Esto permite adjuntar files a projects, requirements, issues,
goals, messages, etc.

### 4. Policies con scope = defense in depth
El sistema de policies actual (platform + project) es bueno pero le falta
el nivel SKILL. Con skill_policies podemos decir "este skill especifico
solo permite SELECT, no DELETE" sin afectar otros skills.

## Arquitectura

```
+--------------------------------+
|       domain-admin (Django)    |  <-- HU-50.1, 50.2, 50.3, 50.4 UI
+--------------------------------+
            |
            v
+--------------------------------+
|        domain-mcp (Go)          |  <-- HU-50.1, 50.2, 50.3, 50.4 API
+--------------------------------+
   |              |          |
   v              v          v
+------+   +-----------+   +------+
|Goals |   |Skills Auto|   |Files |
|  DB  |   |   Improve |   |  DB  |  <-- HU-50.1, 50.2, 50.3 DB
+------+   +-----------+   +------+
            |             |
            v             v
        +-------+    +-------+
        | LLM   |    | MinIO |  <-- HU-50.3 storage
        +-------+    +-------+
```

## Scope de cada HU

| HU | Alcance | Estimacion |
|----|---------|------------|
| 50.1 Goals | Sistema central de objetivos con jerarquia padre-hijo, criterios de exito, metricas, evaluacion automatica | 3-4 dias |
| 50.2 Auto-mejora skills | LLM-as-judge + 4 tipos de sugerencias (split/merge/refine/archive) + human approval flow | 3-4 dias |
| 50.3 Adjuntos MinIO | Endpoints HTTP para upload/download + Django UI + MinIO storage + thumbnails | 1-2 dias |
| 50.4 Policies scope | skill_policies nueva tabla + resolver unificado + inyeccion en system prompt | 2-3 dias |

**Total: 9-13 dias** distribuidos incrementalmente.

## Orden de implementacion recomendado

1. **HU-50.3 Adjuntos** (1-2 dias, mas rapido, valor inmediato)
2. **HU-50.4 Policies scope** (2-3 dias, mejora seguridad)
3. **HU-50.1 Goals** (3-4 dias, la pieza central)
4. **HU-50.2 Auto-mejora** (3-4 dias, requiere HU-50.1 deployado)

## Riesgos

| Riesgo | Mitigacion |
|--------|------------|
| Goals: LLM hallucina criterios de exito | Goals se crean DRAFT por default, requieren aprobacion explicita |
| Auto-mejora: LLM sugiere splits malos | Toda sugerencia REQUIERE aprobacion humana, nunca auto-apply |
| Adjuntos: virus en PDFs | Hook pre-download con ClamAV (opcional) |
| Policies: combinacion inesperada causa LLM overload | max_size 50KB + truncado + log de policies efectivas |
| Storage MinIO: costos si se acumulan adjuntos | Policy de retencion: soft-delete > 30 dias, hard-delete > 90 dias |

## Out of scope

- **No** se migran los platform_policies existentes al nuevo sistema
  (se mantienen como estan, el resolver los lee de su tabla)
- **No** se cambia la UI existente de platform_policies/project_policies
  (se agrega una vista unificada al lado, no se reemplaza)
- **No** se implementa auto-aplicacion de sugerencias (siempre human-in-loop)
- **No** se agrega OCR/lectura automatica de PDFs al chat
  (HU-50.3 solo guarda el archivo, el chat lo muestra como link)

## Metricas de exito

| HU | Metrica | Target |
|----|---------|--------|
| 50.1 Goals | Goals activos por org | > 5 (en uso) |
| 50.2 Auto-mejora | Sugerencias aprobadas vs rechazadas | > 50% aprobadas |
| 50.3 Adjuntos | Adjuntos subidos por dia | > 10 |
| 50.4 Policies | Policies inyectadas en system prompt | 100% de los agentes |