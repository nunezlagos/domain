# Como usuario del SDD, el flujo me consulta de forma interactiva en los puntos clave (spec, design, elección de modo), mantiene openspec/ sincronizado con la BD por contrato, detecta policies/skills de mis .md al bootstrap con confirmación, y me deja auditar qué inyectan los hooks — cerrando los gaps de human-in-the-loop detectados en producción.

## Why
Feedback del usuario tras usar REQ-54 en prod. Seis gaps reales verificados por
auditoría de código: (1) el spec pregunta en prosa, no interactivo; (2) no se
elige exec_mode al inicio (default silencioso auto); (3) design no avisa; (4) el
sync openspec vive solo en el agent-protocol global (frágil); (5) auto-skill/
policy no ocurre solo (el usuario "no lo visualiza"); (6) las inyecciones de
hooks son invisibles.

## Límites de plataforma (honestos)
- AskUserQuestion NO se puede forzar 100% desde el server MCP, y NO existe en
  subagentes → spec/design nunca en subagente; interactividad reforzada por
  prompt, no garantizada.
- additionalContext de hooks es invisible en la UI de Claude Code sin log
  nativo → solo mitigable con log a archivo.

## Scope (6 issues)
- 55.1 interactive-spec-design: AskUserQuestion en templates spec/design + no-subagente.
- 55.2 exec-mode-choice: preguntar auto vs human-in-the-loop al iniciar + hardspec.
- 55.3 openspec-sync-in-phases: sync en prompts de propose/design/tasks + required_tool_calls.
- 55.4 auto-skill-policy-bootstrap: bootstrap devuelve candidatos + gate de confirmación con scope.
- 55.5 hook-injection-logging: hooks loguean additionalContext a archivo.

## Testing
Cada issue con su TDD. Los server-side (55.2/55.3/55.4) con tests Go; los de
hooks (55.5) con stdin simulado; interactividad (55.1) verificable por prompt.
