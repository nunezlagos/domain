# issue-54.7 sdd-code-gate — Spec

## ADDED Requirements

### Requirement: ninguna edición de código sin flow SDD
El gate PreToolUse DEBE interceptar Edit/Write/NotebookEdit (y Bash que parezca
edición de código) cuando la sesión no tiene flow SDD activo.

#### Scenario: edición sin flow en modo normal pregunta al humano
- GIVEN una sesión sin flow SDD activo, permission_mode default
- WHEN el agente intenta Edit
- THEN el diálogo de permisos pregunta al usuario (permissionDecision=ask)

#### Scenario: edición sin flow en modo automático se deniega
- GIVEN una sesión sin flow activo, permission_mode acceptEdits/bypass/auto
- WHEN el agente intenta Write
- THEN la edición se DENIEGA con razón "ejecutá domain_orchestrate"

#### Scenario: con flow activo todo pasa
- GIVEN el agente llamó domain_orchestrate (PostToolUse marcó la sesión)
- WHEN intenta cualquier edición
- THEN pasa sin fricción

#### Scenario: Bash de edición se gatea, Bash inocuo no
- GIVEN sesión sin flow
- WHEN corre `sed -i 's/a/b/' main.go` → gateado
- AND corre `go test ./... > /tmp/log.txt` → permitido

### Requirement: consulta obligatoria antes del spec
La fase sdd-spec DEBE instruir consultar al usuario (AskUserQuestion) ante
ambigüedades ANTES de redactar el spec.

#### Scenario: spec con dudas
- GIVEN el agente entra a sdd-spec con decisiones abiertas
- WHEN construye el prompt de la fase
- THEN el prompt exige preguntar y esperar respuesta antes de redactar

### Requirement: worktrees no crean proyectos fantasma
Los hooks DEBEN resolver el project_slug vía el repo principal
(git-common-dir), no por basename del cwd.

#### Scenario: captura desde un worktree
- GIVEN una sesión corriendo en <repo>/.claude/worktrees/feature-x
- WHEN el hook captura un prompt
- THEN el project_slug es el del repo principal, no "feature-x"
