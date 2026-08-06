package main

import (
	"embed"
	_ "embed"
)

// agentsDir es la raíz del catálogo de agentes dentro del binario. DOMAINSERV-137: se
// embebe el DIRECTORIO en vez de cada archivo por nombre, así agregar un agente al
// catálogo es agregar un archivo y no editar el installer.
const agentsDir = "templates/agents"

// agents-hooks viaja en el MISMO FS que los agentes: un guard que no llegue al cliente deja
// abierto el tool que venía a acotar, así que no puede quedar como artefacto opcional.
//
//go:embed templates/agents templates/agents-hooks
var agentsFS embed.FS

// DOMAINSERV-239: los lifecycle hooks viajan en el binario. Antes los instalaba install-curl.sh
// y por eso el doctor solo podía comprobar que el archivo EXISTIERA — no tenía con qué comparar.
// Embebidos, el hash pasa a ser una verdad del binario y no del disco, así que un hook adulterado
// o divergente se detecta.
//
//go:embed hooks
var hooksFS embed.FS

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
