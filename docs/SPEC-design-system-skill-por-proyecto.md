# SPEC — Skill de "design system" por proyecto

**Estado:** propuesta (spec, sin implementar)
**Fecha:** 2026-07-09
**Origen:** pedido del usuario en sesión — "cada proyecto debería tener su propio design system + convenciones utilizadas por proyecto; eso podría ser una skill, pero se arma por proyecto"

## Problema

Hoy domain tiene:
- **Skills globales** (catálogo: commit-message, wcag-audit, etc.) → aplican a toda la org.
- **Project-policies** → convenciones de arquitectura/estilo scoped a un proyecto (ej. `hook-sessionstart-es-user-message`).
- **Project-skills** → skills asignadas a un proyecto vía `domain_project_skill_register` (ej. `vps-deploy-admin`).

Lo que NO existe: un artefacto único por proyecto que capture su **design system** (identidad visual: paleta, tipografía, spacing, tokens, componentes) Y sus **convenciones de uso** (patrones propios, do/don't), consultable por el agente al construir o revisar UI de ESE proyecto.

Un `wcag-audit` global sabe de accesibilidad genérica, pero no sabe que "en quiensabe.cl los botones primarios son verde #2E7D32, radio 8px, y nunca se usa sombra dura". Eso es conocimiento de proyecto.

## Propuesta

Una **skill `design-system` que se instancia por proyecto** (project-scoped), no una skill global única. Cada proyecto con frontend arma la suya con su contenido propio.

### Forma
- **Tipo:** project-skill (scope=project), registrada vía `domain_project_skill_register`, igual que `vps-deploy-admin`.
- **Slug:** `design-system` (mismo slug, distinto `project_id` → cada proyecto tiene la suya; el catálogo global NO la trae).
- **skill_type:** `prompt` (contenido que el agente lee como contexto al construir/revisar UI).

### Contenido (estructura sugerida del body)
```
<identidad>
  nombre del proyecto, tono visual (ej: sobrio/corporativo, lúdico, minimal)
</identidad>
<tokens>
  colores (primary, secondary, semantic: success/warning/error, neutrales)
  tipografía (familias, escala, pesos)
  spacing (escala base, ej. 4/8/16/24)
  radios, sombras, breakpoints
</tokens>
<componentes>
  patrones de botones, inputs, cards, navegación — con sus variantes y estados
</componentes>
<convenciones>
  do / don't propios del proyecto
  reglas de composición (ej. "nunca sombra dura", "íconos siempre 24px")
</convenciones>
<referencias>
  links a Figma, tokens.json, storybook, o el archivo del repo que sea fuente de verdad
</referencias>
```

### Ciclo de vida
1. **Armado:** al detectar que un proyecto tiene frontend (package.json con react/vue/astro/tailwind, o directorios de UI), el agente PROPONE armar la `design-system` skill del proyecto, extrayendo tokens de la config existente (tailwind.config, tokens.json, CSS variables) + preguntando lo que no pueda inferir.
2. **Confirmación humana síncrona** (como toda skill/policy nueva, per domain.md): mostrar contenido → confirmar/modificar/descartar → `domain_project_skill_register`.
3. **Consulta:** el agente la lee al construir o revisar UI de ese proyecto (junto con `wcag-audit` global). Es la fuente de verdad de identidad visual del proyecto.
4. **Drift:** si cambia la config de tokens del repo (tailwind.config, tokens.json) en un `head.changed`, señalar que la skill puede estar desactualizada y proponer re-sincronizar.

## Decisiones abiertas (a resolver antes de implementar)
- ¿`design-system` es UN solo doc por proyecto o se parte (tokens / componentes / convenciones como skills separadas)? Recomendación: un solo doc para empezar (YAGNI), partir si crece.
- ¿La fuente de verdad es la skill en BD o el repo (tokens.json)? Recomendación: el repo es la fuente; la skill es un primer-plano consultable + convenciones que NO viven en el código. Sincronizar del repo hacia la skill, no al revés.
- ¿Se genera automáticamente en el onboarding de proyecto (fase F de detección de stacks) o solo a pedido? Recomendación: proponer en onboarding si detecta frontend, armar solo con OK humano.

## No-objetivos
- NO es un design system compartido entre proyectos (eso sería una skill global).
- NO reemplaza a `wcag-audit` (accesibilidad genérica) — lo complementa con identidad propia.
- NO genera código de componentes; documenta el sistema para que el agente lo respete.

## Relación con lo existente
- Reusa el mecanismo de project-skills ya probado (`vps-deploy-admin`).
- Reusa el flujo de confirmación humana síncrona de skills/policies (domain.md).
- Complementa `wcag-audit` (global, accesibilidad) con identidad visual (por proyecto).
