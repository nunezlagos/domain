# issue-12.5-agent-setup

**Origen:** `REQ-12-mcp-server`
**Prioridad tentativa:** alta
**Tipo:** feature

## Historia de usuario

**Como** desarrollador usando Claude Code / OpenCode / Codex / Cline
**Quiero** ejecutar `domain setup` para configurar automáticamente el agente con Domain como servidor MCP principal
**Para** que Domain reemplace las tools nativas del agente, coexistiendo sin tocar archivos locales sensibles

## Criterios de aceptación

### Escenario 1: Setup detecta el agente instalado

```gherkin
Dado que tengo Claude Code y OpenCode instalados
Cuando ejecuto `domain setup`
Entonces Domain detecta automáticamente los agentes disponibles
Y muestra un selector interactivo para elegir cuál configurar
Y pregunta confirmación antes de modificar archivos
```

### Escenario 2: Configurar Claude Code

```gherkin
Dado que elegí configurar Claude Code
Cuando ejecuto `domain setup claude-code`
Entonces Domain localiza o crea `~/.config/Claude/claude_desktop_config.json`
Y agrega el servidor MCP de Domain con:
  | key          | value                        |
  | command      | "domain-mcp"                 |
  | args         | []                           |
  | transport    | "stdio"                      |
Y crea el directorio `.ai/` en el proyecto actual
Y crea `.ai/directives.md` con instrucciones para usar tools Domain
Y no modifica ningún archivo en `.env`, `.git/`, `*.pem`, `*.key`, `credentials.*`
```

### Escenario 3: Configurar OpenCode

```gherkin
Dado que elegí configurar OpenCode
Cuando ejecuto `domain setup opencode`
Entonces Domain localiza o crea `.opencode/mcp.json`
Y agrega el servidor MCP de Domain con:
  | key          | value        |
  | command      | "domain-mcp" |
  | args         | []           |
Y crea `.ai/directives.md` con instrucciones para usar tools Domain
```

### Escenario 4: Configurar Codex / Cline

```gherkin
Dado que elegí configurar Codex
Cuando ejecuto `domain setup codex`
Entonces Domain localiza o crea `~/.codex/config.json`
Y agrega el servidor MCP de Domain

Dado que elegí configurar Cline
Cuando ejecuto `domain setup cline`
Entonces Domain localiza o crea `~/.config/Cline/mcp_settings.json`
Y agrega el servidor MCP de Domain
```

### Escenario 5: .ai/directives.md contiene instrucciones correctas

```gherkin
Dado que `domain setup` se ejecutó exitosamente
Cuando reviso `.ai/directives.md`
Entonces contiene:
  | "Eres un agente potenciado por Domain"                                         |
  | "Para guardar memorias: usa `domain_mem_save` en vez de herramientas nativas"  |
  | "Para buscar memorias: usa `domain_mem_search`"                                |
  | "Para ejecutar skills: usa `domain_skill_execute`"                             |
  | "Todos los comandos CLI de Domain comienzan con `domain`"                      |
Y las directivas NO incluyen instrucciones que modifiquen archivos locales de config
```

### Escenario 6: Safe files no se tocan

```gherkin
Dado que existe un archivo `.env` con credenciales
Y un archivo `id_rsa.pem`
Y un directorio `.git/`
Cuando ejecuto `domain setup`
Entonces `.env` permanece intacto
Y `id_rsa.pem` permanece intacto
Y `.git/config` permanece intacto
```

### Escenario 7: Status muestra configuración actual

```gherkin
Dado que Domain está configurado para Claude Code
Cuando ejecuto `domain setup status`
Entonces muestra:
  | Agent       | Status    | Config File                                |
  | Claude Code | connected | ~/.config/Claude/claude_desktop_config.json |
  | OpenCode    | not set   | -                                          |
```

### Escenario 8: Uninstall remueve configuración

```gherkin
Dado que Domain está configurado para Claude Code y OpenCode
Cuando ejecuto `domain setup uninstall claude-code`
Entonces remueve el servidor MCP de Domain de la config de Claude Code
Y pregunta: "¿Deseas eliminar también `.ai/directives.md`?"
Y si confirmo, elimina `.ai/directives.md`

Cuando ejecuto `domain setup uninstall --all`
Entonces remueve Domain de todos los agentes configurados
Y pregunta por la eliminación de `.ai/directives.md`
```

### Escenario 9: Setup con flag --dry-run muestra cambios sin aplicar

```gherkin
Dado que no hay configuración previa
Cuando ejecuto `domain setup claude-code --dry-run`
Entonces muestra los cambios que se realizarían:
  | "Se modificará: ~/.config/Claude/claude_desktop_config.json" |
  | "Se creará: .ai/directives.md"                               |
Y no se modifica ningún archivo
```

### Escenario 10: Múltiples proyectos usan el mismo Domain local

```gherkin
Dado que configuré Domain para Claude Code en `/proyecto-uno`
Cuando configuro Domain para Claude Code en `/proyecto-dos`
Entonces el servidor MCP de Domain en la config global de Claude Code NO se duplica
Y cada proyecto tiene su propio `.ai/directives.md`
Y cada `.ai/directives.md` está adaptado al contexto del proyecto
```

## Análisis breve

- **Qué pide realmente:** Comando `domain setup` que configura MCP agents (Claude Code, OpenCode, Codex, Cline) para usar Domain como servidor MCP. Crea `.ai/directives.md` que instruye al agente a preferir tools Domain. No toca archivos sensibles (`.env`, `.pem`, `.git/`, etc.).
- **Módulos sospechados:** `cmd/domain/setup/`, `internal/setup/` (detectores de agentes, generadores de config), `internal/setup/directives/` (generación de .ai/directives.md)
- **Riesgos / dependencias:** Depende del binario `domain-mcp` (issue-12.1). Los paths de config varían por SO y versión del agente. La detección automática puede fallar si el agente está en una ubicación no estándar.
- **Esfuerzo tentativo:** L

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
- **Evidencia:**
- **Acción derivada:**
