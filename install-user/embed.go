package main

import (
	"embed"
	_ "embed"
)

// agentsDir es la raíz del catálogo de agentes dentro del binario. DOMAINSERV-137: se
// embebe el DIRECTORIO en vez de cada archivo por nombre, así agregar un agente al
// catálogo es agregar un archivo y no editar el installer.
const agentsDir = "templates/agents"

//go:embed templates/agents
var agentsFS embed.FS

//go:embed templates/skill-domain/SKILL.md
var skillDomainMD []byte

// DOMAINSERV-135: un agente puede tener DOS templates porque los esquemas de frontmatter
// son incompatibles. OpenCode declara el modelo como provider/model-id y las
// restricciones con `permission:`; Claude Code usa el alias del modelo más
// `tools`/`disallowedTools`/`effort`. Un `model: haiku` pelado no es un campo desconocido
// que OpenCode pueda ignorar: es un valor malformado de un campo que sí conoce. La variante
// vive en <slug>.opencode.md y la empareja agentCatalog().

//go:embed templates/claude-global.md
var claudeGlobalMD []byte

//go:embed templates/claude-persona.md
var claudePersonaMD []byte

//go:embed templates/opencode-global.md
var opencodeGlobalMD []byte

//go:embed templates/opencode-git-guard.js
var opencodeGitGuardJS []byte

//go:embed templates/opencode-sdd-gate.js
var opencodeSddGateJS []byte
