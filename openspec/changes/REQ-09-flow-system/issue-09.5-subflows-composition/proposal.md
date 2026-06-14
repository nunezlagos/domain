# Proposal: issue-09.5-subflows-composition

## Intención

Permitir que un flow ejecute otro flow como un paso, con paso de contexto padre→hijo, detección de circularidad, ejecución paralela de sub-flows, y consulta de flujos padres. Los flows se convierten en templates reutilizables parametrizables vía `{{input.*}}`.

## Scope

**Incluye:**
- SubFlowRunner en step types (issue-09.2) que lanza un flow run interno
- Detección de referencias circulares (depth-first con set de slugs visitados)
- Pasaje de contexto: input del paso → contexto raíz del sub-flow
- Output del sub-flow → integrado al contexto del padre
- Ejecución paralela de sub-flows vía ParallelRunner
- Límite de profundidad de anidamiento (máximo 5 niveles)
- Endpoint GET /flows/:slug/parents para listar flows que referencian a otro

**Excluye:**
- Versionado de sub-flows (siempre usa la última versión del flow referenciado)
- Compartición de estado en tiempo real entre padre e hijo
- Debugging trans-flow (se puede hacer consultando ambos runs)

## Enfoque técnico

- SubFlowRunner: implementa StepRunner, llama al FlowRunner internamente creando un nuevo FlowRun con parent_run_id
- Para detección de circularidad: antes de ejecutar un sub-flow, verificar que su slug no esté en un set de "ancestors" que se pasa en el contexto de ejecución
- Límite de profundidad: contador en contexto de ejecución, si > 5, error
- Context mapping: el input del paso se resuelve con templates sobre el contexto del padre, el resultado es el contexto raíz del sub-flow
- Output mapping: el resultado del sub-flow (su output final) se guarda como resultado del paso en el contexto del padre
- Endpoint parents: escanea todos los flows buscando steps de tipo sub_flow que referencien al slug dado

## Riesgos

- Anidamiento excesivo: límite de 5 niveles + timeout global
- Circularidad: detección preventiva antes de ejecutar
- Performance: sub-flows anidados profundos pueden ser lentos; cada nivel agrega latencia
- Estado inconsistente: si padre e hijo se ejecutan en transacciones separadas, el padre podría ver estado incompleto del hijo

## Testing

- Unit: SubFlowRunner ejecuta flow correctamente
- Unit: detección de circularidad (directa e indirecta)
- Unit: límite de profundidad
- Unit: context passing padre→hijo→padre
- Integration: sub-flow execution completa
- Integration: parallel sub-flows
- Sabotaje: quitar detección de circularidad → test de ciclo falla
