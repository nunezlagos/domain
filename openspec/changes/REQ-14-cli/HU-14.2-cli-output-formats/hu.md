# HU-14.2-cli-output-formats

**Origen:** `REQ-14-cli`
**Persona:** dx-engineer
**Prioridad tentativa:** media
**Tipo:** feature

## Historia de usuario
**Como** usuario de la CLI
**Quiero** elegir entre múltiples formatos de salida (tabla, json, yaml) y que la salida se adapte al contexto (pipe vs TTY)
**Para** consumir los resultados de la forma más conveniente: legible en pantalla o estructurada para pipelines

## Criterios de aceptación

```gherkin
Feature: CLI Output Formats

  Background:
    Given el binario `domain` está instalado
    And existen 3 observaciones en el sistema

  Scenario: Output por defecto en TTY es tabla formateada
    Given la salida es un terminal (TYY)
    When ejecuto `domain memory list`
    Then la salida es una tabla con bordes y colores
    And las columnas son: ID, Title, Type, Created At

  Scenario: Output JSON con --output json
    When ejecuto `domain memory list --output json`
    Then la salida es un array JSON válido
    And cada objeto tiene los campos de la entidad

  Scenario: Output YAML con --output yaml
    When ejecuto `domain memory list --output yaml`
    Then la salida es YAML válido
    And cada documento tiene los campos de la entidad

  Scenario: Pipe detectado automáticamente cambia default a JSON
    Given la salida es un pipe (no TTY)
    When ejecuto `domain memory list`
    Then el output por defecto es JSON (no tabla)
    And es válido para piping a jq u otras herramientas

  Scenario: Pipe con --output explícito respeta el flag
    Given la salida es un pipe
    When ejecuto `domain memory list --output table`
    Then la salida es tabla (fuerza formato aunque esté pipeando)

  Scenario: TTY con --output json no tiene colores
    When ejecuto `domain memory list --output json`
    Then la salida es JSON sin colores ANSI

  Scenario: Colores en tabla para TTY
    When ejecuto `domain memory list` en TTY
    Then los encabezados están en negrita
    And los valores tienen color según el tipo (fix=red, context=blue)
    And las celdas están alineadas correctamente

  Scenario: Progreso para operaciones largas
    When ejecuto `domain agent run my-agent --prompt "..."` en TTY
    Then se muestra un spinner o barra de progreso
    And el progreso se oculta cuando el comando termina

  Scenario: Sin progreso en pipe
    Given la salida es un pipe
    When ejecuto `domain agent run my-agent --prompt "..."`
    Then no se muestra spinner
    And solo se imprime el resultado final en JSON

  Scenario: Error en formato inválido
    When ejecuto `domain memory list --output xml`
    Then muestra error "invalid output format: xml"
    And lista formatos válidos: table, json, yaml

  Scenario: Output de una sola entidad en tabla
    When ejecuto `domain memory get abc-123`
    Then muestra los campos como pares key: value en formato legible
```

## Análisis breve

- **Qué pide realmente:** Output formatter interface con 3 implementaciones (table, json, yaml). Detección de pipe vs TTY. Colorized output con tablewriter. Spinner para operaciones largas.
- **Módulos sospechados:** `internal/cli/output/`, `internal/cli/formatter.go`
- **Riesgos / dependencias:** Depende de HU-14.1 (los commands que producen output). Librerías de table formatting y color.
- **Esfuerzo tentativo:** M

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
