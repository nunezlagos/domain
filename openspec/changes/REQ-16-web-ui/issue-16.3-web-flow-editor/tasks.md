# Tasks: issue-16.3-web-flow-editor

## Backend

- [ ] Implementar endpoint GET /api/v1/flows/{id} (full definition for editor)
- [ ] Implementar endpoint POST /api/v1/flows (create/update flow)
- [ ] Implementar endpoint POST /api/v1/flows/validate (server-side DAG validation)
- [ ] Implementar endpoint POST /api/v1/flows/import (recibir YAML/JSON, devolver flow)
- [ ] Implementar endpoint POST /api/v1/flows/{id}/test (ejecutar test run)
- [ ] Implementar endpoint GET /api/v1/flows/{id}/versions (version history)
- [ ] Implementar endpoint POST /api/v1/flows/{id}/versions/{v}/restore
- [ ] Implementar DAG validation: cycle detection, connectivity, required fields
- [ ] Implementar versionado: crear versión en cada save
- [ ] Implementar test run: ejecutar flow con input mock, no persistir

## Frontend

- [ ] Inicializar proyecto con React Flow + dagre
- [ ] Implementar FlowCanvas con React Flow (zoom, pan, grid background)
- [ ] Implementar StepPalette con 7 step types draggables
- [ ] Implementar CustomNode para cada step type (icon + label + status indicator)
- [ ] Implementar drag-and-drop desde palette al canvas
- [ ] Implementar edge creation al conectar nodos
- [ ] Implementar ConfigPanel con DynamicForm según step type
- [ ] Implementar formularios: LLM Call config
- [ ] Implementar formularios: Tool config
- [ ] Implementar formularios: Condition config (if/else branches)
- [ ] Implementar formularios: Subflow config
- [ ] Implementar formularios: Code config
- [ ] Implementar formularios: Input/Output config
- [ ] Implementar Toolbar: Save, Validate, Import, Export, Run Test, History
- [ ] Implementar Import: file picker → parse YAML/JSON → renderizar
- [ ] Implementar Export: flow → YAML/JSON → descargar
- [ ] Implementar Validate: llamar API + mostrar resultados en canvas
- [ ] Implementar TestRunPanel: ejecutar + mostrar resultados streaming
- [ ] Implementar VersionHistory: lista, preview diff, restore
- [ ] Implementar layout automático con dagre
- [ ] Implementar auto-save (cada 30s si hay cambios)
- [ ] Manejar empty state: flow nuevo sin pasos
- [ ] Manejar error state: flow inválido

## Tests

- [ ] Test unitario: DAG validation cycle detection
- [ ] Test unitario: DAG validation connectivity check
- [ ] Test unitario: flow definition ↔ editor nodes mapping
- [ ] Test unitario: import/export consistency
- [ ] Test de integración: crear flow → editar → guardar → recargar
- [ ] Test de integración: importar YAML → editar → exportar YAML consistente
- [ ] Test E2E: drag paso, conectar, configurar, guardar
- [ ] Test visual: flow editor layout
- [ ] Sabotaje: DAG con ciclo se guarda → validate lo rechaza

## Cierre

- [ ] Verificación manual: crear flow completo, guardar, recargar, editar
- [ ] Verificación manual: importar/exportar YAML
- [ ] Verificación manual: test run desde editor
- [ ] Suite backend: `go test ./internal/api/...`
- [ ] Suite frontend: `npm run test`
- [ ] Build: `npm run build` sin errores
