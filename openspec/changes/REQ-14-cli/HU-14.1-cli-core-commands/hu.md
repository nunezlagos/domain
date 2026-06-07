# HU-14.1-cli-core-commands

**Origen:** `REQ-14-cli`
**Prioridad tentativa:** alta
**Tipo:** feature

## Historia de usuario
**Como** operador de memoria desde la terminal
**Quiero** comandos CLI para todas las entidades del sistema: memory, skill, agent, flow, cron, config
**Para** gestionar la plataforma sin depender de la interfaz web ni la API directa

## Criterios de aceptación

```gherkin
Feature: CLI Core Commands

  Background:
    Given el binario `domain` está instalado en el PATH
    And el cliente está configurado con API endpoint y API key válida

  Scenario: Comando sin argumentos muestra ayuda
    When ejecuto `domain`
    Then muestra la ayuda general con lista de comandos disponibles
    And el exit code es 0

  Scenario: Estructura de comandos entity action
    When ejecuto `domain memory save --title "test" --content "hello" --type context`
    Then la observación se guarda via API
    And se imprime el ID de la observación creada

  Scenario: Listar entidades
    Given existen 5 observaciones
    When ejecuto `domain memory list`
    Then muestra tabla con las observaciones
    And el exit code es 0

  Scenario: Obtener entidad por ID
    Given existe una observación con id "abc-123"
    When ejecuto `domain memory get abc-123`
    Then muestra los detalles de la observación
    And el exit code es 0

  Scenario: Eliminar entidad
    Given existe una observación con id "abc-123"
    When ejecuto `domain memory delete abc-123`
    Then confirma la eliminación
    And el exit code es 0

  Scenario: Comandos de skill
    When ejecuto `domain skill list`
    Then lista todos los skills disponibles
    And `domain skill run my-skill --input "data"` ejecuta el skill

  Scenario: Comandos de agent
    When ejecuto `domain agent list`
    Then lista los agentes
    And `domain agent run my-agent --prompt "hello"` ejecuta el agente

  Scenario: Comandos de flow
    When ejecuto `domain flow list`
    Then lista los flows
    And `domain flow execute my-flow` ejecuta el flow

  Scenario: Comandos de cron
    When ejecuto `domain cron list`
    Then lista los crons
    And `domain cron create --schedule "*/5 * * * *" --action "..."` crea un cron

  Scenario: Comandos de config
    When ejecuto `domain config set --key api_key --value sk-xxx`
    Then guarda la config localmente
    And `domain config get --key api_key` muestra el valor

  Scenario: Comando con --help muestra ayuda específica
    When ejecuto `domain memory --help`
    Then muestra ayuda específica del comando memory
    And lista subcomandos: save, list, get, delete, search

  Scenario: Error de conexión muestra mensaje claro
    Given el API endpoint no está disponible
    When ejecuto `domain memory list`
    Then muestra "Error: unable to connect to API at http://..."
    And exit code es 1

  Scenario: Flags globales
    When ejecuto `domain --api-endpoint http://custom:8080 memory list`
    Then usa el endpoint custom en lugar del default
    And `--output json` cambia el formato de salida
    And `--verbose` muestra información de debug
```

## Análisis breve

- **Qué pide realmente:** CLI con Cobra, commands jerárquicos, cliente HTTP hacia API REST, configuración local (config file), manejo de errores con mensajes claros
- **Módulos sospechados:** `cmd/domain/`, `internal/cli/`, `internal/client/`
- **Riesgos / dependencias:** Depende de API REST (REQ-13). Config file management. Cross-platform paths.
- **Esfuerzo tentativo:** XL

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
