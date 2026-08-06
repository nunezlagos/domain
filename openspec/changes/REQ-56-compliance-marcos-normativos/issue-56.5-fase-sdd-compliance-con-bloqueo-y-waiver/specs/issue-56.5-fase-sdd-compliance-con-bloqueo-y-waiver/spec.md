# Spec: fase `sdd-compliance` con bloqueo y waiver auditado

**Issue:** `issue-56.5-fase-sdd-compliance-con-bloqueo-y-waiver`
**REQ padre:** `REQ-56-compliance-marcos-normativos`
**Depende de:** `issue-56.4-marcos-normativos-por-proyecto-con-crosswalk` (las tablas)
**Estado:** proposed

## Contexto

Hoy no existe dónde poner una obligación legal **con autoridad para bloquear**:

- `sdd-4r` declara en su propio prompt: *"El controller tiene toda la autoridad: mergea y decide,
  **esta fase no bloquea**"*.
- Su template `r1_shift_left` tiene una regla dura de scoping: accionable solo si
  `causal_disposition ∈ {introduced, behavior-activated, worsened}` **y** hay un `changed-hunk:` en
  `proof_refs`; y *"vulnerabilidades pre-existing/base-only: se LISTAN como informativas. NUNCA
  bloquear, NUNCA proponer fix, NUNCA abrir ticket"*.

El compliance es un **estado del sistema**, no una propiedad del diff: "no hay RAT", "no hay
mecanismo ARCO", "no se declaró plazo de retención" son exactamente *pre-existing*. Dentro de R1
quedarían mudas por diseño, y una obligación legal que no bloquea es una sugerencia.

## Ubicación en el DAG

El pipeline es lineal (`modes/validator.go`). La fase se inserta **entre `sdd-design` y
`sdd-tasks`**:

```go
"sdd-compliance": {"sdd-design"},
"sdd-tasks":      {"sdd-compliance"},   // antes: {"sdd-design"}
```

Es el último punto donde corregir es barato: el design ya declara qué datos toca y qué controles
piensa implementar —o sea hay sustrato real que evaluar— y todavía no se generaron tasks ni se
escribió una línea de código. Bloquear después de `sdd-apply` sería la peor experiencia posible, que
es justo lo que motiva no usar 4R.

## Requisitos

### REQ-1 — La fase MUST ser un no-op si el proyecto no declaró marcos

Es lo que hace barato agregarla a todos los flows `full`.

#### Scenario: Proyecto sin marcos declarados
- **Given** un proyecto sin filas activas en `project_compliance_frameworks`
- **When** el flow llega a `sdd-compliance`
- **And** la fase cierra con `verdict: not_applicable` sin consultar obligaciones ni gastar un turno de agente
- **Then** el flow avanza a `sdd-tasks` sin fricción

### REQ-2 — La severidad MUST derivarse del catálogo, no de una tabla aparte

`obligatorio` y `vigente_desde` de `compliance_frameworks` deciden el veredicto.

#### Scenario: Ley obligatoria y vigente incumplida bloquea
- **Given** un proyecto afecto a un marco con `obligatorio = true` y `vigente_desde` ya cumplido
- **And** una obligación de ese marco sin satisfacer según el design
- **When** corre `sdd-compliance`
- **Then** el finding sale como `BLOCKER`
- **And** el flow se detiene sin avanzar a `sdd-tasks`
- **And** el finding cita el marco y su referencia de artículo

#### Scenario: Ley declarada pero aún no vigente no bloquea
- **Given** un proyecto afecto a `ley-21719`, cuyo `vigente_desde` es 2026-12-01
- **And** la fecha actual es anterior a esa
- **When** corre `sdd-compliance` con esa obligación sin satisfacer
- **Then** el finding sale como `WARNING` con la fecha de entrada en vigencia
- **And** el flow avanza

#### Scenario: Norma voluntaria no bloquea
- **Given** un proyecto afecto a un marco con `obligatorio = false` (ISO 27001)
- **When** una de sus obligaciones no se satisface
- **Then** el finding sale como `SUGGESTION`
- **And** el flow avanza

### REQ-3 — Un BLOCKER MUST admitir waiver con razón escrita, auditado en BD

Un gate sin válvula de escape se vuelve insatisfacible y empuja al bypass permanente — este repo ya
documentó ese modo de falla en DOMAINSERV-111, 175 y 195.

#### Scenario: El waiver destraba el flow y queda registrado
- **Given** un flow detenido por un `BLOCKER` de compliance
- **When** se otorga un waiver con razón escrita
- **Then** el flow puede avanzar a `sdd-tasks`
- **And** queda persistida en BD la razón, quién lo otorgó, cuándo y sobre qué obligación
- **And** el waiver aparece en el reporte del flow

#### Scenario: Un waiver sin razón se rechaza
- **Given** un flow detenido por un `BLOCKER` de compliance
- **When** se intenta otorgar un waiver con razón vacía o solo espacios
- **Then** se rechaza y el flow sigue detenido

#### Scenario: El waiver no es un archivo local
- **Given** el mecanismo de waiver de esta fase
- **When** se compara con el bypass del commit-gate (`~/.local/state/domain/gate-bypass-*`)
- **Then** el de compliance vive en la base y es consultable por otro, no en el filesystem del que lo otorgó

### REQ-4 — La fase MUST entregar a R1 la lista de controles exigidos, sin duplicar la verificación

`sdd-compliance` decide **qué se exige**; R1 de `sdd-4r` verifica **que el diff no lo viole**.
`sdd-4r` ya recibe `PriorOutputs`.

#### Scenario: R1 recibe los controles exigidos
- **Given** un flow cuyo `sdd-compliance` cerró con una lista de controles exigidos
- **When** se construye el prompt de `sdd-4r`
- **Then** el `initial_review_tree` incluye esos controles como criterio de R1
- **And** `sdd-compliance` no evalúa el diff por su cuenta

### REQ-5 — El hueco de los modos reducidos MUST quedar declarado, no silencioso

`lite`, `express` y `micro` no tienen `sdd-design`, así que la fase no corre. Un cambio de 10 líneas
puede agregar un campo `email` igual.

#### Scenario: Un cambio sensible en modo reducido sugiere subir a full
- **Given** un proyecto con marcos declarados
- **And** un flow en modo `lite` o `express`
- **When** el cambio toca paths con indicios de datos personales
- **Then** el orquestador **sugiere** subir a `full` nombrando el motivo
- **And** NO bloquea: la sugerencia es informativa

#### Scenario: El salto de la fase queda registrado
- **Given** un flow en modo reducido en un proyecto con marcos declarados
- **When** el flow termina sin haber corrido `sdd-compliance`
- **Then** el reporte del flow lo dice explícitamente
- **And** no se reporta como "compliance OK"

### REQ-6 — La fase MUST respetar el DAG y no romper los flows existentes

#### Scenario: El DAG sigue siendo válido
- **Given** el catálogo `FullPhases` con `sdd-compliance` insertada
- **When** se valida el DAG
- **Then** `sdd-tasks` depende de `sdd-compliance` y `sdd-compliance` de `sdd-design`
- **And** `ValidateDAG` acepta el catálogo

#### Scenario: Saltear la fase explícitamente es válido
- **Given** un flow `full` con `skip_phases: ["sdd-compliance"]`
- **When** se construye el plan
- **Then** el plan se arma sin esa fase
- **And** `sdd-tasks` no queda huérfana de dependencias

## Fuera de alcance

- Generar la documentación de compliance (RAT, política de privacidad, DPA, EIPD). Eso es estado
  documental, no una propiedad de un change: va por un proceso periódico (cron), no por el pipeline.
- Modificar el comportamiento de `sdd-4r` o de `r1_shift_left`. La fase le **entrega** los controles
  exigidos; R1 sigue con su regla de scoping intacta.
- Poblar el catálogo de marcos. Es de `issue-56.4`.

## Nota operativa

Si se toca el template del agente de la fase, hay que **bumpear `agentTemplatesSeedVersion`**
(hoy en 25). Sin el bump el cambio no llega a la BD y, en palabras del propio comentario del
código, *"el síntoma es indistinguible del éxito"*.
