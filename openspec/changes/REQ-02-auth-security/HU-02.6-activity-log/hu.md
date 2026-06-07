# HU-02.6-activity-log

**Origen:** `REQ-02-auth-security`
**Prioridad tentativa:** alta
**Tipo:** feature

## Historia de usuario

**Como** desarrollador usando Domain
**Quiero** un registro cronológico de toda la actividad del sistema (creaciones, modificaciones, consultas) filtrable por proyecto, usuario, entidad y acción
**Para** entender qué pasó, quién lo hizo y cuándo, sin necesidad de revisar logs de aplicación

## Criterios de aceptación

### Escenario 1: Registrar actividad al crear una observación

```gherkin
Dado que el usuario "user-juan" guarda una observación en el proyecto "proj-abc"
Entonces se registra un activity log con:
  | actor_id       | "user-juan"    |
  | project_id     | "proj-abc"     |
  | action         | "observation.create" |
  | entity_type    | "observation"  |
  | entity_id      | <uuid>         |
  | metadata       | {"title": "Fix aplicado", "type": "fix"} |
```

### Escenario 2: Consultar actividad de un proyecto

```gherkin
Dado que hay 50 entries de activity en "proj-abc"
Cuando consulto GET /api/v1/activity?project_id=proj-abc&limit=10
Entonces recibo los últimos 10 eventos ordenados por created_at DESC
Y cada evento incluye: actor_id, action, entity_type, entity_id, metadata, created_at
```

### Escenario 3: Filtrar por usuario y acción

```gherkin
Dado que hay actividad de múltiples usuarios
Cuando consulto GET /api/v1/activity?actor_id=user-juan&action=observation.create
Entonces solo recibo eventos de creación de observaciones hechos por user-juan
```

### Escenario 4: Activity log vs audit log

```gherkin
Dado que un usuario actualiza un skill
Entonces se registra en el activity log: action = "skill.update"
Y se registra en el audit log (HU-02.4): old_values y new_values del cambio
```

### Escenario 5: Cleanup automático

```gherkin
Dado que la política de retención es 90 días
Cuando un activity log tiene más de 90 días
Entonces se elimina automáticamente (cleanup diario)
```

## Análisis breve

- **Qué pide realmente:** Tabla `activity_log` con: id, project_id, actor_id, action, entity_type, entity_id, metadata (JSONB), created_at. Diferente del audit log: activity es operacional (qué pasó), audit es compliance (valores antes/después).
- **Módulos sospechados:** `internal/store/pg/activity.go`, middleware de logging automático
- **Riesgos / dependencias:** Puede crecer rápido. Cleanup automático obligatorio. Metadata JSONB sin schema fijo puede ser difícil de consultar.
- **Esfuerzo tentativo:** M

## Verificación previa

- [ ] Revisar codebase (grep)
- [ ] Revisar git log
- [ ] Probar en ambiente correcto

### Resultado de verificación

- **Estado:** pendiente
- **Evidencia:**
- **Acción derivada:**
