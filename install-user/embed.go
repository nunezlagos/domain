package main

import _ "embed"

//go:embed templates/skill-domain/SKILL.md
var skillDomainMD []byte

//go:embed templates/agents/domain-memory.md
var agentDomainMemoryMD []byte

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
