# Plan: soporte multi-lenguaje en `code_graph`

**Fecha:** 2026-07-03
**Objetivo:** que el indexador de código de domain soporte los lenguajes reales de los
proyectos del usuario (PHP, Vue, HTML, CSS/SCSS, Blade, Astro), y arreglar el bug que
hace que archivos no-Go se marquen como `language: "go"`.

---

## 0. Estado actual (verificado en código)

Parser: `services/domain-mcp/internal/service/codegraph/`
- `parser.go` — `ParsedNode`/`ParsedFile`, parser nativo Go (`go/ast`).
- `treesitter.go` — motor tree-sitter genérico (build tag `treesitter`, CGO).
- `register_treesitter.go` — declara un `langSpec` por lenguaje. **Agregar lenguaje = 1 import + 1 bloque.**
- `service.go` — Build (server-side) y Upload (client-side).
- `install-user/scripts/domain-code-graph.sh` — script cliente (mapa `LANG_EXT`).

**Lenguajes hoy:** Go (nativo) · Python, PHP, JS (.js/.jsx/.mjs/.cjs), TS (.ts/.mts/.cts), TSX (tree-sitter).

**Faltan:** Vue, HTML, CSS/SCSS/LESS, Blade, Astro.

---

## 1. Censo real de los 16 proyectos locales (base del plan)

| Lenguaje | Archivos | Estado | Nota |
|---|---:|---|---|
| PHP | 18.592 | ✅ ya | ~17k son Moodle (terceros) → excluir |
| HTML | 14.997 | ❌ falta | ~14k son reportes generados de cipher-engine → excluir |
| JS | 5.501 | ✅ ya | |
| Vue | 1.357 | ❌ falta | ace-did, ace-dide, saargo-moodle |
| Go | 1.002 | ✅ nativo | |
| CSS/SCSS/LESS | 996 | ❌ falta | |
| TS/TSX | 504 | ✅ ya | |
| Blade (.blade.php) | 340 | ❌ falta | todo en sigec_v2 (Laravel) |
| Python | 229 | ✅ ya | |
| Astro | 39 | ❌ falta | quien-sabe-de-web |

**No hay:** .jsx, .twig, .svelte, .sass, .htm → fuera de alcance.

**Sesgo importante:** los totales están dominados por código de terceros/generado
(Moodle, reportes HTML). El plan prioriza **volumen de código escrito a mano**, no el crudo.

---

## 2. Bug bloqueante: `language: "go"` en archivos no-Go

**Causa raíz (verificada):**
- `parser.go:48` — `ParsedNode` NO tiene campo `Language`.
- Script cliente arma el JSON de nodos SIN `language`.
- `service.go:~221` (Upload) — cae al hardcode `defaultLanguage = "go"` (service.go:58).
- Build NO tiene el bug (deriva language del parser vía `languageOf`).

**Fix (4 puntos):**
1. `parser.go` — agregar `Language string` a `ParsedNode`.
2. `domain-code-graph.sh` — incluir `'language': '$lang'` en el JSON de cada nodo.
3. `service.go` (Upload) — usar `n.Language` en vez de `defaultLanguage`.
4. `code_graph_tools.go` — parsear `language` de los nodos entrantes.

**Este fix es prerequisito.** Sin él, agregar lenguajes no sirve: todo lo subido por
Upload seguiría marcándose `go`.

---

## 3. Clasificación de lenguajes a agregar

### Grupo A — tree-sitter directo (barato: import + langSpec)
Gramáticas oficiales en `go-sitter-forest`. Cada una es un bloque declarativo.
- **CSS / SCSS / LESS** — `.css`, `.scss`, `.less`. Extrae reglas/selectores. Trivial.
- **HTML** — `.html`. Extrae estructura (tags, ids, scripts embebidos). Nodos estructurales.

### Grupo B — formatos híbridos (necesitan tratamiento especial)
No son "una gramática": mezclan varios lenguajes en un archivo.
- **Vue SFC (.vue)** — bloques `<template>` / `<script>` / `<style>`. Hay que extraer el
  `<script>` (JS/TS) y parsearlo con la gramática JS/TS ya existente. Tree-sitter tiene
  gramática `vue`, pero lo útil (funciones/imports) está en el `<script>`.
- **Astro (.astro)** — frontmatter JS/TS (`---...---`) + template JSX. Extraer el
  frontmatter y parsearlo como TS. 39 archivos, un solo proyecto.
- **Blade (.blade.php)** — HTML + PHP embebido + directivas Blade (`@if`, `{{ }}`).
  **Doble extensión:** el matcher debe evaluar `.blade.php` ANTES que `.php`, o los 340
  quedan mal clasificados. Opción pragmática: tratar el PHP embebido con la gramática PHP
  y las directivas Blade como texto.

### Prioridad recomendada
1. **Fix del bug** (bloqueante).
2. **Grupo A: CSS/SCSS + HTML** (barato, alto volumen).
3. **Vue** (1.357 archivos, 3 proyectos — el que más valor da del grupo B).
4. **Blade** (340, Laravel/sigec_v2).
5. **Astro** (39, un proyecto — el más marginal).

---

## 4. Exclusiones (evitar inflar el grafo con ruido)

Verificado en el censo: sin exclusiones, el grafo se llenaría de código de terceros y
artefactos generados. Agregar/reforzar exclusiones en `service.go` (ya excluye
vendor/testdata/.git/node_modules):
- **Código de terceros:** Moodle core, librerías vendorizadas.
- **Generado:** reportes HTML, `dist/`, `build/`, `.next/`, `.nuxt/`, minificados (`*.min.js`).
- **Proyectos sin código fuente** (censo): cipher-tools, cipher_v2, files, memorias-engram,
  skill-ley, corne-keyboard (solo firmware C).

---

## 5. Fases de implementación

| # | Fase | Archivos | Riesgo |
|---|---|---|---|
| 1 | Fix bug `language` (4 puntos §2) + test | parser.go, service.go, code_graph_tools.go, script | Bajo |
| 2 | Grupo A: CSS/SCSS/LESS + HTML (langSpecs) | register_treesitter.go, register_default.go, script | Bajo |
| 3 | Vue SFC: extraer `<script>` → parser JS/TS | nuevo `parser_vue.go` o pre-split en treesitter.go | Medio |
| 4 | Blade: doble extensión + PHP embebido | matcher de extensión + langSpec | Medio |
| 5 | Astro: frontmatter → TS | nuevo `parser_astro.go` | Medio |
| 6 | Exclusiones reforzadas + re-indexar proyectos | service.go + correr script por proyecto | Bajo |

Cada fase con su test (el repo ya tiene `*_test.go` de codegraph y e2e).

---

## 6. Decisión pendiente (vehículo)

- **Flow SDD nuevo** — ceremonial completo (Gherkin, verify, judge). El flow activo
  `a453fdf3` es de OTRO tema (fix reportería/hooks), NO tocar.
- **Directo** — commits convencionales por fase, sin SDD formal. Más rápido.

Recomendación: fase 1 (fix bug) puede ir directa hoy; el resto (lenguajes nuevos) se
beneficia de SDD si querés trazabilidad formal.
