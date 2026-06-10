# issue-13.5-obsidian-plugin

**Origen:** `REQ-13-obsidian-export`
**Prioridad:** media
**Tipo:** feature

## Historia de usuario

**Como** usuario de Obsidian
**Quiero** tener un plugin que agregue comandos para exportar, sync y abrir el engram vault
**Para** ejecutar estas acciones directamente desde Obsidian sin ir a la terminal

**Como** desarrollador del plugin
**Quiero** usar TypeScript con esbuild para build rápido y manifest.json estándar
**Para** seguir las mejores prácticas de desarrollo de plugins Obsidian

## Criterios de aceptación

```gherkin
Scenario: Plugin se carga correctamente en Obsidian
  Given el plugin está instalado en .obsidian/plugins/engram-vault/
  When Obsidian inicia
  Then el plugin se carga sin errores
  And aparece en la lista de plugins comunitarios

Scenario: Plugin registra comando "Export to engram"
  Given el plugin está activo
  When abro Command Palette
  Then veo el comando "Engram: Export observations"
  And al ejecutarlo, se dispara el export de observaciones a markdown

Scenario: Plugin registra comando "Sync vault"
  Given el plugin está activo
  When ejecuto "Engram: Sync vault"
  Then ejecuta sync completo (export + graph + hubs)

Scenario: Plugin registra comando "Open engram"
  Given el plugin está activo
  When ejecuto "Engram: Open engram"
  Then se abre la vista de engram (o se navega al vault)

Scenario: manifest.json tiene campos correctos
  Given el plugin build
  When se inspecciona manifest.json
  Then contiene:
    | id             | "engram-vault"                |
    | name           | "Engram Vault"                |
    | minAppVersion  | "0.15.0"                      |
    | version        | match semver                  |
    | description    | no vacío                      |
    | author         | "engram"                      |

Scenario: Plugin se build con esbuild
  Given el proyecto plugin/
  When se ejecuta `npm run build`
  Then se genera main.js + manifest.json en dist/
  And main.js no excede 1MB

Scenario: Plugin respeta configuración de vault path
  Given el plugin tiene config con vaultPath
  When ejecuto cualquier comando
  Then usa vaultPath de la config del plugin

Scenario: Plugin informa progreso al usuario
  Given se ejecuta el comando "Export observations"
  When el export está en progreso
  Then se muestra una notificación "Exporting observations..."
  When el export termina
  Then se muestra "Export complete: 42 observations"

Scenario: Plugin maneja errores gracefulmente
  Given el vault path no existe
  When ejecuto "Export observations"
  Then se muestra una notificación de error
  And el plugin no crashea

Scenario: Plugin tiene settings tab
  Given el plugin está activo
  When abro Settings → Community Plugins → Engram Vault
  Then veo campos de configuración:
    - Vault path (text input)
    - Auto-sync toggle
    - Sync interval (number, en segundos)
```

## Análisis breve

- **Qué pide realmente:** Plugin TypeScript para Obsidian que sirve como interfaz gráfica dentro de Obsidian para disparar las operaciones de export/sync. Se comunica con el CLI de engram (o directamente con Go via subprocess). Build con esbuild. Sigue la convención de plugins Obsidian (main.js + manifest.json).
- **Módulos sospechados:** `plugin/obsidian/` — `src/main.ts` plugin entry, `src/settings.ts` settings tab, `manifest.json`, `package.json`, `esbuild.config.js`
- **Riesgos / dependencias:** Depende de issue-13.1-13.4 para funcionalidad; el plugin es un wrapper que ejecuta el CLI de engram como subprocess; requiere Node.js para build pero no para runtime (es standalone JS)
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
