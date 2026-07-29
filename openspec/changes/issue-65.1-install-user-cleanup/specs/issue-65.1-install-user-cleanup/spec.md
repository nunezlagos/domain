# issue-65.1 — Limpieza y fix del install-user

Endurece el instalador de usuario en tres frentes: un bug con pérdida de datos
(Continue), backups faltantes, desalineaciones de documentación en los templates
y ruido (referencias muertas, archivo huérfano).

## Requisitos

### Requirement: configureContinue preserva los MCP servers existentes (bug)
`configureContinue` MUST mergear la entrada de domain en el array
`experimental.modelContextProtocolServers` de `~/.continue/config.json` en vez de
reemplazarlo. Si ya existe un server de domain, MUST actualizarlo en su lugar; si
no, MUST hacer append; los demás servers del usuario MUST conservarse.

#### Scenario: continue con otros servers los conserva
- **Given** un config.json de Continue con 2 MCP servers ajenos a domain
- **When** el installer configura Continue
- **Then** el array resultante contiene los 2 servers ajenos MÁS el de domain

#### Scenario: re-instalar no duplica el server de domain
- **Given** un config.json de Continue que ya tiene el server de domain
- **When** el installer configura Continue de nuevo
- **Then** el server de domain queda actualizado sin duplicarse

### Requirement: los writes destructivos crean backup previo
`uninstallClient` (rama Continue) y el paso `--remove-engram` MUST crear un
backup con timestamp del archivo antes de escribirlo, igual que el resto de las
mutaciones de config.

#### Scenario: uninstall de continue respalda antes de escribir
- **Given** un config.json de Continue con la entrada de domain
- **When** se desinstala y solo se limpia la entrada de continue
- **Then** existe un backup con timestamp del config.json antes del write

#### Scenario: remove-engram respalda settings.json
- **Given** un settings.json con engram activo
- **When** se corre el installer con --remove-engram
- **Then** existe un backup del settings.json antes de desactivar engram

### Requirement: los templates no citan tools ni scripts inexistentes (doc)
Los templates `claude-global.md` y `opencode-global.md` MUST referirse a
`domain_flow_status` (no a `domain_orchestrate_status`, que no existe), MUST
documentar el paso `_preview` en el issue workflow, y MUST marcar qué path de la
tabla Tool paths aplica a cada cliente. NO MUST advertir contra
`domain-code-graph.sh` (script inexistente), pero SÍ MUST conservar la nota de
que `domain_code_*` está deprecado.

#### Scenario: los templates usan el nombre real de la tool de status
- **Given** los templates del installer
- **When** se busca 'domain_orchestrate_status'
- **Then** no aparece; en su lugar se usa 'domain_flow_status'

#### Scenario: los templates no advierten contra un script inexistente
- **Given** los templates del installer
- **When** se busca 'domain-code-graph.sh'
- **Then** no aparece la advertencia; la nota de domain_code_* deprecado sigue presente

### Requirement: sin archivos huérfanos ni comentarios stale (ruido)
El repositorio del installer NO MUST contener `templates/personality.md` (huérfano,
sin referencias). El comentario de `claude_hook.go` sobre SessionStart NO MUST
mencionar el code graph. La descarga de Go en `bootstrap.sh` MUST tener un timeout
explícito en el curl.

#### Scenario: personality.md ya no existe
- **Given** el árbol del install-user
- **When** se lista templates/
- **Then** personality.md no está presente

#### Scenario: bootstrap.sh descarga Go con timeout
- **Given** bootstrap.sh
- **When** se inspecciona el curl que descarga Go
- **Then** incluye una bandera de timeout (-m)
