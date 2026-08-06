# Spec: marcos normativos por proyecto con crosswalk de controles

**Issue:** `issue-56.4-marcos-normativos-por-proyecto-con-crosswalk`
**REQ padre:** `REQ-56-compliance-marcos-normativos`
**Estado:** proposed

## Contexto

Habilita integrar la skill [`compliance-cl`](https://github.com/Lelemon-studio/compliance-cl) —que audita un repo contra packs de ley y genera la documentación de cumplimiento— sin que se auto-aplique a proyectos que no tienen relación con la jurisdicción.

Dos hallazgos medidos el 2026-08-06 motivan el diseño:

1. **El modelo de skills es opt-OUT, no opt-in.** `SkillApplicableIDs`
   (`internal/service/skill/sql/query.sql:149`) resuelve
   `WHERE (s.project_id IS NULL OR s.project_id = :project_id) AND NOT EXISTS (… ps.is_enabled = FALSE)`:
   toda skill global auto-aplica a todos los proyectos y `project_skills` solo sirve para excluir.
   No hay forma de expresar "esta capacidad aplica solo si el proyecto la declara".
2. **Sin crosswalk, un control se audita una vez por marco.** "Cifrado de datos en reposo" lo
   exigen a la vez la Ley 21.719 (deber de seguridad), el GDPR (Art. 32), ISO 27001 (Anexo A) y
   SOC 2. Multiplicado por los ~90 controles del Anexo A, la duplicación deja de ser estilística.

## Requisitos

### REQ-1 — El sistema MUST tratar la ausencia de declaración como "no aplica"

Un proyecto sin filas en `project_compliance_frameworks` NO está afecto a ningún marco. El default
es la exclusión, al revés que el modelo de skills.

#### Scenario: Proyecto sin marcos declarados no recibe ninguna obligación
- **Given** un proyecto sin filas en `project_compliance_frameworks`
- **And** el catálogo `compliance_frameworks` tiene cargadas `ley-21719`, `ley-21595` y `gdpr`
- **When** se consultan los marcos aplicables a ese proyecto
- **Then** el resultado es vacío
- **And** no se evalúa ningún control para ese proyecto

#### Scenario: Un proyecto declara un marco y solo recibe ese
- **Given** un proyecto con una fila activa para `ley-21719`
- **When** se consultan sus marcos aplicables
- **Then** el resultado contiene exactamente `ley-21719`
- **And** no contiene `ley-21595` ni `gdpr`

### REQ-2 — El sistema MUST propagar el estado de un control a todos los marcos que lo exigen

#### Scenario: Un control satisface varios marcos a la vez
- **Given** el control `cifrado-en-reposo` vinculado en `framework_controls` a `ley-21719`, `gdpr` e `iso-27001`
- **And** un proyecto afecto a `ley-21719` y `gdpr`
- **When** se registra `project_control_status` de `cifrado-en-reposo` en estado `ok`
- **Then** el reporte de `ley-21719` muestra ese control como cumplido
- **And** el reporte de `gdpr` también lo muestra como cumplido
- **And** el control se evaluó UNA sola vez

#### Scenario: El crosswalk conserva la referencia propia de cada marco
- **Given** el control `cifrado-en-reposo` vinculado a `gdpr` con referencia `Art. 32`
- **And** el mismo control vinculado a `iso-27001` con su referencia de Anexo A
- **When** se reporta el cumplimiento por marco
- **Then** el reporte de `gdpr` cita `Art. 32`
- **And** el reporte de `iso-27001` cita su propia cláusula

### REQ-3 — El sistema MUST impedir la ingesta del texto de marcos no redistribuibles

Las leyes son texto público; las normas ISO/IEC son de pago y su redistribución está prohibida.
`fuente_tipo` es un guard, no metadata descriptiva.

#### Scenario: Una ley pública se puede ingestar completa
- **Given** el marco `ley-21719` con `fuente_tipo = 'texto_libre'`
- **When** se ingesta su texto oficial a `knowledge_docs`
- **Then** la operación se completa
- **And** el texto queda citable por artículo

#### Scenario: Una norma de pago se rechaza al intentar ingestar su texto
- **Given** el marco `iso-27001` con `fuente_tipo = 'solo_referencia'`
- **When** se intenta ingestar el texto de la norma
- **Then** la operación se rechaza con un error que nombra la restricción de licencia
- **And** sí se admite registrar número de cláusula, interpretación propia y evidencia

### REQ-4 — El catálogo MUST ser legible sin scope de proyecto y las tablas de proyecto MUST estar bajo RLS

Si el catálogo queda bajo RLS por error, las consultas devuelven cero filas **sin error** y el
sistema parece no tener marcos cargados — el mismo modo de falla medido en DOMAINSERV-240
(webhooks) y en la 000287 (knowledge_chunks).

#### Scenario: El catálogo se lee sin GUC de proyecto
- **Given** una sesión sin `app.current_project_id` seteado
- **When** se consulta `compliance_frameworks`
- **Then** devuelve todos los marcos del catálogo

#### Scenario: Las asignaciones de otro proyecto no son visibles
- **Given** dos proyectos A y B, cada uno con marcos declarados
- **And** una sesión con `app.current_project_id` = A
- **When** se consulta `project_compliance_frameworks`
- **Then** solo se ven las filas del proyecto A
- **And** las del proyecto B no aparecen

#### Scenario: Sabotaje — escribir el estado de un control sin scope
- **Given** una sesión sin `app.current_project_id` seteado
- **When** se intenta insertar una fila en `project_control_status`
- **Then** la escritura es rechazada por la policy de RLS
- **And** no queda una fila huérfana de proyecto

### REQ-5 — El sistema MUST distinguir un marco vigente de uno que aún no rige

La Ley 21.719 rige desde diciembre de 2026: "te aplica" y "te va a aplicar" no son lo mismo.

#### Scenario: Un marco declarado pero aún no vigente se reporta aparte
- **Given** el marco `ley-21719` con `vigente_desde` posterior a la fecha actual
- **And** un proyecto afecto a ese marco
- **When** se consulta la postura de cumplimiento
- **Then** el marco aparece marcado como "aún no vigente" con su fecha
- **And** no cuenta como incumplimiento en el score

### REQ-6 — El modelo MUST distinguir ley de norma técnica

#### Scenario: Una norma no territorial no exige jurisdicción
- **Given** el marco `iso-27001` con `tipo = 'norma_tecnica'`
- **When** se persiste con `jurisdiccion` en NULL
- **Then** la fila se acepta
- **And** se reporta como no obligatorio y certificable

#### Scenario: Dos ediciones de la misma norma conviven
- **Given** `iso-27001` edición `2013` y edición `2022` en el catálogo
- **When** un proyecto declara la edición `2022`
- **Then** sus controles se resuelven contra las cláusulas de esa edición
- **And** las de la edición `2013` no se mezclan

## Fuera de alcance

- Poblar el catálogo con ISO 27001 o SOC 2: el esquema los soporta desde el día uno, pero la carga
  inicial es solo `ley-21719`, `ley-21595` y `gdpr`.
- Reemplazar la skill `compliance-cl`. Sigue siendo el motor que corre sus 6 fases; estas tablas solo
  le dicen a qué marcos está afecto el proyecto.
- Emitir certificaciones. `certificable = true` describe que la norma admite auditor externo
  acreditado; domain no lo sustituye.
