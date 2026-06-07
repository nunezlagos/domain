# Design: HU-12.3-mcp-agent-tools

## Decisión arquitectónica

```
                    ┌─────────────────────────────────────┐
                    │           MCPServer                  │
                    │                                     │
                    │  domain_skill_execute → SkillHandler       │
                    │  domain_skill_search  → SkillHandler       │
                    │  domain_agent_run     → AgentHandler       │
                    │  domain_agent_create  → AgentHandler       │
                    │  domain_flow_run      → FlowHandler        │
                    │  domain_flow_create   → FlowHandler        │
                    │  domain_flow_status   → FlowHandler        │
                    │  domain_cron_list     → CronHandler        │
                    │  domain_knowledge_search → KnowlHandler    │
                    └──────────┬──────────────────────────┘
                               │
         ┌─────────────────────┼─────────────────────┐
         │                     │                     │
  ┌──────▼──────┐    ┌────────▼───────┐    ┌────────▼──────┐
  │ SkillService │    │  AgentService  │    │  FlowService  │
  │              │    │                │    │               │
  │ Execute()    │    │ Run()          │    │ Run()         │
  │ Search()     │    │ Create()       │    │ Create()      │
  └──────────────┘    └────────────────┘    │ GetStatus()   │
                                            └───────────────┘
  ┌──────────────┐    ┌────────────────┐    ┌──────────────┐
  │ CronService  │    │KnowledgeService│    │  RunnerSvc   │
  │              │    │                │    │  (async)     │
  │ List()       │    │ Search()       │    │              │
  └──────────────┘    └────────────────┘    └──────────────┘
```

**Decisión:** Tools asincrónicas devuelven inmediatamente con `run_id`. El cliente usa `domain_flow_status` para polling. Cada tool handler es independiente y se registra por separado. Los handlers NO contienen lógica de negocio - solo orquestación y formato.

## Alternativas descartadas

| Alternativa | Motivo de descarte |
|---|---|
| Tools síncronas bloqueantes | Ejecuciones largas (minutos) timeout del lado del cliente MCP |
| Webhook callback en MCP | MCP no soporta callbacks nativamente |
| Tool única con type dispatch | Viola SRP, difícil de testear y mantener |
| gRPC streaming para resultados | MCP es el protocolo, no podemos cambiarlo |

## Diagrama

```
─── FLUJO ASINCRÓNICO ───

Cliente MCP                    MCPServer                 Service                 Runner
    │                            │                         │                       │
    │──── domain_skill_execute ────────►│                         │                       │
    │    {skill_id, params}      │── ValidateArgs          │                       │
    │                            │── EnqueueExecution ────►│                       │
    │                            │                         │── PublishTask ───────►│
    │◄─── {run_id: "sr_abc"} ───│                         │                       │
    │                            │                         │                       │
    │──── domain_flow_status ─────────►│                         │                       │
    │    {run_id: "sr_abc"}      │── GetStatus ───────────►│                       │
    │◄─── {status: "running",    │                         │                       │
    │      current_step: 1}     │                         │                       │
    │                            │                         │                       │
    │         ... tiempo ...     │                         │                       │
    │                            │                         │◄──── result ──────────│
    │                            │                         │                       │
    │──── domain_flow_status ─────────►│                         │                       │
    │    {run_id: "sr_abc"}      │── GetStatus ───────────►│                       │
    │◄─── {status: "success",    │                         │                       │
    │      result: {...}}       │                         │                       │
```

## TDD plan

1. **Red:** Test que `domain_skill_execute` valida skill_id requerido
2. **Green:** Implementar handler con validación
3. **Red:** Test que `domain_skill_execute` devuelve run_id inmediatamente
4. **Green:** Implementar llamada async a SkillService.Execute()
5. **Red:** Test que `domain_agent_create` valida model requerido
6. **Green:** Implementar AgentService.Create() con validación de modelo
7. **Red:** Test que `domain_flow_run` + `domain_flow_status` ciclo completo funciona
8. **Green:** Implementar ambos handlers con FlowService
9. **Red:** Test que `domain_cron_list` filtra por proyecto correctamente
10. **Green:** Implementar CronService.List() con filtro
11. **Red:** Test que `domain_knowledge_search` devuelve snippets con score
12. **Green:** Implementar KnowledgeService.Search() con embeddings
13. **Sabotaje:** No validar args → SQL injection potencial
14. **Sabotaje:** domain_flow_status sin run_id → panic

## Riesgos y mitigación

- **Long-running operations:** Cliente MCP timeout. Mitigación: respuesta inmediata con run_id, el cliente hace polling.
- **Rate limiting:** Agentes maliciosos spameando. Mitigación: rate limit por tool y por project, cola de ejecución.
- **Consistencia:** domain_flow_status polling puede leer estado desactualizado. Mitigación: consistencia eventual, TTL de caché mínimo.
- **Error reporting:** Errores en ejecución asincrónica. Mitigación: domain_flow_status devuelve error field cuando ocurre.
