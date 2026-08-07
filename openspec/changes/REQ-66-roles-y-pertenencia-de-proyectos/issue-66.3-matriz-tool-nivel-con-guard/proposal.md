# Proposal: matriz tool→nivel con guard de cobertura

**REQ padre:** REQ-66-roles-y-pertenencia-de-proyectos
**Esfuerzo:** L — el trabajo es mecánico pero son 181 decisiones
**Prioridad:** alta
**Cubre:** REQ-5 del spec

## Intention

Que cada una de las 181 tools declare qué nivel exige, y que sea **imposible** agregar una tool sin
declararlo.

## Scope

**Entra:**
- Mapa explícito tool → nivel requerido, para las 181.
- Guard que rompe CI ante una tool sin nivel.
- Decisión explícita sobre las **~40 tools sin noción de proyecto**.
- Marcado del catálogo global como legible por todos.

**No entra:**
- Aplicar la matriz (66.4). Acá se **declara**, no se enforcea.
- Cambiar la firma de ninguna tool.

## Approach

Calcado de `tool_channels.go` (REQ-54 issue-54.6), que clasifica 166 tools y cuyo guard
`TestAllToolsHaveChannel` rompe CI ante una tool sin canal. Ese precedente prueba que la invariante
"cero tools huérfanas" es sostenible en este repo a esta escala.

**Mapa explícito, no convención de nombres.** Derivar el nivel de un sufijo (`*_delete` → admin)
parece más elegante y es peor: una tool mal nombrada queda mal autorizada **en silencio**, y no hay
forma de notarlo. La lista explícita obliga a decidir una por una, que es exactamente el punto.

### Las ~40 tools sin eje de proyecto

`client`, `sync`, `attachment`, `cron_crud`, `workflow`, `error_reporting`, `mem_usage` y `health`
no reciben `project_slug`, así que no se pueden filtrar por membresía. Cada una va a una de tres
categorías, con su razón escrita:

1. **global-lectura** — cualquiera autenticado. Ej: `health`.
2. **global-admin** — exige rol global `admin` u `owner`. Ej: `cron_crud`, `client`.
3. **necesita eje de proyecto** — hay que agregárselo antes de poder autorizarla. Se declara como
   deuda, no se deja sin clasificar.

Que la categoría 3 exista y esté nombrada es preferible a una clasificación cómoda que finja que
el problema no está.

## Risks

- **181 decisiones tomadas de corrido son 181 oportunidades de poner el nivel de más.** Un nivel
  demasiado alto rompe un flujo legítimo; uno demasiado bajo abre un agujero. El primero se nota, el
  segundo no — ante la duda, el nivel más alto.
- **El catálogo global es el error más fácil de cometer**: si a las policies de plataforma o a las
  skills globales se les exige nivel, un developer deja de ver las reglas que lo gobiernan. Necesita
  su propio test, no solo una clasificación cuidadosa.
- **El guard puede volverse ruido** si falla por tools internas o de test. Debe mirar el registry
  real, igual que `TestAllToolsHaveChannel`.
- **La matriz se desactualiza sola** si alguien renombra una tool. El guard tiene que detectar tanto
  las tools sin nivel como los niveles que apuntan a tools inexistentes — como ya hace el guard de
  cobertura de `ci-shell-guards.yml` en las dos direcciones.
