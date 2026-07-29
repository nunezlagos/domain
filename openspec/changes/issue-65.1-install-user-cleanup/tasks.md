# issue-65.1 — Tasks

## Implementación

- [ ] G1: reescribir configureContinue (clients.go) — merge por url en vez de reemplazar array
- [ ] G1: backupIfExists antes del write en uninstallClient rama continue (clients.go:150)
- [ ] G1: backupIfExists antes de desactivar engram en --remove-engram (engram.go:52)
- [ ] G2: templates — domain_orchestrate_status -> domain_flow_status (claude-global.md, opencode-global.md)
- [ ] G2: templates — agregar paso _preview al issue workflow en la sección Issues vs tickets
- [ ] G2: templates — marcar en la tabla Tool paths qué path aplica a Claude Code vs OpenCode
- [ ] G3: rm install-user/templates/personality.md
- [ ] G3: templates — quitar advertencia 'NO corras domain-code-graph.sh' (mantener nota domain_code_* deprecado)
- [ ] G3: claude_hook.go:21 — actualizar comentario stale (quitar 'code graph')
- [ ] G3: bootstrap.sh:136 — agregar -m 120 al curl de descarga de Go

## Tests

- [ ] G1: continue con otros servers los conserva (red->green)
- [ ] G1: re-instalar no duplica el server de domain
- [ ] G1: uninstall continue respalda antes de escribir
- [ ] G1: remove-engram respalda settings.json
- [ ] Sabotaje: volver configureContinue a reemplazo -> test de "conserva otros" falla
- [ ] Doc/ruido: grep de domain_orchestrate_status=0, domain-code-graph.sh=0, personality.md ausente, -m en bootstrap.sh

## Documentación

- [ ] Actualizar CHANGELOG Unreleased
- [ ] Actualizar state.yaml a implemented al cerrar
