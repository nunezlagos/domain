# Proposal: marcos normativos por proyecto con crosswalk

**REQ padre:** REQ-56-compliance-marcos-normativos
**Esfuerzo estimado:** M (1-3 días)
**Prioridad:** media

## Intention

Que cada proyecto declare explícitamente a qué marcos normativos está afecto —leyes chilenas, GDPR,
ISO 27001— y que un control implementado una vez se reporte como cumplido en todos los marcos que lo
exigen, sin que ningún marco se auto-aplique a proyectos que no lo declararon.

## Scope

**Entra:**
- Migración `000291`: 5 tablas (3 de catálogo global, 2 por proyecto con RLS).
- Tools MCP `domain_compliance_*` para listar el catálogo, declarar marcos por proyecto y registrar
  el estado de un control.
- Carga inicial del catálogo: `ley-21719`, `ley-21595`, `gdpr` + los controles compartidos.
- Tests de integración de RLS con testcontainers, incluidos los tres sabotajes del design.

**No entra:**
- Poblar ISO 27001 / SOC 2. El esquema los soporta desde el día uno; la carga se hace cuando un
  proyecto los necesite, sin migración nueva.
- Modificar el modelo de skills. Invertir `SkillApplicableIDs` a opt-in es deseable y va por
  separado; este issue no depende de eso ni lo bloquea.
- La ingesta de los textos legales a `knowledge`. Es el paso siguiente y ya es viable: el fix de
  DOMAINSERV-227 (desplegado 2026-08-06) subió el techo de ingesta — 206 KB en 1,74 s, y el texto de
  la 21.719 pesa 176 KB, que antes reventaba por timeout.

## Approach

1. Migración con las 5 tablas. RLS **solo** en `project_compliance_frameworks` y
   `project_control_status`, por `app.current_project_id`, siguiendo el patrón de la 000287/000288.
   El catálogo queda sin RLS a propósito.
2. Service `internal/service/compliance` con la resolución de marcos aplicables y la expansión del
   crosswalk, contra interfaces chicas definidas en el consumidor.
3. Tools MCP bajo `rlsProyecto`, como quedaron las de webhooks en DOMAINSERV-240.
4. Seeder del catálogo inicial siguiendo `data-migration-methodology` (seeder + bump, no SQL suelto).

## Risks

- **El catálogo bajo RLS por error** → cero filas sin error, indistinguible de "no hay marcos".
  Cubierto por el escenario de sabotaje de REQ-4.
- **Copyright ISO/IEC** → el texto de la norma no se puede redistribuir. `fuente_tipo` es el guard,
  verificado en los dos sentidos por REQ-3.
- **Ambigüedad de cláusulas entre ediciones** → `UNIQUE (slug, edicion)` y escenario propio.
- **Interpretar un `ok` autoevaluado como certificación** → `certificable` explícito y el disclaimer
  que la skill ya obliga a incluir al pie de cada documento generado.
