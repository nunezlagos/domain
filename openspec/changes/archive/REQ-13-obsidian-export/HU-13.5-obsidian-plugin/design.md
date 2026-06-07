# Design: HU-13.5-obsidian-plugin

## Decisión arquitectónica

### Directory structure

```
plugin/obsidian/
├── manifest.json
├── package.json
├── tsconfig.json
├── esbuild.config.js
├── src/
│   ├── main.ts
│   └── settings.ts
└── dist/          (generado por build)
    ├── main.js
    └── manifest.json
```

### manifest.json

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

### Plugin settings interface

```typescript
interface EngramPluginSettings {
    vaultPath: string;
    autoSync: boolean;
    syncIntervalSeconds: number;
}

const DEFAULT_SETTINGS: EngramPluginSettings = {
    vaultPath: '',
    autoSync: false,
    syncIntervalSeconds: 300, // 5 minutos
};
```

### main.ts plugin entry

```typescript
import { Plugin, Notice, addIcon } from 'obsidian';
import { EngramSettingsTab } from './settings';

export default class EngramPlugin extends Plugin {
    settings: EngramPluginSettings;

    async onload() {
        await this.loadSettings();

        // Command: Export observations
        this.addCommand({
            id: 'engram-export',
            name: 'Export observations',
            callback: () => this.runExport(),
        });

        // Command: Sync vault
        this.addCommand({
            id: 'engram-sync',
            name: 'Sync vault',
            callback: () => this.runSync(),
        });

        // Command: Open engram
        this.addCommand({
            id: 'engram-open',
            name: 'Open engram',
            callback: () => this.openEngram(),
        });

        // Settings tab
        this.addSettingTab(new EngramSettingsTab(this.app, this));

        // Auto-sync interval
        if (this.settings.autoSync) {
            this.registerInterval(
                window.setInterval(
                    () => this.runSync(),
                    this.settings.syncIntervalSeconds * 1000
                )
            );
        }
    }

    async runExport() {
        new Notice('Exporting observations...');
        try {
            const result = await this.execEngram(`export --vault "${this.settings.vaultPath}"`);
            new Notice(`Export complete: ${result}`);
        } catch (err) {
            new Notice(`Export failed: ${err.message}`);
        }
    }

    async runSync() {
        new Notice('Syncing engram vault...');
        try {
            const result = await this.execEngram(
                `export --vault "${this.settings.vaultPath}" --force --include-hub-notes --graph-mode force`
            );
            new Notice(`Sync complete`);
        } catch (err) {
            new Notice(`Sync failed: ${err.message}`);
        }
    }

    async openEngram() {
        // Open vault in Obsidian or show path
        if (this.settings.vaultPath) {
            // Try to open as another vault (requires vault switch)
            new Notice(`Engram vault: ${this.settings.vaultPath}`);
        } else {
            new Notice('Configure vault path in Engram settings');
        }
    }

    async execEngram(args: string): Promise<string> {
        const { exec } = require('child_process');
        return new Promise((resolve, reject) => {
            exec(`engram obsidian ${args}`, (error, stdout, stderr) => {
                if (error) reject(new Error(stderr || error.message));
                else resolve(stdout.trim());
            });
        });
    }

    async loadSettings() {
        this.settings = Object.assign(
            {},
            DEFAULT_SETTINGS,
            await this.loadData()
        );
    }

    async saveSettings() {
        await this.saveData(this.settings);
    }
}
```

### settings.ts

```typescript
import { App, PluginSettingTab, Setting } from 'obsidian';
import EngramPlugin from './main';

export class EngramSettingsTab extends PluginSettingTab {
    plugin: EngramPlugin;

    constructor(app: App, plugin: EngramPlugin) {
        super(app, plugin);
        this.plugin = plugin;
    }

    display(): void {
        const { containerEl } = this;
        containerEl.empty();

        containerEl.createEl('h2', { text: 'Engram Vault Settings' });

        new Setting(containerEl)
            .setName('Vault path')
            .setDesc('Path to your engram Obsidian vault')
            .addText(text => text
                .setPlaceholder('/path/to/vault')
                .setValue(this.plugin.settings.vaultPath)
                .onChange(async (value) => {
                    this.plugin.settings.vaultPath = value;
                    await this.plugin.saveSettings();
                }));

        new Setting(containerEl)
            .setName('Auto-sync')
            .setDesc('Automatically sync on interval')
            .addToggle(toggle => toggle
                .setValue(this.plugin.settings.autoSync)
                .onChange(async (value) => {
                    this.plugin.settings.autoSync = value;
                    await this.plugin.saveSettings();
                }));

        new Setting(containerEl)
            .setName('Sync interval (seconds)')
            .setDesc('How often to auto-sync')
            .addText(text => text
                .setPlaceholder('300')
                .setValue(String(this.plugin.settings.syncIntervalSeconds))
                .onChange(async (value) => {
                    const num = parseInt(value);
                    if (!isNaN(num) && num > 0) {
                        this.plugin.settings.syncIntervalSeconds = num;
                        await this.plugin.saveSettings();
                    }
                }));
    }
}
```

### esbuild.config.js

```javascript
const esbuild = require('esbuild');
const fs = require('fs');

const prod = process.argv[2] === 'production';

esbuild.build({
    entryPoints: ['src/main.ts'],
    bundle: true,
    external: ['obsidian'],
    format: 'cjs',
    target: 'es2020',
    outfile: 'dist/main.js',
    sourcemap: prod ? false : 'inline',
    minify: prod,
}).then(() => {
    // Copy manifest.json to dist
    fs.copyFileSync('manifest.json', 'dist/manifest.json');
    console.log('Build complete');
}).catch(() => process.exit(1));
```

### package.json

```json
{
  "name": "engram-obsidian-plugin",
  "version": "0.1.0",
  "description": "Export and sync engram memories to Obsidian",
  "scripts": {
    "build": "node esbuild.config.js production",
    "dev": "node esbuild.config.js"
  },
  "devDependencies": {
    "obsidian": "^1.0.0",
    "esbuild": "^0.20.0",
    "typescript": "^5.0.0"
  }
}
```

### Security consideration

El plugin ejecuta comandos del sistema via `child_process.exec`. Para evitar command injection:

```typescript
async execEngram(args: string): Promise<string> {
    // Sanitizar vaultPath: solo caracteres seguros
    const vaultPath = this.settings.vaultPath.replace(/[^a-zA-Z0-9_\/\-\.]/g, '');
    const fullCmd = `engram obsidian export --vault "${vaultPath}"`;
    // ...
}
```

## Alternativas descartadas

| Alternativa | Razón de descarte |
|-------------|-------------------|
| Comunicación directa Go → plugin via WebSocket | Complejidad innecesaria; CLI subprocess es más simple y ya existe |
| Python en vez de TypeScript | Obsidian plugins son TypeScript/JS nativos |
| Custom view en Obsidian | Commands + settings es suficiente; custom view agrega complejidad sin valor claro |
| No plugin (solo CLI) | El plugin es opcional y mejora UX para usuarios de Obsidian |

## TDD plan

No aplica TDD para TypeScript plugin (no hay test runner configurado). Verificación manual:

1. Build: `npm run build` → genera dist/main.js + manifest.json
2. Copiar a `.obsidian/plugins/engram-vault/` en un vault de prueba
3. Cargar Obsidian → plugin aparece en lista
4. Configurar vault path
5. Ejecutar Export → notificación de éxito
6. Ejecutar Sync → notificación de éxito
7. Verificar archivos generados en vault path

## Riesgos y mitigación

| Riesgo | Mitigación |
|--------|------------|
| Command injection via vaultPath | Sanitizar input; escapado con comillas dobles |
| esbuild no disponible | Documentar que `npm install` es necesario para build |
| Obsidian API deprecations | Usar APIs estables; target minAppVersion conservador |
| macOS Gatekeeper bloquea main.js | Firmar no es necesario; el usuario carga como plugin comunitario |
