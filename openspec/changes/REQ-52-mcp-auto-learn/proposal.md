# REQ-52 — Auto-mejora del MCP (4 HUs)

> El MCP tiene el 80% de la infra para auto-mejora pero le falta el
> 20% que es la pieza clave: el feedback loop. Esta REQ agrega ese
> loop y convierte al MCP en un sistema que aprende de su propio uso.

## Investigacion previa

### Lo que YA tenemos en el MCP (80% de la infra)

1. **LLM abstraction completa** (`internal/llm/`): anthropic, openai, google, ollama
2. **Embeddings** con pgvector (1536 dims) y busqueda hibrida BM25 + cosine + RRF
3. **Memory** (`internal/service/observation/`): CRUD + dedup hash SHA-256 + privacy scrubbing
4. **Knowledge** (`internal/service/knowledge/`): chunks indexados para contexto
5. **Codegraph** (`internal/service/codegraph/`): parsea codigo real (Python, Go, TS, JS, PHP)
6. **Skill execution tracking** (`skill_executions`): status, duration, output, error
7. **Skill versioning** (`internal/service/skill/versioning.go`): snapshots, pin, rollback
8. **Audit log** inmutable con field-level diffs
9. **Cron service** con `robfig/cron/v3` para jobs automaticos
10. **Circuit breaker + ratelimit + retry** en el cliente LLM
11. **CLI** para inspeccion (suggest.go, configman.go, policies.go)
12. **Policy stack** (global + project + skill)
13. **Seeds versionados** (SkillsCatalogSeeder v6)

### Lo que NO tenemos (20% que falta = 4 HUs)

1. **Feedback loop del usuario**: el operador nunca califica respuestas
2. **Skill success rate**: no se computa "exito vs fallo" por skill
3. **LLM-as-judge**: no hay un service que use LLM para auto-evaluar
4. **A/B testing de prompts**: no hay forma de comparar 2 versiones

## Decisiones de diseno

### 1. Feedback loop primero (porque sin signal no hay mejora)
Sin datos de feedback, no podemos saber que mejorar. La HU-52.1 es
el prerequisito de las demas: sin data, no hay ML.

### 2. Success tracking automatico (no requiere UI)
skill_executions ya existe. Solo agregamos una vista SQL materializada
que agrega invocaciones, success_count, failure_count, avg_duration_ms
por skill_id. Esta data es la que alimenta al LLM-as-judge.

### 3. LLM-as-judge con human-in-the-loop
El LLM sugiere, el humano aprueba. Mismo patron que HU-50.2:
- 4 tipos de sugerencias: split / merge / refine / archive
- Requieren aprobacion humana via UI
- Nunca se aplican automaticamente

### 4. A/B testing opt-in (no rompe nada)
El A/B testing es opt-in: solo los skills marcados como `ab_test=true`
participan. Para los demas, se sigue usando la version pin normal.

## Arquitectura (el feedback loop)

```
Operador (👍/👎) ---> Admin (boton) ---> POST /feedback ---> MCP
                                                          |
                                                          v
                                                    skill_feedback table
                                                          |
                                                          v
                                                    metrics_aggregator (cron diario)
                                                          |
                                                          v
                                                    meta_optimizer (cron semanal)
                                                          |
                                                          v
                                                    LLM-as-judge:
                                                      - Evaluar cada skill
                                                      - Generar sugerencias
                                                      - Persistir en skill_suggestions
                                                          |
                                                          v
                                                    UI en domain-admin:
                                                      - Lista de sugerencias
                                                      - Approve / Reject / Modify
                                                          |
                                                          v
                                                    Al aprobar:
                                                      - REFINE: actualizar skill
                                                      - SPLIT: crear 2 skills hijos
                                                      - MERGE: consolidar 2 skills
                                                      - ARCHIVE: soft delete
```

## Scope de cada HU

| HU | Alcance | Estimacion | Depende de |
|----|---------|------------|------------|
| 52.1 User feedback loop | Thumbs up/down en cada respuesta del chat, POST /feedback, tabla skill_feedback, cron de agregacion | 1-2 dias | - |
| 52.2 Skill success tracking | Vista materializada skill_metrics, computar success rate (status='completed' y output no vacio), cron diario | 1-2 dias | - |
| 52.3 LLM-as-judge | MetaOptimizer (cron semanal), 4 tipos de sugerencias, UI de approval, sin auto-apply | 3-4 dias | 52.1, 52.2 |
| 52.4 A/B testing de prompts | ab_test boolean en skills, traffic split 50/50, metric por variant, elegir ganador | 2-3 dias | 52.2 |

**Total: 7-11 dias** distribuidos incrementalmente.

## Orden de implementacion

1. **HU-52.1** (feedback loop) → recolecta signal
2. **HU-52.2** (success tracking) → automatico, no requiere UI
3. **HU-52.3** (LLM-as-judge) → usa data de 52.1 + 52.2
4. **HU-52.4** (A/B testing) → opcional, despues de 52.3

## Metricas de exito

| HU | Metrica | Target |
|----|---------|--------|
| 52.1 | Tasa de feedback (% de mensajes con 👍/👎) | > 30% |
| 52.2 | Skills con metricas computadas | 100% |
| 52.3 | Sugerencias aprobadas vs rechazadas | > 50% aprobadas |
| 52.3 | Mejora en success rate despues de aplicar sugerencias | +5% |
| 52.4 | Variants con al menos 100 invocaciones cada una | 100% de los ab_tests |

## Out of scope

- **No** se hace online learning del LLM (no fine-tuning)
- **No** se hace reinforcement learning from human feedback (RLHF) real
- **No** se hace A/B testing de las respuestas del chat (solo de skills)
- **No** se envia feedback automatico al provider de LLM (no se hace training de sus modelos)

## Riesgos

| Riesgo | Mitigacion |
|--------|------------|
| Operador no da feedback | UX: botones visibles despues de cada respuesta, prompt sutil |
| Success rate mal definido | "exito" = status='completed' AND output no vacio AND execution_time_ms < 60s |
| LLM sugiere splits absurdos | Toda sugerencia requiere aprobacion humana |
| A/B test con trafico desigual | hash(skill_id + user_id) % 2 == 0 para determinismo |
| Sesgo de auto-mejora (solo mejora lo que ya es bueno) | Trackear distribution de feedback por skill_id, alertar si sesgo |