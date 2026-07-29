# issue-54.3 async worker + fixes — Spec

Como plataforma domain, arreglo los cuatro puntos que hoy aparentan funcionar y no
funcionan, para que el estado del sistema sea honesto y el trabajo desatendido con Minimax
sea posible.

## ADDED Requirements

### Requirement: worker de flows async
El sistema DEBE ejecutar los flow_runs creados en modo async hasta un estado terminal
(`completed`/`failed`), mediante un worker de fondo, sin dejarlos en `pending`
indefinidamente.

#### Scenario: un flow async se procesa hasta el final
- **GIVEN** un flow_run creado con modo async, en estado `pending`
- **AND** el servidor con LLM disponible
- **WHEN** el worker async corre
- **THEN** el flow_run alcanza `completed` o `failed`
- **AND** no queda en `pending`

#### Scenario: sin LLM el worker no arranca pero el server sigue sano
- **GIVEN** el servidor sin LLM disponible
- **WHEN** arranca
- **THEN** el worker async no se inicia
- **AND** el servidor arranca normalmente y loguea el motivo

#### Scenario: el worker no toca flows interactivos
- **GIVEN** un flow_run interactivo (client-driven, no async) en `pending`
- **WHEN** el worker async corre
- **THEN** ese flow_run no es tomado por el worker

### Requirement: LLM disponible en el proceso Go
El proceso del servidor Go (domain-mcp) DEBE recibir la configuración del LLM
(`LLM_PROVIDER`/`LLM_API_KEY`/`LLM_MODEL`) desde su entorno de deploy.

#### Scenario: el health reporta el LLM activo
- **GIVEN** el container domain-mcp con las variables LLM en su environment
- **WHEN** se consulta `domain_health`
- **THEN** reporta el LLM (Minimax) como disponible

### Requirement: materialización openspec consistente entre binarios
El armado del orquestador DEBE incluir Spec/Tasks/IssueSvc en el binario que se despliega,
de modo que `persistOpenspec` no sea un no-op silencioso.

#### Scenario: persistOpenspec materializa en el binario desplegado
- **GIVEN** el orquestador construido por la función de armado compartida
- **WHEN** se completa una fase propose/design/tasks
- **THEN** los outputs se materializan en las tablas openspec correspondientes

### Requirement: sin código de orquestación muerto
El sistema NO DEBE conservar paquetes de orquestación sin call-sites salvo con un plan de
uso documentado.

#### Scenario: agent/orchestration sin uso se elimina o se documenta
- **GIVEN** el paquete `agent/orchestration/*` sin call-sites en producción
- **WHEN** se revisa el código
- **THEN** el paquete está eliminado, o conservado con un plan de uso documentado
