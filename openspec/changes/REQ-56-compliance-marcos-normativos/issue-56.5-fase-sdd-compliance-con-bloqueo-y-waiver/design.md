# Design: fase `sdd-compliance`

## ADR-1 — Fase propia, no una quinta lente ni una augmentación de R1

**Decisión:** una fase `sdd-compliance` con handler y contrato propios, con autoridad para detener el
flow.

**Alternativas evaluadas:**

- **Augmentar `r1_shift_left` con checks de compliance** (patrón de la policy
  `security-review-domain-specific`, que ya hace exactamente eso). Es la opción más barata: una
  policy, cero código. **Rechazada** por dos razones textuales del propio pipeline: `sdd-4r` declara
  *"esta fase no bloquea"*, y R1 excluye por regla dura todo lo *pre-existing* — que es la
  naturaleza de casi toda obligación de compliance. Entraría, y quedaría muda.
- **Lente R5 nueva dentro de 4R.** Hereda el "no bloquea" de la fase entera y además rompe la
  simetría del nombre (4R son cuatro por definición).
- **Gate en `sdd-spec` + policy en R1, sin fase** (las "tres capas"). Cubre bien los tres momentos,
  pero ninguna de las dos piezas puede detener el flow con autoridad propia, que es el requisito.

**Tradeoff aceptado:** una fase más en todos los flows `full`. Se mitiga con el no-op de REQ-1: sin
marcos declarados la fase cierra sin consultar nada ni gastar un turno de agente.

## ADR-2 — Entre `sdd-design` y `sdd-tasks`

**Decisión:** `"sdd-compliance": {"sdd-design"}` y `sdd-tasks` pasa a depender de ella.

**Por qué ahí y no en otro lado:**

| Punto | Problema |
|---|---|
| Antes de `sdd-design` | no hay sustrato: todavía no se sabe qué datos toca el cambio |
| Después de `sdd-apply` | el código ya está escrito; un BLOCKER ahí es rework caro y empuja al waiver por fatiga |
| Después de `sdd-verify` | peor todavía: además de código hay tests escritos contra el diseño incumplido |
| **Entre `design` y `tasks`** | **el design declara qué datos toca y qué controles piensa implementar, y no hay ni una línea escrita** |

## ADR-3 — La severidad se deriva del catálogo, no de una tabla propia

`obligatorio` + `vigente_desde` de `compliance_frameworks` (issue-56.4) determinan el veredicto:

```
obligatorio && vigente        → BLOCKER
obligatorio && !vigente aún   → WARNING  (la 21.719 hasta dic-2026)
!obligatorio                  → SUGGESTION  (ISO, salvo contrato)
```

**Alternativa rechazada:** una tabla de severidades por obligación. Duplica la fuente de verdad y
permite que una ley obligatoria quede configurada como sugerencia — un incumplimiento silencioso por
configuración.

## ADR-4 — Waiver en BD, no en el filesystem

**Decisión:** el waiver se persiste con razón obligatoria, actor, timestamp y obligación afectada, y
aparece en el reporte del flow.

**Por qué no el patrón del commit-gate** (`echo razón > ~/.local/state/domain/gate-bypass-<id>`):
sirve para un gate local de un solo uso, pero un waiver de compliance tiene que ser **auditable por
otro** —es su razón de ser— y un archivo en el home del que lo otorgó no lo es.

**Por qué existe el waiver:** un gate sin válvula de escape se vuelve insatisfacible y empuja al
bypass permanente. Está documentado tres veces en este repo (DOMAINSERV-111, 175, 195). El waiver
con fricción —razón escrita obligatoria— es más seguro que un BLOCKER duro que alguien termina
desactivando entero.

## ADR-5 — Handoff con R1 en vez de duplicación

`sdd-compliance` responde *"¿qué exige el marco?"* y deja la lista de controles exigidos en su
output. R1 de `sdd-4r` responde *"¿este diff lo viola?"*, que es lo que ya sabe hacer y para lo que
su regla de scoping por `changed-hunk` es la correcta.

`sdd-4r` ya recibe `PriorOutputs`, así que el canal existe: no hay que tocar R1 ni bumpear su
template.

## Data flow

```
sdd-design  ──> declara qué datos toca y qué controles piensa implementar
     │
     ▼
sdd-compliance
     ├─ ¿el proyecto declaró marcos?  NO → verdict: not_applicable, no-op
     ├─ SÍ → obligaciones = marcos declarados ⋈ compliance_framework_controls
     ├─ contrasta contra lo que el design declara
     ├─ severidad = f(obligatorio, vigente_desde)
     └─ ¿hay BLOCKER sin waiver?
            SÍ → el flow se detiene acá
            NO → output: controles_exigidos[] ──┐
     │                                          │
     ▼                                          │
sdd-tasks → sdd-apply → sdd-verify → sdd-judge  │
                                                ▼
                                          sdd-4r (R1 verifica el diff
                                          contra controles_exigidos)
```

## Riesgos

1. **Fatiga de waiver.** Si la fase produce demasiados BLOCKER discutibles, el waiver se vuelve
   trámite y el gate deja de significar algo. Mitigación: la severidad sale del catálogo, no de una
   heurística, y solo una ley **obligatoria y vigente** bloquea.
2. **El hueco de los modos reducidos.** `lite`/`express`/`micro` no tienen `sdd-design`, así que la
   fase no corre. Mitigación: REQ-5 lo declara explícito — sugerencia de subir a `full` y constancia
   en el reporte de que la fase no corrió. Un hueco declarado es deuda; uno silencioso es un bug.
3. **Falsa sensación de cumplimiento.** Pasar la fase no es cumplir la ley: es no violar las
   obligaciones que el catálogo modela. Mitigación: el reporte lo dice con todas las letras.
4. **Seed version.** Tocar el template del agente sin bumpear `agentTemplatesSeedVersion` deja el
   cambio fuera de la BD con un síntoma indistinguible del éxito.

## TDD plan

- **Red:** el no-op primero (REQ-1) — con la fase creada y sin la guarda, un proyecto sin marcos
  debería consultar obligaciones y el test lo detecta.
- **Green:** handler + inserción en el DAG + derivación de severidad.
- **Refactor:** la derivación de severidad a una función pura, testeable sin BD.
- **Sabotaje:**
  1. invertir la condición del no-op → el test de REQ-1 se pone en rojo;
  2. cambiar `obligatorio && vigente` por `obligatorio` → una ley aún no vigente bloquearía, y el
     escenario de la 21.719 lo atrapa;
  3. aceptar un waiver con razón vacía → el escenario de REQ-3 falla;
  4. quitar `sdd-compliance` de `phaseDependencies` dejando `sdd-tasks` colgada → `ValidateDAG` debe
     rechazar el catálogo.
