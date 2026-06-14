# issue-14.3-cli-autocomplete-help

**Origen:** `REQ-14-cli`
**Prioridad tentativa:** baja
**Tipo:** feature

## Historia de usuario
**Como** usuario de la CLI
**Quiero** autocompletado de comandos en mi shell, ayuda detallada con ejemplos y sugerencias cuando me equivoco
**Para** ser más productivo y descubrir funcionalidades sin leer documentación externa

## Criterios de aceptación

```gherkin
Feature: CLI Autocomplete and Help

  Background:
    Given el binario `domain` está instalado

  Scenario: Autocompletado para bash
    When ejecuto `source <(domain completion bash)`
    Then el shell bash tiene autocompletado para Domain
    And al presionar Tab después de `domain ` sugiere: memory, skill, agent, flow, cron, config
    And al presionar Tab después de `domain memory ` sugiere: save, list, get, delete, search

  Scenario: Autocompletado para zsh
    When ejecuto `source <(domain completion zsh)`
    Then el shell zsh tiene autocompletado para Domain

  Scenario: Autocompletado para fish
    When ejecuto `domain completion fish | source`
    Then el shell fish tiene autocompletado

  Scenario: --help muestra ayuda detallada con ejemplos
    When ejecuto `domain memory save --help`
    Then muestra:
      And usage: domain memory save [flags]
      And description del comando
      And flags disponibles: --title, --content, --type
      And ejemplos de uso:
        domain memory save --title "Fix" --content "Done" --type fix
        domain memory save --title "Idea" --content "..."
      And aliases si existen

  Scenario: Error de tipeo sugiere comandos similares
    When ejecuto `domain memry list`
    Then muestra "Error: unknown command 'memry' for 'Domain'"
    And "Did you mean this?" → "memory"

  Scenario: Error de tipeo en subcomandos sugiere
    When ejecuto `domain memory sav`
    Then muestra "Did you mean?" → "save"

  Scenario: Error de flag desconocido sugiere
    When ejecuto `domain memory list --titel "test"`
    Then muestra "Did you mean?" → "--title"

  Scenario: Sin argumentos muestra ayuda del comando raíz
    When ejecuto `domain`
    Then muestra la ayuda con todos los comandos disponibles
    And flags globales: --help, --output, --api-endpoint, --verbose

  Scenario: --help en comando raíz
    When ejecuto `domain --help`
    Then muestra ayuda completa con uso, comandos, flags, ejemplos

  Scenario: Man page generada
    When ejecuto `domain man`
    Then genera página de manual en stdout
    And puede redirigirse a /usr/local/share/man/man1/domain.1

  Scenario: Versión del CLI
    When ejecuto `domain --version`
    Then muestra la versión del binario
    And el commit SHA del build

  Scenario: Verbose mode muestra debug info
    When ejecuto `domain --verbose memory list`
    Then muestra información de debug: config file usado, API endpoint, request duration
```

## Análisis breve

- **Qué pide realmente:** Cobra tiene soporte nativo para completion y help. La sugerencia de errores con Levenshtein distance. Man page generation.
- **Módulos sospechados:** `cmd/domain/` (completion subcommand), `internal/cli/help.go`
- **Riesgos / dependencias:** Depende de issue-14.1 (estructura de comandos). Shell completion requiere documentación de instalación.
- **Esfuerzo tentativo:** S

## Verificación previa

- [ ] Revisar codebase (grep)
- [ ] Revisar memorias engram (domain_mem_search)
- [ ] Revisar git log
- [ ] Probar en ambiente correcto
- [ ] Reproducir con perfil correcto
- [ ] Verificar caché / build
- [ ] Verificar feature flag / config

### Resultado de verificación

- **Estado:** pendiente
- **Evidencia:**
- **Acción derivada:**
