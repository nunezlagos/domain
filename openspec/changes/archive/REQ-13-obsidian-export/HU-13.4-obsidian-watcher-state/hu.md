# HU-13.4-obsidian-watcher-state

**Origen:** `REQ-13-obsidian-export`
**Prioridad:** media
**Tipo:** feature

## Historia de usuario

**Como** usuario de engram
**Quiero** que el vault de Obsidian se mantenga sincronizado automáticamente mientras trabajo
**Para** no tener que ejecutar manualmente el export cada vez que creo una observación

**Como** usuario power
**Quiero** que el export sea incremental, trackeando en un state file cuándo fue la última exportación por proyecto
**Para** que solo se exporten las observaciones nuevas o modificadas, ahorrando tiempo en vaults grandes

**Como** usuario power
**Quiero** poder forzar un export completo con `--force` que ignore el state file
**Para** regenerar todo cuando sea necesario (cambios de slug, nuevas relaciones, etc.)

## Criterios de aceptación

```gherkin
Scenario: File watcher monitorea cambios en la store
  Given el watcher está activo
  When se crea una nueva observación
  Then el watcher detecta el cambio
  And ejecuta el export incremental automáticamente

Scenario: Watcher usa fsnotify para detectar cambios
  Given el watcher está configurado
  When hay cambios en la store
  Then fsnotify events son recibidos por el watcher

Scenario: State file se crea en el vault al primer export
  Given un vault sin state file
  When se ejecuta export por primera vez
  Then se crea .engram-state.yaml en la raíz del vault
  And contiene last_export con timestamp actual

Scenario: State file trackea last_export por proyecto
  Given vault con state file existente
  When se ejecuta export
  Then state file se actualiza con last_export = now
  And last_export es específico por proyecto

Scenario: Export incremental usa state file como filtro
  Given state file indica last_export = "2026-06-07T10:00:00Z"
  When se ejecuta export sin --force
  Then solo se exportan observaciones con updated_at > "2026-06-07T10:00:00Z"

Scenario: --force ignora state file y exporta todo
  Given state file con last_export reciente
  When se ejecuta export con --force
  Then se exportan todas las observaciones sin filtro temporal
  And state file se actualiza al finalizar

Scenario: State file contiene slugs exportados
  Given se exportaron observaciones con slugs "bug-fix" y "fix-timezone"
  When se consulta el state file
  Then contiene exported_slugs con mapeo id → slug

Scenario: Slugs persistidos en state evitan colisiones entre exports
  Given state file tiene slug "bug-fix" → id=1
  When se exporta una nueva obs con título "Bug fix"
  Then el slug generado es "bug-fix-2" (no colisiona con el existente)

Scenario: Watcher se detiene gracefulmente
  Given el watcher está activo
  When recibe señal de terminación (SIGTERM, SIGINT)
  Then el watcher se detiene gracefulmente
  And no corrompe el state file

Scenario: Watcher configura intervalo de debounce
  Given eventos de cambio continuos en la store
  When se configuró debounce de 5 segundos
  Then el watcher espera 5 segundos sin cambios antes de ejecutar export

Scenario: State file corrupto no bloquea export
  Given .engram-state.yaml tiene formato inválido
  When se ejecuta export
  Then se ignora el state file (o se recrea)
  And se exporta completo (como con --force)
  And se loguea un warning
```

## Análisis breve

- **Qué pide realmente:** Sistema de file watching + state tracking. El watcher monitorea la store engram y ejecuta export incremental cuando hay cambios. El state file (.engram-state.yaml) persiste last_export timestamp y slugs exportados para permitir export incremental y evitar colisiones entre sesiones.
- **Módulos sospechados:** `internal/obsidian/` — `watcher.go` con Watcher struct usando fsnotify; `state.go` con VaultState load/save
- **Riesgos / dependencias:** Depende de HU-13.1 (exporter) y HU-13.2 (graph); fsnotify es dependencia externa; debounce es necesario para evitar múltiples exports en cascada
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
