package main

import _ "embed"

//go:embed templates/skill-domain/SKILL.md
var skillDomainMD []byte

//go:embed templates/agents/domain-memory.md
var agentDomainMemoryMD []byte

// DOMAINSERV-135: dos templates para el MISMO agente porque los esquemas de frontmatter
// son incompatibles. OpenCode declara el modelo como provider/model-id y las
// restricciones con `permission:`; Claude Code usa el alias del modelo más
// `tools`/`disallowedTools`/`effort`. Un `model: haiku` pelado no es un campo desconocido
// que OpenCode pueda ignorar: es un valor malformado de un campo que sí conoce.
//
//go:embed templates/agents/domain-memory.opencode.md
var agentDomainMemoryOpencodeMD []byte

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
