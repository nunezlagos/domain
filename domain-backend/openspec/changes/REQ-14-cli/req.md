# REQ-14-cli: Interfaz de línea de comandos: todos los comandos del plataforma, output pipe-friendly, JSON output, autocompletado.

**Estado:** activo
**Creado:** 2026-06-07
**Fase:** F2

## Descripción

Interfaz de línea de comandos: todos los comandos del plataforma, output pipe-friendly, JSON output, autocompletado.

## Criterios de éxito

- CLI completa con todos los comandos del sistema
- Output pipe-friendly con soporte JSON
- Autocompletado para bash, zsh, fish

## HUs hijas

| HU | Estado | Descripción |
|----|--------|-------------|
| issue-14.1-cli-core-commands | proposed | Comandos core: memory, skill, agent, flow, cron, config, project, setup |
| issue-14.2-cli-output-formats | proposed | Output formats: table, JSON, YAML, silent, colores |
| issue-14.3-cli-autocomplete-help | proposed | Autocompletado bash/zsh/fish, man pages, --help detallado |
