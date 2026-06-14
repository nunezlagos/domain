# Proposal: issue-13.5-obsidian-plugin

## Intención

Crear un plugin TypeScript para Obsidian que sirva como interfaz gráfica para las operaciones de export/sync de engram. El plugin se comunica con el CLI de engram mediante subprocess (Node.js child_process), exponiendo comandos en la Command Palette de Obsidian y una Settings tab para configuración.

## Scope

**Incluye:**

- `plugin/obsidian/package.json` con dependencias: obsidian (API types), esbuild (build)
- `plugin/obsidian/manifest.json` con id, name, version, minAppVersion, author
- `plugin/obsidian/src/main.ts` — Plugin class con:
  - `onload()`: registro de 3 comandos (Export, Sync, Open Engram)
  - Settings tab con vault path, auto-sync toggle, sync interval
  - Notificaciones de progreso y error
  - Ejecución del CLI engram via child_process (o exec)
- `plugin/obsidian/src/settings.ts` — SettingsTab con campos configurables
- `plugin/obsidian/esbuild.config.js` — build config para generar main.js
- `plugin/obsidian/tsconfig.json` — TypeScript config

**No incluye:**

- UI custom dentro de Obsidian (solo commands + settings)
- Comunicación directa con Go (va via CLI subprocess)
- Hot reload en desarrollo

## Enfoque técnico

| Aspecto | Decisión |
|---------|----------|
| Lenguaje | TypeScript |
| Build | esbuild — bundle single main.js |
| Comunicación | `child_process.exec` del CLI `engram` |
| Configuración | Obsidian PluginSettings + data.json |
| UI | Notificaciones nativas de Obsidian (new Notice) |
| Commands | 3: Export, Sync, Open Engram |

## Riesgos

| Riesgo | Probabilidad | Mitigación |
|--------|-------------|------------|
| CLI no instalado en PATH | Media | Plugin detecta y muestra error con instrucciones |
| Build complejo | Baja | esbuild config standard; template oficial de Obsidian como referencia |
| Obsidian API changes | Baja | Usar minAppVersion conservador (0.15.0) |

## Testing

- **Manual:** Probar build, instalación en vault, carga de plugin, ejecución de comandos
- **Build:** `npm run build` genera main.js + manifest.json sin errores
