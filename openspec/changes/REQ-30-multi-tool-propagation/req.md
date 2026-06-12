# REQ-30 — Multi-Tool Propagation (auto-setup al abrir proyecto)

> **Origen**: sesión 2026-06-12. Observación crítica del usuario: instaló
> domain DESPUÉS de tener proyectos con configuración de Claude Code
> (`.mcp.json`) o opencode (`.opencode/`), y esos proyectos no quedaron
> sincronizados con domain. El instalador solo toca el config global de
> opencode y el cwd del repo de domain — no propaga a proyectos
> preexistentes.

## Contexto

Caso real reproducido: `~/Proyectos/quien-sabe-de-web/` tenía
`.mcp.json` (Claude Code) con solo `opsx`. Cuando el usuario abrió ese
proyecto con opencode, el agente vio el global de opencode (que sí tiene
domain) pero como el proyecto no tiene `AGENTS.md` propio ni protocolo
específico, el LLM nunca llamó tools `domain_*` — trabajó "ciego del
contexto del proyecto" durante 20+ turnos sin guardar memoria.

La solución de fondo es **auto-setup al abrir el proyecto**: que domain
detecte qué herramienta de IA usa el proyecto y prepare el contexto
mínimo para que las tools `domain_*` sean usadas proactivamente, sin
intervención manual del usuario.

## Issues

| Issue | Slug | Esfuerzo | Descripción |
|-------|------|----------|-------------|
| 30.1 | `setup-auto-detect-command` | M | Comando `domain setup auto-detect <path>` que escanea: `.claude/`, `.opencode/`, `.cursor/`, `.mcp.json`, `AGENTS.md`, `CLAUDE.md`. Si tiene CLAUDE.md y no AGENTS.md → symlink. Si no tiene opencode.json local pero necesita instructions reforzadas → genera uno mínimo. Idempotente. Manifest de cambios en `<path>/.domain/install-manifest.json`. |
| 30.2 | `opencode-shell-wrapper-optin` | S | Función `opencode()` que envuelve el binario real: corre `domain setup auto-detect "$PWD" --quiet` antes de lanzar. Install ofrece agregarlo al `.zshrc`/`.bashrc` (con confirm explícito). Snippet copiable si el usuario prefiere agregarlo manual. |
| 30.3 | `claude-code-sessionstart-hook` | S | Configura SessionStart hook en `~/.claude/settings.json` que ejecuta `domain setup auto-detect "$PWD"` al iniciar sesión Claude Code. Install muestra diff antes de tocar settings.json. Compatible con hooks existentes (merge, no replace). |
| 30.4 | `install-manifest-and-uninstall` | M | Manifest global `~/.config/domain/install-manifest.json` con: archivos tocados, hash original, hash nuevo, timestamp, qué REQ los tocó. Comando `domain status --installed` muestra qué tocó. Comando `domain uninstall` revierte TODO limpio (restaura backups del manifest). |
| 30.5 | `propagate-existing-projects-scan` | S | Comando `domain setup propagate` que escanea `~/Proyectos/` (path configurable) y reporta qué proyectos tienen config de IA SIN domain. Pregunta interactiva: "encontré 5 proyectos, ¿agregar domain a cuáles?". Lista con checkboxes (o flag `--all`). |

## Prioridad: **alta** (entrega esta semana)

Resuelve el problema del usuario detectado HOY: "instalé domain pero en
mis proyectos viejos no funciona". Sin esto, cada proyecto preexistente
requiere intervención manual.
