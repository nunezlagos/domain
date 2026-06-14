# Tasks: issue-13.5-obsidian-plugin

## Plugin (TypeScript)

- [ ] **P1: Crear estructura de directorios**
      ```
      plugin/obsidian/
      ├── src/
      │   ├── main.ts
      │   └── settings.ts
      ├── manifest.json
      ├── package.json
      ├── tsconfig.json
      └── esbuild.config.js
      ```

- [ ] **P2: Crear manifest.json**
      ```json
      {
        "id": "engram-vault",
        "name": "Engram Vault",
        "version": "0.1.0",
        "minAppVersion": "0.15.0",
        "description": "Export and sync your engram development memories to Obsidian",
        "author": "engram",
        "isDesktopOnly": true
      }
      ```

- [ ] **P3: Crear package.json con dependencias**
      - `obsidian` (API types, devDependency)
      - `esbuild` (build tool, devDependency)
      - `typescript` (devDependency)
      - Scripts: `build`, `dev`

- [ ] **P4: Crear tsconfig.json**
      - target: ES2020
      - module: ESNext
      - moduleResolution: node
      - strict: true

- [ ] **P5: Crear esbuild.config.js**
      - Entry: src/main.ts
      - External: obsidian
      - Format: cjs
      - Outfile: dist/main.js
      - Copy manifest.json to dist/
      - Sourcemap in dev, minify in production

- [ ] **P6: Implementar interfaces y defaults en main.ts**
      ```typescript
      interface EngramPluginSettings {
          vaultPath: string;
          autoSync: boolean;
          syncIntervalSeconds: number;
      }

      const DEFAULT_SETTINGS: EngramPluginSettings = {
          vaultPath: '',
          autoSync: false,
          syncIntervalSeconds: 300,
      };
      ```

- [ ] **P7: Implementar Plugin class con onload**
      - `loadSettings()` desde plugin.data
      - Registrar 3 comandos:
        - `engram-export` → runExport()
        - `engram-sync` → runSync()
        - `engram-open` → openEngram()
      - Agregar SettingsTab
      - Auto-sync interval si settings.autoSync

- [ ] **P8: Implementar execEngram con child_process**
      - Sanitizar vaultPath (solo `[a-zA-Z0-9_/\-.]`)
      - Exec: `engram obsidian export --vault "${vaultPath}" [args]`
      - Retornar Promise<string>
      - Error handling con stderr

- [ ] **P9: Implementar comandos**
      - `runExport()`:
        1. `new Notice('Exporting observations...')`
        2. execEngram(`export --vault "${vault}"`)
        3. `new Notice('Export complete: N observations')`
        4. Catch → `new Notice('Export failed: ...')`
      - `runSync()`:
        1. `new Notice('Syncing vault...')`
        2. execEngram(`export --vault "${vault}" --force --include-hub-notes --graph-mode force`)
        3. `new Notice('Sync complete')`
      - `openEngram()`:
        1. Mostrar notice con vault path

- [ ] **P10: Implementar SettingsTab**
      ```typescript
      class EngramSettingsTab extends PluginSettingTab {
          display(): void {
              // Vault path (text input)
              // Auto-sync (toggle)
              // Sync interval (number input)
          }
      }
      ```

- [ ] **P11: Implementar saveSettings / loadSettings**
      - `loadSettings()`: merge DEFAULT_SETTINGS + this.loadData()
      - `saveSettings()`: this.saveData(this.settings)

## Build

- [ ] **B1: `npm install` en plugin/obsidian/**

- [ ] **B2: `npm run build` genera dist/main.js + dist/manifest.json**
      - Verificar que main.js existe y no excede 1MB
      - Verificar que manifest.json se copia correctamente

## Tests

- [ ] **T1: Verificar build sin errores**
      ```bash
      cd plugin/obsidian && npm install && npm run build
      ```

- [ ] **T2: Verificar manifest.json válido**
      - id: "engram-vault"
      - name: "Engram Vault"
      - minAppVersion: "0.15.0"
      - version: formato semver
      - isDesktopOnly: true

- [ ] **T3: Verificar main.js syntax**
      - `node -c dist/main.js` no debe dar error

## Sabotaje

- [ ] **S1: Romper sanitización de vaultPath**
      1. Eliminar el `.replace(/[^a-zA-Z0-9_\/\-\.]/g, '')` en execEngram
      2. Ejecutar build → build pasa (no hay test que atrape esto)
      3. Pero desde Obsidian, un vaultPath con comillas podría causar command injection
      4. Restaurar sanitización
      5. Documentar: "la sanitización de vaultPath es crítica para seguridad"

## Cierre

- [ ] `cd plugin/obsidian && npm run build` — build exitoso
- [ ] Verificar dist/ contiene main.js + manifest.json
- [ ] Probar carga en Obsidian (manual):
      1. Copiar dist/ a {vault}/.obsidian/plugins/engram-vault/
      2. Activar plugin en Community Plugins
      3. Configurar vault path en settings
      4. Ejecutar Export command
- [ ] Commit: `feat: obsidian typescript plugin with esbuild and export commands`
