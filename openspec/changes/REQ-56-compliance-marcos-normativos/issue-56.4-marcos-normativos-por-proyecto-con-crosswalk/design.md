# Design: marcos normativos por proyecto con crosswalk

## Decisión

Cinco tablas divididas en **dos mitades con reglas de acceso distintas**: un catálogo global de
marcos y controles, y dos tablas por proyecto bajo RLS.

```sql
-- CATÁLOGO — global a la instancia, SIN RLS
compliance_frameworks
  id, slug              'ley-21719' | 'gdpr' | 'iso-27001'
  nombre
  tipo                  ley | reglamento | norma_tecnica | estandar_industria
  jurisdiccion          'CL' | 'EU' | NULL      -- NULL: las normas no son territoriales
  obligatorio           BOOLEAN                 -- ley sí, norma no (salvo contrato)
  certificable          BOOLEAN                 -- ISO sí (auditor externo), ley no
  edicion               TEXT                    -- ISO 27001:2022 ≠ :2013
  vigente_desde         DATE                    -- la 21.719 rige 2026-12-01
  fuente_tipo           texto_libre | solo_referencia
  UNIQUE (slug, edicion)

compliance_controls
  id, slug              'cifrado-en-reposo'
  nombre, descripcion, como_se_verifica

compliance_framework_controls      -- EL CROSSWALK
  framework_id, control_id
  referencia            'Art. 32' | 'A.8.24'
  UNIQUE (framework_id, control_id)

-- POR PROYECTO — CON RLS por app.current_project_id
project_compliance_frameworks     -- el opt-in: SIN FILA = NO APLICA
  project_id, framework_id, activo, activado_por_id, activado_at
  UNIQUE (project_id, framework_id)

project_control_status
  project_id, control_id
  estado                ok | parcial | falta | no_verificable
  evidencia, evaluado_at, evaluado_por_id
  UNIQUE (project_id, control_id)
```

## Alternativas evaluadas

### A. `projects.settings` (jsonb) — descartada
`projects` ya tiene `settings jsonb`, así que `{"compliance":["ley-21719"]}` no requiere migración.
Se descartó porque sin catálogo nadie valida los slugs —un typo crea un marco fantasma en
silencio— y responder "qué proyectos están afectos a la 21.719" obliga a escanear el JSON de todos.
Sirve para prototipar, no para sostener el crosswalk.

### B. Catálogo + puente, sin controles — descartada al ampliar a ISO
Dos tablas (`compliance_frameworks` + `project_compliance_frameworks`) resuelven el opt-in y lo
internacional. Alcanzaba mientras el alcance eran solo leyes chilenas. Al entrar ISO 27001 dejó de
alcanzar: sin `compliance_framework_controls`, cada uno de los ~90 controles del Anexo A se escribe y audita
por separado en cada marco que lo exige.

### C. Invertir el modelo de skills (`requires_optin` en `skills`) — descartada como solución principal
Agregar una columna a `skills` para que una skill marcada no auto-aplique resuelve el opt-in con una
migración chica y sirve para cualquier skill futura. Se descartó **como sustituto** porque una skill
no modela marcos, ediciones, vigencia ni crosswalk: forzarlo dejaría el compliance dentro de un
campo de texto. Sigue siendo deseable por separado, y este issue no lo bloquea.

## Data flow

```
domain_session_bootstrap
  └─ resuelve marcos del proyecto (project_compliance_frameworks)
       └─ vacío → no se inyecta nada de compliance   ← el caso por defecto

skill compliance-cl (motor, 6 fases)
  Fase 0 encuadre    ← LEE los marcos declarados en vez de preguntarlos
  Fase 1 descubrir   → grep del repo
  Fase 2 evaluar     → escribe project_control_status (UNA vez por control)
  Fase 3 generar     → docs por cada marco activo
  Fase 4 estado      → el crosswalk expande el estado a cada marco
  Fase 5 construir   → brechas como domain_ticket_create

reporte por marco = project_control_status ⋈ compliance_framework_controls ⋈ compliance_frameworks
```

El punto de enganche es el **bootstrap de sesión**: es donde hoy se resuelve qué aplica al proyecto.

## Riesgos

1. **Que el catálogo termine bajo RLS.** Devolvería cero filas sin error y parecería que no hay
   marcos cargados. Es el modo de falla medido en DOMAINSERV-240 y en la 000287. Mitigación: el
   escenario de sabotaje de REQ-4 lo cubre explícitamente.
2. **Copyright de las normas ISO/IEC.** El texto es de pago y no se puede redistribuir. `fuente_tipo`
   es el guard; sin él alguien va a ingestar la ISO como se ingestó la 21.719. Mitigación: REQ-3 lo
   verifica en los dos sentidos.
3. **Numeración de cláusulas atada a la edición.** El Anexo A se reorganizó entre ISO 27001:2013 y
   :2022, así que una referencia sin edición es ambigua. Mitigación: `UNIQUE (slug, edicion)` y el
   escenario de convivencia de ediciones.
4. **Falsa sensación de cumplimiento.** Un `estado = ok` autoevaluado no es una certificación.
   Mitigación: `certificable` explícito y el disclaimer que la propia skill ya exige.

## TDD plan

- **Red:** tests de RLS primero — catálogo legible sin GUC, tablas de proyecto invisibles cross-project.
  Con las tablas creadas y sin policies, el test de aislamiento debe fallar.
- **Green:** migración `000291` con las 5 tablas + policies de RLS solo en las dos de proyecto.
- **Refactor:** extraer la resolución de marcos aplicables a una query única, para que no se duplique
  entre el bootstrap y el reporte.
- **Sabotaje:** (a) quitar la policy de `project_control_status` → el test cross-project debe ponerse
  en rojo; (b) poner el catálogo bajo RLS → el test de lectura sin GUC debe fallar con cero filas;
  (c) marcar `iso-27001` como `texto_libre` → el guard de ingesta debe dejar de rechazar.

Los tests de RLS van con `//go:build integration` y testcontainers: la suite unitaria pasa entera con
el RLS mal puesto, como quedó demostrado hoy con webhooks.
