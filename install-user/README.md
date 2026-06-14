# install-user

Configurá tu laptop para usar Domain via MCP, apuntando al VPS donde corre
`domain serve --mcp-http`.

**Cero binarios a instalar.** Solo configs JSON + rules para que los clientes
MCP (Claude Code, Cursor, Cline, Continue, Claude Desktop) sepan que tienen
que hablar con tu VPS y que prefieran las tools `domain_*` sobre alternativas.

## Uso

### Interactivo (pide URL, email, API key)

```bash
./install-user.sh
```

### No-interactivo (flags)

```bash
./install-user.sh \
  --url http://1.2.3.4 \
  --email you@example.com \
  --api-key domk_live_xxxxxxxxxxxx
```

### Dry-run (detecta clientes, no toca configs)

```bash
./install-user.sh --dry-run
```

### Desinstalar (restaura backups)

```bash
./install-user.sh --uninstall
```

## Clientes soportados

| Cliente | Path config | Rules path |
|---|---|---|
| **claude-code** | `~/.claude/mcp_servers.json` | `~/.claude/instructions/domain.md` |
| **Cursor** | `~/.cursor/mcp.json` | `~/.cursor/rules/domain.mdc` |
| **Cline** (VS Code ext) | `<vscode>/.../cline_mcp_settings.json` | `~/.clinerules-domain` |
| **Continue** (VS Code ext) | `~/.continue/config.json` | embebido en config.json |
| **Claude Desktop** | `~/Library/.../claude_desktop_config.json` (macOS) o `~/.config/Claude/...` (Linux) | embebido |

Cada cliente detectado se configura solo si su path existe.

## Qué hace el script

1. **Detecta** clientes MCP instalados en paths conocidos.
2. **Pide** URL del VPS + email + API key (interactive o por flags).
3. **Verifica** que el VPS responde (`curl $VPS_URL/healthz`).
4. **Backup** de configs existentes con timestamp (`*.backup-YYYYMMDDTHHMMSSZ`).
5. **Inserta** el server `domain` en cada config (merge JSON con `jq` o write si no existe).
6. **Planta rules** desde `templates/` por cliente — esto es lo que hace que
   el LLM **prefiera** las tools `domain_*` sobre alternativas locales.
7. **Reporta** qué tocó.

## Idempotencia

Re-correr el script sobrescribe el server entry y las rules; los backups
quedan timestamped (no se sobrescriben). Seguro re-ejecutar.

## Desinstalador

`./install-user.sh --uninstall`:

- Restaura el backup más reciente de cada config.
- Si no hay backup (el script creó el archivo), remueve solo la entry `domain`.
- Borra los `domain.md` / `domain.mdc` / `.clinerules-domain` plantados.

**No toca otros MCPs ni configs no relacionadas.**

## Dependencias

- `curl` (siempre requerido).
- `jq` (recomendado; sin jq el script usa fallback que sobreescribe el JSON
  completo — solo aplicar si NO tenés otros MCPs configurados).

## OS soportados

- macOS
- Linux

Windows no soportado por ahora — usar WSL.

## Troubleshooting

### "Ningún cliente MCP detectado"

El script busca paths conocidos. Si tu cliente está en una ubicación distinta:
- Verificá que el cliente esté instalado.
- Si es una versión nueva con paths diferentes, abrí un issue.

### "VPS no responde"

El script continúa igual (la config se aplica). Verificá:
- URL correcta (con `http://` o `https://`).
- VPS arriba: `curl http://<vps-ip>/healthz`.
- Firewall del VPS permite el puerto.

### El LLM no usa `domain_*`

- Reiniciá el cliente MCP.
- Verificá que las rules están en su path (`~/.claude/instructions/domain.md`, etc.).
- Algunos clientes requieren cargar manualmente la rule (Cursor: panel de Rules).

### Conflicto con otra MCP

Sin `jq`, el script puede sobrescribir otras entries del mcp_servers.json.
Instalá `jq` (`brew install jq` o `apt install jq`) y re-corré.
