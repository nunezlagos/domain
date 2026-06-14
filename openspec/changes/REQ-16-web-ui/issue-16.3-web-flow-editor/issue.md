# issue-16.3-web-flow-editor

**Origen:** `REQ-16-web-ui`
**Prioridad tentativa:** media
**Tipo:** feature

## Historia de usuario
**Como** ingeniero de IA
**Quiero** un editor visual de flows con drag-and-drop para definir pasos, configurarlos y validar el DAG
**Para** crear pipelines de IA complejos sin escribir código YAML manualmente

## Criterios de aceptación

```gherkin
Feature: Web Flow Editor

  Background:
    Given el usuario está autenticado en la web UI
    And navega a /flows/new

  Scenario: Editor visual con drag-and-drop
    When veo el flow editor
    Then veo un canvas vacío
    And una paleta de pasos disponibles:
      | paso         | descripción                        |
      | LLM Call     | llamada a un modelo LLM           |
      | Tool         | ejecución de una herramienta       |
      | Condition    | branch condicional (if/else)       |
      | Subflow      | ejecutar otro flow                 |
      | Code         | ejecutar código custom             |
      | Input        | entrada del usuario                |
      | Output       | salida del flow                    |
    And puedo arrastrar pasos al canvas

  Scenario: Conectar pasos con flechas
    When arrastro desde el puerto de salida de un paso al puerto de entrada de otro
    Then se crea una conexión (edge) entre los pasos
    And la flecha apunta en dirección del flujo

  Scenario: Configurar un paso
    When hago doble click en un paso LLM Call
    Then se abre un panel de configuración con:
      | campo          | tipo                |
      | Model          | dropdown de modelos |
      | System Prompt  | textarea            |
      | User Prompt    | textarea            |
      | Temperature    | slider (0-2)        |
      | Max Tokens     | number input        |
    And puedo guardar la configuración

  Scenario: Configurar paso Condition
    When configuro un paso Condition
    Then puedo definir: variable, operador (==, !=, >, <), valor
    And dos ramas: true y false

  Scenario: DAG validation
    When el DAG tiene ciclos
    Then el editor muestra error: "Flow contains cycles"
    And no permite guardar

  Scenario: DAG validation exitoso
    Given el DAG es válido (acíclico, todos los pasos conectados)
    When hago click en "Validate"
    Then muestra "Flow is valid"
    And resalta pasos desconectados si los hay

  Scenario: Importar YAML/JSON
    When hago click en "Import" y selecciono un archivo YAML
    Then el flow se carga en el editor
    And los pasos y conexiones se renderizan en el canvas
    And si hay errores de formato, muestra mensaje explicativo

  Scenario: Exportar YAML/JSON
    When hago click en "Export"
    Then descargo el flow como archivo YAML o JSON
    And el formato es compatible con flow definition (REQ-09)

  Scenario: Test run desde editor
    When hago click en "Run Test"
    Then el flow se ejecuta con input de prueba
    And veo los resultados en un panel lateral
    And cada paso muestra su output

  Scenario: Version history
    When hago click en "History"
    Then veo la lista de versiones del flow
    And cada versión tiene: número, fecha, autor
    And puedo previsualizar y restaurar versiones anteriores

  Scenario: Guardar flow
    When hago click en "Save"
    Then el flow se guarda en la base de datos
    And se crea una nueva versión

  Scenario: Editar flow existente
    When navego a /flows/{id}/edit
    Then el flow se carga en el editor con todos los pasos y conexiones
```

## Análisis breve

- **Qué pide realmente:** Flow editor visual con React Flow (drag-and-drop, conexiones). Panel de configuración de pasos. Validación de DAG (ciclos, conectividad). Import/Export YAML/JSON. Test run. Version history.
- **Módulos sospechados:** `web/` (frontend), `internal/api/handlers/flows.go`
- **Riesgos / dependencias:** Depende de REQ-09 (flow system). React Flow expertise. Complejidad alta en UX.
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
