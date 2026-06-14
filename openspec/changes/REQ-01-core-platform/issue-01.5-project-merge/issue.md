# issue-01.5-project-merge

**Origen:** `REQ-01-core-platform`
**Prioridad tentativa:** media
**Tipo:** feature

## Historia de usuario

**Como** equipo que cambia de repositorio o unifica proyectos
**Quiero** poder mergear dos proyectos de Domain (memorias, skills, flows, crons) en uno solo, y poder referenciar memorias de otro proyecto en modo lectura
**Para** no perder el historial cuando el repo cambia, y poder compartir conocimiento entre proyectos relacionados

## Criterios de aceptación

### Escenario 1: Mergear proyecto A en proyecto B

```gherkin
Dado que existen los proyectos "abc1" y "abc-docker"
Cuando ejecuto `domain project merge --from abc-docker --to abc1`
Entonces todas las observaciones de "abc-docker" se migran a "abc1"
Y los skills de "abc-docker" se migran a "abc1" (sin duplicar por nombre)
Y los flows de "abc-docker" se migran a "abc1"
Y los crons de "abc-docker" se migran a "abc1"
Y los agentes de "abc-docker" se migran a "abc1"
Y se registra un entry en `project_merges` con source, target y merge_log
Y "abc-docker" se marca como archived
```

### Escenario 2: Detección y resolución de conflictos

```gherkin
Dado que "abc1" y "abc-docker" tienen un flow con el mismo nombre "deploy"
Cuando ejecuto `domain project merge --from abc-docker --to abc1`
Entonces se detecta el conflicto en "deploy"
Y se renombra el flow duplicado a "deploy (from abc-docker)"
Y el merge_log registra el conflicto resuelto

Dado que ambos proyectos tienen una observación con el mismo hash SHA-256
Cuando se migran las observaciones
Entonces el duplicado se omite (dedup check)
Y el merge_log registra "N observaciones omitidas por duplicado"
```

### Escenario 3: Cross-project references (lectura)

```gherkin
Dado el proyecto "core-lib" con observaciones de arquitectura
Cuando enlazo "mi-app" a "core-lib" como dependencia de lectura
Entonces "mi-app" puede buscar observaciones de "core-lib"
Y `domain_mem_search` desde "mi-app" incluye resultados de "core-lib"
Pero "mi-app" NO puede escribir observaciones en "core-lib"
```

### Escenario 4: Detección de proyecto desde repositorio

```gherkin
Dado un proyecto con repository_url = "https://github.com/org/mi-repo"
Cuando ejecuto `domain project detect` desde un clon de "https://github.com/org/mi-repo"
Entonces detecta el proyecto por el git remote
Y muestra "Proyecto detectado: mi-repo (slug: mi-repo)"

Dado que NO existe proyecto para ese repositorio
Cuando ejecuto `domain project detect`
Entonces muestra "No se encontró proyecto para este repositorio"
Y pregunta "¿Crear proyecto nuevo desde template default?"
```

### Escenario 5: Migración por cambio de repositorio

```gherkin
Dado el proyecto "api-v1" con repository_url = "https://github.com/org/api-v1"
Y el repositorio cambió a "https://github.com/org/api-v2"
Cuando ejecuto `domain project relocate --new-repo "https://github.com/org/api-v2"`
Entonces se actualiza el repository_url del proyecto
Y se registra el cambio en audit_log
Y las observaciones, skills, flows y crons se mantienen intactos

Dado que ya existe un proyecto con el nuevo repository_url
Cuando ejecuto `domain project relocate --new-repo "https://github.com/org/api-v2"`
Entonces muestra error: "Ya existe un proyecto con ese repositorio. Usá merge en vez de relocate."
```

### Escenario 6: Listar proyectos enlazados

```gherkin
Dado que "mi-app" está enlazado a "core-lib"
Cuando ejecuto `domain project links`
Entonces muestra:
  | Project   | Linked Project | Access |
  | mi-app    | core-lib       | read   |
```

## Análisis breve

- **Qué pide realmente:** Merge de proyectos con migración de todas las entidades (observations, skills, flows, crons, agents), resolución de conflictos (rename + dedup), cross-project references (read-only), detección por git remote, y relocación por cambio de repo.
- **Módulos sospechados:** `internal/service/project/merge.go`, `internal/service/project/link.go`, `internal/service/project/detect.go`
- **Riesgos / dependencias:** Depende de issue-01.1 (migraciones 000022, 000023). Merge puede ser destructivo si no se hace backup. Las cross-project references requieren modificar las queries de search para incluir proyectos linkeados.
- **Esfuerzo tentativo:** XL

## Verificación previa

- [ ] Revisar codebase (grep)
- [ ] Revisar git log
- [ ] Probar en ambiente correcto
- [ ] Reproducir con perfil correcto
- [ ] Verificar caché / build
- [ ] Verificar feature flag / config

### Resultado de verificación

- **Estado:** pendiente
- **Evidencia:**
- **Acción derivada:**
