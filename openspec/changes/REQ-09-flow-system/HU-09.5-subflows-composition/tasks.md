# Tasks: HU-09.5-subflows-composition

## Backend

- [ ] Implementar `SubFlowRunner` que crea FlowRun hijo con parent_run_id
- [ ] Agregar campo `parent_run_id` al modelo FlowRun
- [ ] Agregar campo `ancestor_slugs` (TEXT[]) al modelo FlowRun
- [ ] Agregar campo `depth` (INT) al modelo FlowRun
- [ ] Crear migración SQL para columnas de sub-flow en flow_runs
- [ ] Implementar detección de circularidad (ancestors set con slugs)
- [ ] Implementar límite de profundidad (máximo 5 niveles)
- [ ] Implementar pasaje de contexto padre → hijo (template resolution del input)
- [ ] Implementar retorno de output del sub-flow → contexto padre (último step output)
- [ ] Integrar SubFlowRunner en flow runner (HU-09.3)
- [ ] Implementar endpoint GET /api/v1/flows/:slug/parents
- [ ] Agregar validación estática de circularidad en guardado de flow (opcional, optimización)
- [ ] Limitar tamaño de output de sub-flow (1MB)

## Tests

- [ ] Test unitario: SubFlowRunner ejecuta flow hijo correctamente
- [ ] Test unitario: parent_run_id se asigna correctamente
- [ ] Test unitario: detección de circularidad directa (A → A)
- [ ] Test unitario: detección de circularidad indirecta (A → B → A)
- [ ] Test unitario: límite de profundidad 5+1 falla
- [ ] Test unitario: context passing padre → hijo (input mapeado)
- [ ] Test unitario: context passing hijo → padre (output disponible)
- [ ] Test unitario: sub-flow fallido → step padre fallido
- [ ] Test de integración: flow con sub-flow ejecuta correctamente
- [ ] Test de integración: parallel con 2 sub-flows
- [ ] Test de integración: 3 niveles de anidamiento
- [ ] Test unitario: GET parents retorna flows correctos
- [ ] Sabotaje: quitar detección de circularidad → test de ciclo falla

## Cierre

- [ ] Verificación manual: crear flow A que usa flow B como sub-flow
- [ ] Verificación manual: ver parents de flow B
- [ ] Suite verde
