# SDD Workflow — Domain

## Flujo obligatorio para cualquier cambio

1. Buscá la HU correspondiente en `openspec/changes/REQ-*/issue-*/`
2. Leé en orden: `issue.md` (Gherkin) → `design.md` (ADR + decisión) → `tasks.md` (bullets atómicos)
3. Verificá la **Persona** que declara la HU — debés entender perspectiva del actor
4. Seguí TDD estricto: **test → implementación mínima → refactor → sabotaje**
5. Si encontrás un gap en los requirements: **PREGUNTÁ ANTES DE CODEAR** (regla CLAUDE.md global)

## Header obligatorio de toda HU nueva

```markdown
# issue-XX.Y-slug-name

**Origen:** `REQ-XX-req-slug`
**Prioridad tentativa:** alta | media | baja
**Tipo:** feature | infrastructure | hardening | tooling | docs | runbook

## Historia de usuario

**Como** <rol natural-language>
**Quiero** <capacidad>
**Para** <beneficio>

## Criterios de aceptación

### Escenario 1: ...
```gherkin
Dado que ...
Cuando ...
Entonces ...
```
...

## Análisis breve

- Qué pide realmente:
- Módulos sospechados:
- Riesgos / dependencias:
- Esfuerzo tentativo: S | M | L
```

> NOTA: el concepto previo de `**Persona:**` field (catálogo de 10 user-types)
> fue deprecado. Ver issue-01.9 archivada. El equivalente correcto vive en
> `agent_personalities` (issue-08.5 agent-templates) para describir cómo se
> comporta el AGENTE IA — no a quién va dirigida la HU.

## TDD strict workflow

### Step 1: Test primero

- Escribir test que cubra Gherkin scenario
- Confirmar que falla (red) por la razón correcta — no por syntax error

### Step 2: Implementación mínima

- Mínimo código necesario para hacer pasar el test
- NO over-engineering, NO features extra no pedidas

### Step 3: Refactor

- Limpiar duplicación
- Mejorar naming
- Aplicar conventions de `.claude/rules/*.md`
- Tests siguen verdes

### Step 4: Sabotaje

- Cada HU declara al menos UN test "sabotaje"
- Romper intencionalmente la invariante que el test valida
- Confirmar que el test atrapa la regresión
- Restaurar — esto garantiza que el test no es "always green"

Ejemplo: issue-25.5 RLS — sabotaje "query sin SET LOCAL → 0 rows".

## Reglas duras

- **NUNCA** implementar sin HU aprobada
- **NUNCA** modificar `issue.md` / `design.md` sin orden explícita del humano
- **NUNCA** commitear sin tests verdes locales
- **NUNCA** skipear linters con `--no-verify`
- **PREGUNTÁ** si hay ambigüedad en spec antes de codear
- **VERIFICÁ** antes de afirmar (regla CLAUDE.md): leé el código, no asumás

## Workflow agente IA (proyecto 100% AI-generated)

Ver `.claude/rules/ai-generation.md` para detalle del workflow multi-agent estándar (planning → coder → reviewer → tester → humano).

## Estados de una HU

- `proposed` — spec aprobada, no implementada
- `in_progress` — algún PR abierto la implementa
- `implemented` — código mergeado, tests verdes, criterios cumplidos
- `archived` — sustituida o cancelada (mover a `openspec/changes/archive/`)

Tracked en `state.yaml` per HU + per REQ.

## Workflow PR

1. Branch `feat/hu-XX-Y-short-name` desde main
2. Implementar siguiendo `tasks.md`
3. Commits siguen Conventional Commits (`.claude/rules/git.md`)
4. PR usa template `.github/PULL_REQUEST_TEMPLATE.md`
5. CI verde: lint + tests + linters custom (issue-25.13, issue-13.9)
6. Review humano
7. Merge squash; mensaje squash respeta Conventional + referencia HU
8. Actualizar `state.yaml` de la HU a `implemented`
9. CHANGELOG Unreleased actualizada
