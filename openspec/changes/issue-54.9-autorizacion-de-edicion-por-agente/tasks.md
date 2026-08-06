# Tasks — Autorización de edición por agente

DOMAINSERV-218, incremento 3. Los grupos se ejecutan en orden ascendente; dentro de un
grupo las tasks son independientes entre sí.

## schema

- [ ] **t1** *(grupo 1, 2 h)* — Crear `000289_flow_agent_scopes.up.sql` / `.down.sql` con la
  tabla `flow_agent_scopes` (flow_run_id, agent_id, project_id, allowed_paths, expires_at,
  revoked_at), UNIQUE `(flow_run_id, agent_id)`, índice sobre `flow_run_id` y RLS por
  `project_id` con el patrón de la `000288`. Done cuando `migrate up` corre sobre una base
  limpia sin error y el header trae migration/author/issue/description/breaking/duration.

## code

- [ ] **t2** *(grupo 2, 1 h)* — Agregar `AgentID string \`json:"a,omitempty"\`` a
  `FlowTokenPayload` y un constructor que lo acepte sin cambiar la firma de `GenerateToken`.
  Done cuando pasa `TestFlowToken_GenerateToken_SinAgente_PayloadEsElDeHoy`.
- [ ] **t3** *(grupo 3, 2 h)* — Implementar el store de `flow_agent_scopes` con UPSERT por
  `(flow_run_id, agent_id)` y una query de scopes vigentes que **excluya** el agente dado.
  Done cuando pasan `TestFlowAgentScopes_Grant_MismoAgenteMismoScope_ActualizaYNoDuplica` y
  `..._MismoAgenteRenovando_NoSeAutoBloquea`.
- [ ] **t4** *(grupo 4, 2 h)* — Extraer del handler al service la lógica de emisión:
  validar allowlist, consultar scopes vigentes, invocar `ValidarParticionDisjunta` y hacer el
  UPSERT. Done cuando pasa `TestFlowGrantToken_AllowlistSolapadaConOtroAgente_RechazaYNoEmite`
  y `handleFlowGrantToken` sigue bajo 50 líneas según `size-lint`.
- [ ] **t5** *(grupo 4, 1 h)* — Agregar el parámetro `agent_id` a la tool
  `domain_flow_grant_token` y pasarlo al service. Done cuando un grant sin el parámetro emite
  el token de hoy y uno con él lo firma.
- [ ] **t6** *(grupo 5, 2 h)* — Implementar el deny simétrico en `handleFlowValidateToken`,
  después del check de org/sesión y antes del de flow activo. Done cuando pasan los tests 2,
  3 y 4 del plan, y el 5 (fallback intacto) sigue verde.
- [ ] **t7** *(grupo 5, 2 h)* — Implementar la renovación deslizante: bajo 15 min de vigencia,
  extender `expires_at` y devolver `token` nuevo en el response. Done cuando pasan
  `..._VigenciaBajoUmbral_ExtiendeYDevuelveTokenNuevo` y `..._VigenciaHolgada_NoEscribeEnLaBase`.
- [ ] **t8** *(grupo 6, 1 h)* — `post-orchestrate.sh`: mandar `agent_id` en el JSON del grant
  (la variable ya existe desde `f58464e7`). Done cuando `bash -n` pasa y el hilo principal
  sigue emitiendo un marker con el mismo nombre que antes.
- [ ] **t9** *(grupo 6, 2 h)* — `pre-edit.sh`: mandar `agent_id` al validar, consumir el token
  renovado reescribiendo el marker, y que el deny por allowlist nombre el scope propio. Done
  cuando pasa `TestHookPreEdit_AgenteFueraDeSuAllowlist_DenyNombraSuScope`.
- [ ] **t10** *(grupo 6, 1 h)* — Liberar el scope al cancelar o cerrar el flow: marcar
  `revoked_at` en las filas del `flow_run_id`. Done cuando un re-grant tras un cancel no
  choca por solapamiento con el scope liberado.

## tests

- [ ] **t11** *(grupo 7, 2 h)* — Escribir los 12 tests del plan TDD del design, cada uno en
  rojo antes de su implementación. Los de `install-user/` con `-count=1`.
- [ ] **t12** *(grupo 7, 1 h)* — Escribir `TestValidarParticionDisjunta_TieneCallerEnProduccion`:
  grep de callers en `services/` e `install-user/` excluyendo tests. Done cuando falla al
  quitar el caller, nombrando la policy `guards-deben-ejecutarse`.

## sabotage

- [ ] **t13** *(grupo 8, 2 h)* — Ejecutar los 12 sabotajes de la tabla del design, uno por uno,
  confirmando que cada test queda **en rojo con su razón textual**. Restaurar por `cp` y
  confirmar con `sha256sum` idéntico — `git checkout --` dentro de un `.sh` evade el
  git-guard. Done cuando los 12 sabotajes están registrados con su verdict.

## docs

- [ ] **t14** *(grupo 8, 1 h)* — Documentar en el CHANGELOG (Unreleased) y en el comentario de
  `token.go` que el `session_id` se hereda del padre y el `agent_id` no, que es la razón por
  la que el eje de agente hace falta. Done cuando el CHANGELOG tiene la entrada y ningún
  comentario del repo sigue afirmando que nada distingue a un subagente.

## verify

- [ ] **t15** *(grupo 9, 2 h)* — **Criterio 4 del ticket**: correr una orquestación REAL de 2+
  agentes editando paths disjuntos y registrar la evidencia. No se cierra leyendo el hook —
  es exactamente lo que DOMAINSERV-110 declaró cumplido sin cumplir.
- [ ] **t16** *(grupo 10, 1 h)* — Auditoría final del change: ninguna función nueva supera 50
  líneas, inputs validados en el boundary, sin secretos hardcodeados, sin N+1 nuevas, sin
  código muerto, y la suite verde contra los **jobs reales del CI** (incluido `size-lint`),
  no contra `go test ./...`.
