# Como orquestador SDD, al iniciar cada fase preparo server-side el contexto read-only barato (auto-policies, auto-skills, mem_context, code graph) y se lo entrego al cliente ya armado, de modo que las tools de consulta dejen de ser huérfanas y el cliente reciba la fuente de verdad sin tener que acordarse de pedirla.

## Why

Aún con el contrato de issue-54.1, un grupo grande de tools read-only (listar policies,
traer skills relevantes, mem_context, overview del code graph) son de PREPARACIÓN, no de
trabajo creativo. Obligar al cliente a llamarlas una por una es fricción y sigue siendo
frágil. Como son baratas y no requieren el LLM caro del cliente, el servidor puede correrlas
y entregar el contexto listo. Esto materializa la visión "el MCP provee la fuente de verdad,
el cliente recibe el contexto ya armado".

## Scope

- Un paso de preparación por fase que corre server-side las tools read-only pertinentes y
  agrega el resultado a `PriorOutputs` / al prompt de la fase.
- Uso de **Minimax** (LLM barato ya disponible en la plataforma) SOLO para la parte
  inteligente-barata de la preparación: filtrar qué policies/skills aplican a esta fase,
  resumir mem_context. Nunca para ejecutar la fase.
- Reusar el motor DAG (`flowrunner`) donde encaje, en vez de un camino nuevo.

Fuera de alcance: ejecución de fases creativas server-side (explícitamente NO se hace),
async worker (issue-54.3).

## Approach

Capa de "context prep" que se ejecuta antes de entregar el prompt de la fase al cliente.
Para fases spec/exec, la preparación es enriquecimiento (auto-policies/skills). El LLM del
cliente sigue haciendo todo el trabajo creativo. División por costo/criticidad: Minimax
prepara y filtra; el cliente crea.

## Risks

- Latencia: si la preparación se mete en el camino síncrono del cliente, lo hace esperar →
  la preparación debe ser rápida (tools read-only + Minimax con timeout corto) y, si tarda,
  degradar a entregar el contexto crudo sin el filtrado de Minimax.
- Dependencia de Minimax en el server → requiere el fix de env del issue-54.3 (la key hoy
  no llega al proceso Go). Sin Minimax, la preparación degrada a listados crudos (sigue
  siendo útil, solo sin filtrado inteligente).

## Testing

Preparación produce el contexto esperado por fase; degradación elegante si Minimax no está;
la latencia de preparación queda bajo un umbral. Aserciones sobre el `PriorOutputs`
resultante.
