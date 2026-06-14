package main

import _ "embed"

// Templates de skill + subagent embebidos en el binario.
// Compartidos con install-user.sh (no se duplican).

//go:embed templates/skill-domain/SKILL.md
var skillDomainMD []byte

//go:embed templates/agents/domain-memory.md
var agentDomainMemoryMD []byte
