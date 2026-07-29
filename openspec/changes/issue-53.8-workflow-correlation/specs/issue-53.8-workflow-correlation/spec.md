# Como operator del MCP, cada workflow (secuencia iniciada por un agent o usuario) deja un workflow_id uuid v7 propagado via ctx que correlaciona tool invocations + SQL queries + fn calls + HTTP requests en un arbol cronologico consultable via domain_workflow_trace(workflow_id).

