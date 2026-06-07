# REQ-02-session-lifecycle: Gestión del ciclo de vida de sesiones: inicio, fin, resumen, captura pasiva, buffer de contexto de prompt.

**Estado:** activo
**Creado:** 2026-06-07

## Descripción

Gestión del ciclo de vida de sesiones: inicio, fin, resumen, captura pasiva, buffer de contexto de prompt.

## Criterios de éxito

- Sesiones con inicio, fin y status tracking funcionales
- Resúmenes estructurados guardados y recuperables
- Aprendizajes extraídos automáticamente del texto
- Contexto reciente consultable con filtros por scope

## HUs hijas

| HU | Estado | Descripción |
|----|--------|-------------|
| HU-02.1-session-start-end | proposed | Session register: start/end con id, project, timestamps, status active/completed, badge UI, errores |
| HU-02.2-session-summary | proposed | End-of-session summary estructurado: Goal, Discoveries, Accomplished, Next Steps, validación |
| HU-02.3-passive-capture | proposed | Extract learnings: parsear "## Key Learnings:" del texto, dedup, guardar como observaciones |
| HU-02.4-context-retrieval | proposed | Recent context: sesiones, observaciones y prompts con filtros project/scope/personal/global |
