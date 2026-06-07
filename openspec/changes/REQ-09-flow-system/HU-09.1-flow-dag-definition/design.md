# Design: HU-09.1-flow-dag-definition

## Decisión arquitectónica

| Decisión | Opción elegida | Alternativas |
|----------|---------------|--------------|
| Validación de DAG | Kahn's algorithm (BFS) | DFS + color marking (elegimos Kahn por mejor DX: da el orden topológico directamente) |
| Formato de steps | JSONB en Postgres | Tabla separada `flow_steps` (JSONB evita JOINs y permite versionado atómico) |
| Versionado | Optimistic locking con columna version | Tabla de versiones históricas (elegimos simple por MVP, histórico va en HU futura) |
| Serialización | `gopkg.in/yaml.v3` + `encoding/json` | `libyaml` bindings (yaml.v3 es puro Go, suficiente para este uso) |
| Slug | `github.com/gosimple/slug` | Manual (evitamos reinventar) |

## Alternativas descartadas

- **Tabla separada `flow_steps`**: Descartada porque versionar el flow requiere snapshot atómico del DAG completo. JSONB permite leer/escribir todo el grafo en una operación. Si en futuro se necesita query sobre steps individuales, se puede normalizar.
- **Validación con DFS recursivo**: Descartado por riesgo de stack overflow en DAGs profundos. Kahn's es iterativo y O(V+E).

## Diagrama

```
┌─────────────────────────────────────────────────────────┐
│                        Flow                             │
├─────────────────────────────────────────────────────────┤
│ id: UUID (PK)                                           │
│ name: VARCHAR(255) NOT NULL                             │
│ slug: VARCHAR(255) NOT NULL                             │
│ description: TEXT                                       │
│ project_id: UUID (FK → projects.id)                     │
│ steps: JSONB NOT NULL                                   │
│ version: INT NOT NULL DEFAULT 1                         │
│ created_at: TIMESTAMPTZ NOT NULL                        │
│ updated_at: TIMESTAMPTZ NOT NULL                        │
├─────────────────────────────────────────────────────────┤
│ UNIQUE(project_id, slug)                                │
│ INDEX(project_id)                                       │
└─────────────────────────────────────────────────────────┘

Step JSONB structure:
{
  "id": "s1",
  "type": "skill_call|llm_call|code_exec|conditional|parallel|wait|human_input|domain_agent_run|sub_flow|transform",
  "label": "Optional label for UI",
  "params": { ... },
  "depends_on": ["s0"],
  "timeout_seconds": 300
}
```

```
Flow de validación (Kahn's algorithm):
1. Calcular in-degree de cada nodo
2. Encolar nodos con in-degree 0
3. Mientras cola no vacía:
   a. Desencolar nodo, agregar a orden
   b. Decrementar in-degree de dependientes
   c. Si in-degree llega a 0, encolar
4. Si hay nodos no procesados → hay ciclo
5. Si no hay ciclo → orden topológico válido
```

## TDD plan

1. **Red:** Test `TestValidateDAG_NoCycle` espera `nil` error
2. **Green:** Implementar Kahn's algorithm
3. **Red:** Test `TestValidateDAG_Cycle` espera `ErrCycleDetected`
4. **Green:** Detectar nodos faltantes en orden topológico
5. **Red:** Test `TestCreateFlow_MissingFields` espera 422
6. **Green:** Validar campos requeridos en cada step
7. **Red:** Test `TestExportImportRoundtrip` espera contenido idéntico
8. **Green:** Serializar/deserializar manteniendo fidelidad
9. **Sabotaje:** Comentar validación de ciclo → test `TestValidateDAG_Cycle` falla

## Riesgos y mitigación

| Riesgo | Probabilidad | Impacto | Mitigación |
|--------|-------------|---------|------------|
| DAG cycle no detectado | Baja | Alto | Test específico + Kahn's probado |
| JSONB corrupto en DB | Baja | Medio | Validación al leer + migration repair tool |
| Race condition en versionado | Media | Medio | Optimistic locking + retry del lado cliente |
| YAML/JSON malicioso | Baja | Alto | Límite de 1MB + schema validation estricta |
