# install-user

Configurá tu laptop para usar Domain via MCP, apuntando al VPS donde corre
`domain serve --mcp-http`.

**Cross-platform**: Linux (Ubuntu/Debian/Arch), macOS (Intel + Apple
Silicon), Windows (nativo), WSL2. Binario Go estático ~6 MB sin
dependencias externas (no necesitás jq, python, ni nada más).

## Uso — 1 comando

### Linux / macOS / WSL2

```bash
git clone https://github.com/nunezlagos/domain.git -b services
cd domain/install-user
./bootstrap.sh
```

Eso es todo. El `bootstrap.sh`:

1. Detecta si tenés Go (>= 1.22). Si **no** lo tenés, baja Go oficial a
   `~/.local/go/` — **sin sudo, sin tocar `/usr`**, sin afectar tu PATH
   global.
2. Compila `domain-install` (~6 MB).
3. Lo ejecuta. Te pregunta interactivamente la URL del VPS, email y API
   key (o pasás los args).

Re-ejecutar es instantáneo: si el Go ya está bajado y el binario ya está
compilado, salta esas fases.

### Windows nativo

```powershell
git clone https://github.com/nunezlagos/domain.git -b services
cd domain\install-user
.\bootstrap.ps1
```

Mismo flujo: detecta Go, lo baja a `%LOCALAPPDATA%\go-domain\` si falta
(sin admin), compila `domain-install.exe`, lo ejecuta.

Si Windows bloquea la ejecución del script:
```powershell
Set-ExecutionPolicy -Scope CurrentUser -ExecutionPolicy RemoteSigned
```

### Flags

Los mismos para `bootstrap.sh` y `bootstrap.ps1`:

```bash
./bootstrap.sh                                       # interactive
./bootstrap.sh --url http://1.2.3.4 \
               --email u@x.cl \
               --api-key domk_live_xxx
./bootstrap.sh --uninstall                           # deshacer todo
./bootstrap.sh --dry-run                             # solo detectar clientes
```

En PowerShell se pasan como parámetros nombrados: `.\bootstrap.ps1 -Url
http://... -Email ... -ApiKey ...`.

### Avanzado: build manual sin bootstrap

Si ya tenés Go instalado y querés saltarte el bootstrap:

```bash
cd install-user
make            # binario domain-install nativo
make dist       # cross-compile a los 5 targets (linux/macOS/windows)
./domain-install
```

### Legacy: script bash sin Go

```bash
./install-user.sh
```

El bash original sigue funcionando como fallback, sin necesidad de Go ni
de bootstrap, pero **NO funciona en Windows nativo** (use bootstrap.ps1
en ese caso).

## Filosofía: 2 archivos en disco, el resto vive en la BD

El protocolo de uso de domain (cuándo llamar `mem_save`, cómo manejar
errores, qué hacer al iniciar sesión, etc.) **NO se planta como `.md`
sueltos por todo el filesystem**. Vive en BD como policy
`agent-protocol`, editable con `domain_policy_update` y versionada. El
MCP server lo inyecta al cliente IDE en cada `initialize` handshake via
el campo estándar `instructions` del protocolo MCP.

En disco quedan **2 archivos compartidos por todos los clientes**:

| Archivo | Rol |
|---|---|
| `~/.claude/skills/domain/SKILL.md` | Bootstrap on-demand. 12 líneas. Le dice al LLM "llamá `domain_policy_get('agent-protocol')` para el protocolo vivo". |
| `~/.claude/agents/domain-memory.md` | Subagent read-only para delegar recall profundo sin bloatear contexto. |

Más, por cada cliente detectado, **un único archivo de config MCP**
(transport: URL del VPS + Bearer key). Eso no es "memoria", es plumbing
indispensable.

OpenCode comparte los mismos 2 archivos via `symlink` (no se duplican).

## Uso

```bash
./install-user.sh                          # interactivo
./install-user.sh --url http://1.2.3.4 \
                  --email you@example.com \
                  --api-key domk_live_xxx
./install-user.sh --dry-run                # solo detecta
./install-user.sh --uninstall              # restaura backups + borra los 2 globales
```

## Clientes soportados

| Cliente | Config MCP (transport) | Skill + Agent |
|---|---|---|
| **claude-code** | `~/.claude/mcp_servers.json` | en `~/.claude/skills/` y `~/.claude/agents/` (paths nativos) |
| **opencode** | `~/.config/opencode/opencode.json` | symlinks a los globales en `~/.claude/...` |
| **Cursor** | `~/.cursor/mcp.json` | — recibe protocolo por handshake MCP |
| **Cline** (VS Code ext) | `<vscode>/.../cline_mcp_settings.json` | — recibe protocolo por handshake MCP |
| **Continue** (VS Code ext) | `~/.continue/config.json` | — recibe protocolo por handshake MCP |
| **Claude Desktop** | `~/Library/.../claude_desktop_config.json` (macOS) o `~/.config/Claude/...` (Linux) | — recibe protocolo por handshake MCP |

> Cursor/Cline/Continue/Claude Desktop dependen de que su versión del
> cliente respete el campo `instructions` del MCP initialize. Las
> versiones actuales (2026) lo hacen. Si tu cliente quedó atrás,
> actualizalo.

## Editar el protocolo (sin tocar archivos)

```
domain_policy_get(slug="agent-protocol")        # lee la versión actual
domain_policy_update(slug="agent-protocol", body_md="...")  # nueva versión
```

Próximo cliente que arranque sesión la recibe en el handshake.

## ¿Qué pasa cuando entro a un proyecto clonado que ya tiene `.claude/`, `AGENTS.md`, `CLAUDE.md` propios?

El cliente IDE carga **ambas capas** en el system prompt al iniciar la
sesión:

1. **Tu config global** (`~/.claude/skills/domain/`, `~/.claude/agents/`,
   `~/.claude/CLAUDE.md` si tenés).
2. **MCP server `instructions`** (el protocolo `agent-protocol` que sale
   del handshake con tu VPS de domain).
3. **Lo del proyecto clonado** (`<repo>/.claude/`, `<repo>/CLAUDE.md`,
   `<repo>/AGENTS.md`, sus skills/agents locales si trae).

**No se excluyen — se suman.** Domain entra automáticamente en todo
proyecto porque su MCP está registrado globalmente y el `instructions`
del handshake siempre se inyecta.

El único caso de conflicto real es:

- **Mismo nombre de skill o agent**: si el proyecto clonado trae un
  archivo `.claude/skills/domain/SKILL.md` distinto al tuyo, gana el
  más cercano al cwd (el del proyecto). En la práctica es muy
  improbable que un repo random llame su skill "domain" — el espacio
  de nombres está mayoritariamente libre.
- **MCP server con mismo nombre**: si el proyecto clonado define un
  server `domain` en su `.claude/mcp_servers.json`, sobreescribe tu
  config global mientras estés en ese directorio. Renombrá el del
  proyecto o el tuyo si pasa.

## Idempotencia

Re-correr el script sobreescribe el server entry, refresca los 2
archivos globales (con backup `.backup-YYYYMMDDTHHMMSSZ`) y deja los
symlinks de opencode apuntando a los globales. Seguro re-ejecutar.

## Dependencias

- `curl` (siempre requerido).
- `jq` (recomendado; sin jq el script crea el JSON desde cero — solo
  aplicar si NO tenés otros MCPs configurados).

## OS soportados

- macOS
- Linux

Windows: usá WSL.

## Troubleshooting

**El LLM no usa tools `domain_*` aunque las ve listadas**
Reiniciá el cliente MCP. Si persiste, verificá que el cliente esté en
una versión que respete el campo `instructions` del MCP initialize. Como
fallback, mirá `~/.claude/skills/domain/SKILL.md` — el LLM debería
cargarlo on-demand por la descripción.

**"VPS no responde"**
URL correcta (`http://` o `https://`), VPS arriba (`curl
$VPS_URL/healthz`), firewall del VPS abierto.

**Quiero editar el protocolo sin reinstalar**
`domain_policy_update(slug="agent-protocol", body_md=<nuevo>)`. La
próxima sesión MCP lo recibe en el handshake.

**Conflicto en proyecto clonado**
Mirá la sección de arriba "qué pasa cuando entro a un proyecto
clonado". Renombrá el archivo del proyecto o el tuyo si hay colisión
exacta.
