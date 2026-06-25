package main

import _ "embed"




//go:embed templates/skill-domain/SKILL.md
var skillDomainMD []byte

//go:embed templates/agents/domain-memory.md
var agentDomainMemoryMD []byte
