# issue-08.1-detection-chain

**Origen:** `REQ-08-project-resolution`
**Prioridad:** alta
**Tipo:** feature

## Historia de usuario

**Como** desarrollador usando memoria en múltiples proyectos
**Quiero** que el proyecto se detecte automáticamente mediante una cadena de 6 pasos
**Para** no tener que configurarlo manualmente cada vez y asegurar consistencia entre sesiones

## Criterios de aceptación

```gherkin
Scenario: Detection usa config file como paso primario
  Given existe .engram/config.json con "project": "my-app"
  When se ejecuta DetectProject() en ese directorio
  Then el resultado debe ser "my-app"
  And no debe ejecutar pasos posteriores de detección

Scenario: Fallback a git remote cuando no hay config file
  Given no existe .engram/config.json
  And el directorio tiene un repositorio git con remote "origin" apuntando a "github.com/user/my-app.git"
  When se ejecuta DetectProject()
  Then el resultado debe ser "my-app"

Scenario: Fallback a git root directory name
  Given no existe .engram/config.json
  And no hay git remote configurado
  And el directorio está dentro de un repositorio git cuyo root se llama "my-app"
  When se ejecuta DetectProject()
  Then el resultado debe ser "my-app"

Scenario: Fallback a git child directory name
  Given no existe .engram/config.json
  And no hay git remote
  And git root no es un nombre significativo
  And el cwd está en un subdirectorio "frontend" dentro del repo
  When se ejecuta DetectProject()
  Then el resultado debe ser "frontend"

Scenario: Detección ambigua retorna error con sugerencias
  Given múltiples candidatos de proyecto detectados
  When se ejecuta DetectProject()
  Then debe retornar error "ambiguous project detection"
  And la respuesta debe incluir los candidatos como sugerencias

Scenario: Fallback final a directory basename
  Given ningún paso anterior produce resultado
  When se ejecuta DetectProject() en /home/user/projects/mi-app
  Then el resultado debe ser "mi-app"

Scenario: Child scan respeta depth=1 y max 20 directorios
  Given un repositorio con más de 20 subdirectorios en la raíz
  When se ejecuta el child scan
  Then solo debe escanear depth=1
  And debe procesar máximo 20 directorios

Scenario: Child scan tiene timeout de 200ms
  Given un repositorio con child scan lento (NFS, FUSE)
  When se ejecuta el child scan
  Then debe abortar después de 200ms
  And debe loguear warning de timeout

Scenario: Child scan salta directorios noise
  Given un repositorio con directorios .git, node_modules, vendor, __pycache__, .venv, dist, build, target
  When se ejecuta el child scan
  Then esos directorios no deben ser considerados candidatos

Scenario: Pipeline completo retorna resumen de cada paso
  Given se ejecuta DetectProject()
  When la detección es exitosa
  Then el resultado incluye: source (qué paso ganó), value, confidence, candidates descartados con razón
```

## Análisis breve

- **Qué pide realmente:** Pipeline de detección con 6 strategies en orden de precedencia, child scan con restricciones de profundidad/límite/timeout, skip de noise dirs, reporte de candidates descartados
- **Módulos sospechados:** `internal/project/` — `detect.go` con `DetectProject()`, strategies como `ConfigFileStrategy`, `GitRemoteStrategy`, etc.
- **Riesgos / dependencias:** Git operations pueden ser lentas en repos grandes; timeout es crítico; detección ambigua debe ser informativa no blocker
- **Esfuerzo tentativo:** M

## Verificación previa

- [ ] Revisar codebase (grep)
- [ ] Revisar memorias engram (domain_mem_search)
- [ ] Revisar git log
- [ ] Probar en ambiente correcto
- [ ] Reproducir con perfil correcto
- [ ] Verificar caché / build
- [ ] Verificar feature flag / config

### Resultado de verificación

- **Estado:** pendiente
- **Evidencia:** —
- **Acción derivada:** —
