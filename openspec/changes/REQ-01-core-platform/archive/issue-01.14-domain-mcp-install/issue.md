# HU-01.14 — domain-mcp install + opencode.json auto-config

## Problema (user feedback)

User reporto:
> "hice la instalacion pero no me quedo claro domain ENOENT: no such file
> or directory, posix_spawn '/home/nunezlagos/Proyectos/domain-mcp/bin/domain'
> deberia haberse instalado bien en opencode no?"

Diagnostico:
1. `install.sh` solo compilaba `domain`, no `domain-mcp`. El binario nunca
   quedaba en `~/.local/bin/domain-mcp`. opencode intentaba ejecutar un
   path inexistente.
2. `runInstall` configuraba el opencode MCP server SOLO si era fresh install
   Y si habia API key en credentials.json. En fresh install, credentials.json
   no existe, asi que el setup se skipeaba siempre. Bug logico.
3. Cuando el setup SI corria pero el binario `domain-mcp` no estaba al lado
   del binario `domain`, el command se quedaba como `['']` (path vacio).
   SetupOpenCode retorna ErrAlreadyConfigured y NO actualiza el path. User
   tenia que borrar manualmente el entry.

## Goal

Una vez instalado `domain` y `domain-mcp` en `~/.local/bin/`, correr
`domain install --mode local --non-interactive` debe:

1. Compilar ambos binarios (domain + domain-mcp) via install.sh
2. Configurar `opencode.json` (cwd) con `mcp.domain.command = ['<path>/domain-mcp']`
3. Si el entry ya existia con `command = ['']` (install previo fallo),
   repararlo automaticamente y reescribir con el path correcto

## Acceptance Criteria

### AC1: install.sh compila ambos binarios
- `go build -o $INSTALL_DIR/domain ./cmd/domain`
- `go build -o $INSTALL_DIR/domain-mcp ./cmd/domain-mcp`
- Output final: `MCP server (para opencode/claude): ~/.local/bin/domain-mcp`

### AC2: opencode.json siempre se configura
- Step "Configuring opencode MCP server" corre SIEMPRE en runInstall
- No depende de `state.FirstRun` ni de la existencia de credentials.json
- Si no hay API key, configura sin env.DOMAIN_API_KEY
- Si ya hay API key, la incluye

### AC3: Reparacion de estado roto
- Antes de llamar setup.OpenCode, leer opencode.json del cwd
- Si `mcp.domain.command` es `[""]` o `[]`, borrar el entry
- Preserva todos los otros entries (context7, fetch, etc)
- Setup recrea con el path correcto

## Out of scope

- Auto-update del binario (install.sh cubre el caso)
- Windows (no soportado)
- Detectar cambios de PATH automaticamente

## Implementation plan (3 commits atomicos)

### Commit 1/3: install.sh compila domain-mcp
- install.sh: agregar `go build -o $INSTALL_DIR/domain-mcp ./cmd/domain-mcp`
- Output final: mencionar el path del MCP server

### Commit 2/3: runInstall configura opencode siempre
- install_cli.go: nuevo step 6 "Configuring opencode MCP server" (siempre corre)
- configureOpencodeMCPServer() helper que:
  a) Llama repairOpencodeEmptyCommand() (borra entry con command vacio)
  b) Llama runSetup() con --base-url (sin --api-key, opcional)
- Eliminar el setup de opencode de handleDeploymentMode (movido a runInstall)

### Commit 3/3: tests edge cases + state.yaml + archive + push
- install_cli_edge_test.go: 7 tests del repairOpencodeEmptyCommand
- state.yaml: implemented
- archive
- push a origin

## Archivos a tocar

```
openspec/changes/REQ-01-core-platform/issue-01.14-domain-mcp-install/  (nuevo)
install.sh                                              (commit 1)
cmd/domain/install_cli.go                               (commit 2)
cmd/domain/install_cli_edge_test.go                     (commit 3)
openspec/changes/.../state.yaml                         (commit 3)
```

## Tests

- TestRepairOpencodeEmptyCommand_NoFile (no hay opencode.json)
- TestRepairOpencodeEmptyCommand_NoMcpKey (opencode.json sin mcp)
- TestRepairOpencodeEmptyCommand_ValidCommand_NoRepair (path valido, no toca)
- TestRepairOpencodeEmptyCommand_EmptyString_Repairs (command=[""])
- TestRepairOpencodeEmptyCommand_MissingField_Repairs (command=[])
- TestRepairOpencodeEmptyCommand_PreservesOtherKeys (no borra context7/fetch)
- TestRepairOpencodeEmptyCommand_InvalidJSON_NoCrash

## Smoke test end-to-end

```bash
# Estado inicial: opencode.json con command=[''] (roto)
$ cat opencode.json | grep domain
    "domain": {
      "command": [""],

$ domain install --mode local --non-interactive
[7/5] Configuring opencode MCP server
    (reparado opencode.json con command vacio previo)
    ✓ Domain MCP agregado a .../opencode.json
    ✓ opencode.json updated (idempotent)

$ cat opencode.json | grep domain
    "domain": {
      "command": ["/home/nunezlagos/.local/bin/domain-mcp"],
```

## Riesgos

| Risk | Mitigation |
|------|------------|
| `domain-mcp` no se compila si `cmd/domain-mcp/` no existe | install.sh lo asume; si falla, mensaje claro |
| opencode.json con permisos raros | os.WriteFile con 0o600 (preserva) |
| Otros MCP servers se pierden | Tests verifican que context7/fetch/etc NO se tocan |
| Binario domain-mcp en path distinto a domain | findDomainMCPSibling ya maneja esto |
