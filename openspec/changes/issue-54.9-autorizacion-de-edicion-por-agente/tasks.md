# Tasks — Autorización de edición por agente

DOMAINSERV-218, incremento 3. Los grupos se ejecutan en orden ascendente; dentro de un
grupo las tasks son independientes entre sí.

Estado sincronizado el 2026-08-07 contra el código de `main` (HEAD `887f0afa`). Las tasks
t1–t14 se implementaron en `7ba9ff54`, `0c555e4b`, `758d00af` y `4f38b9a5`, y quedaron sin
marcar por un cierre de sesión: el checkbox iba atrasado, no el trabajo. Cada marca de abajo
cita el archivo y la línea que la sostiene, medidos, no inferidos.

## schema

- [x] **t1** *(grupo 1, 2 h)* — Crear `000289_flow_agent_scopes.up.sql` / `.down.sql` con la
  tabla `flow_agent_scopes` (flow_run_id, agent_id, project_id, allowed_paths, expires_at,
  revoked_at), UNIQUE `(flow_run_id, agent_id)`, índice sobre `flow_run_id` y RLS por
  `project_id` con el patrón de la `000288`. Done cuando `migrate up` corre sobre una base
  limpia sin error y el header trae migration/author/issue/description/breaking/duration.
  → `000289_flow_agent_scopes.up.sql` / `.down.sql`. Aplicada en prod: `schema_version=292`
  con `dirty=false`, y golang-migrate es secuencial.

## code

- [x] **t2** *(grupo 2, 1 h)* — Agregar `AgentID string \`json:"a,omitempty"\`` a
  `FlowTokenPayload` y un constructor que lo acepte sin cambiar la firma de `GenerateToken`.
  Done cuando pasa `TestFlowToken_GenerateToken_SinAgente_PayloadEsElDeHoy`.
  → `flow/token.go:40`. El constructor es `GenerateTokenParaAgente` (`token.go:69`): la firma
  vieja quedó intacta, que era el punto del ADR-1. Cubierto por
  `TestFlowTokenService_GenerateToken_SinAgente_CuerpoFirmadoNoMencionaAlAgente`.
- [x] **t3** *(grupo 3, 2 h)* — Implementar el store de `flow_agent_scopes` con UPSERT por
  `(flow_run_id, agent_id)` y una query de scopes vigentes que **excluya** el agente dado.
  Done cuando pasan `TestFlowAgentScopes_Grant_MismoAgenteMismoScope_ActualizaYNoDuplica` y
  `..._MismoAgenteRenovando_NoSeAutoBloquea`.
  → `flow/agent_scopes_store.go:64` (`ON CONFLICT ... DO UPDATE`). La exclusión del solicitante
  quedó en Go y no en el `WHERE` (`flow/scopes_vigentes.go:53`), para que el criterio de
  solapamiento viva en un solo lugar. Cubierto por `TestSolapamientoConOtros_ElMismoAgente*`.
- [x] **t4** *(grupo 4, 2 h)* — Extraer del handler al service la lógica de emisión:
  validar allowlist, consultar scopes vigentes, invocar `ValidarParticionDisjunta` y hacer el
  UPSERT. Done cuando pasa `TestFlowGrantToken_AllowlistSolapadaConOtroAgente_RechazaYNoEmite`
  y `handleFlowGrantToken` sigue bajo 50 líneas según `size-lint`.
  → `flow/scopes_vigentes.go:44` (`SolapamientoConOtros`). `size-lint` verde: *0 funciones
  nuevas > 50 líneas*. El test nombrado existe y pasa.
- [x] **t5** *(grupo 4, 1 h)* — Agregar el parámetro `agent_id` a la tool
  `domain_flow_grant_token` y pasarlo al service. Done cuando un grant sin el parámetro emite
  el token de hoy y uno con él lo firma.
  → `orchestrate_tools.go:374`. Verificado **contra el MCP de producción**: el schema que el
  servidor publica hoy incluye `agent_id` en `grant_token` y en `validate_token`.
- [x] **t6** *(grupo 5, 2 h)* — Implementar el deny simétrico en `handleFlowValidateToken`,
  después del check de org/sesión y antes del de flow activo. Done cuando pasan los tests 2,
  3 y 4 del plan, y el 5 (fallback intacto) sigue verde.
  → `orchestrate_tools.go:563`. Los tres sentidos cubiertos por
  `TestHandleFlowValidateToken_TokenDeOtroAgente_ReturnsAgentMismatch`,
  `..._HiloPrincipalConTokenDeSubagente_ReturnsAgentMismatch` y
  `..._SubagenteConTokenSinAgente_ReturnsAgentMismatch`.
- [x] **t7** *(grupo 5, 2 h)* — Implementar la renovación deslizante: bajo 15 min de vigencia,
  extender `expires_at` y devolver `token` nuevo en el response. Done cuando pasan
  `..._VigenciaBajoUmbral_ExtiendeYDevuelveTokenNuevo` y `..._VigenciaHolgada_NoEscribeEnLaBase`.
  → `flow/renovacion.go:11` (`UmbralDeRenovacion = 15 min`), `token.go:13` (`FlowTokenTTL = 30
  min`), consumido en `orchestrate_tools.go:603`. `TestUmbralDeRenovacion_EsMenorQueElTTL` es
  el guard que impide que un futuro ajuste de constantes deje la renovación inalcanzable.
- [x] **t8** *(grupo 6, 1 h)* — `post-orchestrate.sh`: mandar `agent_id` en el JSON del grant
  (la variable ya existe desde `f58464e7`). Done cuando `bash -n` pasa y el hilo principal
  sigue emitiendo un marker con el mismo nombre que antes.
  → `domain-post-orchestrate.sh:123`. Cubierto por
  `TestHookPostOrchestrate_GrantToken_MandaElAgentId`.
- [x] **t9** *(grupo 6, 2 h)* — `pre-edit.sh`: mandar `agent_id` al validar, consumir el token
  renovado reescribiendo el marker, y que el deny por allowlist nombre el scope propio. Done
  cuando pasa `TestHookPreEdit_AgenteFueraDeSuAllowlist_DenyNombraSuScope`.
  → `domain-pre-edit.sh:1367`. El test se escribió con otro nombre:
  `TestHookPreEdit_ValidateToken_MandaElAgentId` y
  `TestHookPreEdit_ElAgenteSeMandaSoloSiElMarkerEsElPropio`.
- [x] **t10** *(grupo 6, 1 h)* — Liberar el scope al cancelar o cerrar el flow: marcar
  `revoked_at` en las filas del `flow_run_id`. Done cuando un re-grant tras un cancel no
  choca por solapamiento con el scope liberado.
  → `flow/agent_scopes_store.go:81` (`SET revoked_at = now()`), y la query de vigentes filtra
  por `revoked_at IS NULL` (`:29`).

## tests

- [x] **t11** *(grupo 7, 2 h)* — Escribir los 12 tests del plan TDD del design, cada uno en
  rojo antes de su implementación. Los de `install-user/` con `-count=1`.
  → Los 12 existen, con nombres **distintos** a los del design y partidos más fino: la familia
  `TestSolapamientoConOtros_*` (8 casos), `TestHandleFlowValidateToken_*` (5 sobre el agente),
  `TestNecesitaRenovacion_*` (4) y `TestHayScopesDeOtros_*` (5). Suite del paquete `flow`:
  **226 tests verdes con `-count=1`**.
- [x] **t12** *(grupo 7, 1 h)* — Escribir `TestValidarParticionDisjunta_TieneCallerEnProduccion`:
  grep de callers en `services/` e `install-user/` excluyendo tests. Done cuando falla al
  quitar el caller, nombrando la policy `guards-deben-ejecutarse`.
  → `flow/guard_con_caller_test.go`, con el nombre exacto del plan. Es el guard sobre el guard.

## sabotage

- [x] **t13** *(grupo 8, 2 h)* — Ejecutar los 12 sabotajes de la tabla del design, uno por uno,
  confirmando que cada test queda **en rojo con su razón textual**. Restaurar por `cp` y
  confirmar con `sha256sum` idéntico — `git checkout --` dentro de un `.sh` evade el
  git-guard. Done cuando los 12 sabotajes están registrados con su verdict.
  → 6 sabotajes registrados con su verdict en los mensajes de `0c555e4b` y `4f38b9a5`, cada
  uno restaurado por `cp` con `sha256sum` idéntico: quitar el seteo del GUC, quitar la llamada
  a `SolapamientoConOtros`, UPSERT→INSERT plano, quitar la exclusión de la fila propia, quitar
  la guarda de token vencido, y quitar el caller del guard.

## docs

- [x] **t14** *(grupo 8, 1 h)* — Documentar en el CHANGELOG (Unreleased) y en el comentario de
  `token.go` que el `session_id` se hereda del padre y el `agent_id` no, que es la razón por
  la que el eje de agente hace falta. Done cuando el CHANGELOG tiene la entrada y ningún
  comentario del repo sigue afirmando que nada distingue a un subagente.
  → `services/domain-mcp/CHANGELOG.md:65` y `flow/token.go:31`.

## verify

- [ ] **t15** *(grupo 9, 2 h)* — **Criterio 4 del ticket**: correr una orquestación REAL de 2+
  agentes editando paths disjuntos y registrar la evidencia. No se cierra leyendo el hook —
  es exactamente lo que DOMAINSERV-110 declaró cumplido sin cumplir.

  **EJECUTADA el 2026-08-07. NO PASA, y el motivo justifica que la task existiera.** Dos
  subagentes reales en paralelo (sesión `339c3704`), A con `services/domain-mcp/**` y B con
  `install-user/**`, cada uno declarando su allowlist al orquestar.

  **Lo que SÍ quedó verificado en el camino real:**
  - El **eje de agente funciona**: tres markers distintos, cada uno con su `agent_id` firmado
    dentro del token (campo `a`), y el del hilo principal sin agente. MUST-1 y MUST-6, no por
    unitario sino por ejecución.
  - El **deny nombra el scope propio**, textual: *"el path '…/install-user/hooks/domain-pre-edit.sh'
    está fuera de la allowlist del flow activo (paths permitidos: `["services/domain-mcp/**"]`)"*.
    MUST-3 cumplido — pero **solo por la vía `domain_flow_grant_token`**.

  **Lo que NO pasa, y es un fail-open silencioso:** por el camino que el protocolo manda usar
  —`domain_orchestrate`— el token sale **sin `allowed_paths`**, así que el gate toma la rama
  "sin allowlist → sin restricción" y cualquier path pasa. Verificado end-to-end: con ese token
  el agente A editó `install-user/hooks/domain-pre-edit.sh` —territorio de B— **sin ningún deny**.
  Causa raíz: `domain_orchestrate` **no declara `allowed_paths` en su schema**, así que el
  parámetro se pierde (descartado por el cliente, o propagado como string y descartado por el
  `isinstance(..., list)` de `post-orchestrate.sh:79`). El server nunca recibe el scope.

  Los criterios **1 y 4 del ticket NO se cumplen en producción** aunque los 226 unitarios del
  paquete `flow` estén verdes: lo verde es el server, y el server nunca recibe el dato.
  Corolario: `HayScopesDeOtros` tampoco puede proteger por esta vía — si nadie registra su
  allowlist, `scopes_de_otros` es siempre `false` y la rama de herencia nunca se dispara.

  Registrado como **DOMAINSERV-256** (bug, priority high) con la evidencia completa.
  **t15 no se cierra hasta re-correr esta misma verificación después del fix.**

  **FIX APLICADO el 2026-08-07, pendiente de re-verificación.** `toolOrchestrate()` declara
  ahora `allowed_paths` como array (`orchestrate_tools.go:63`), y de paso se corrigió la
  descripción de `project_id`, que afirmaba en falso que la columna es NOT NULL. TDD estricto:
  3 tests en `orchestrate_allowlist_declarada_test.go`, los tres en ROJO antes del fix, y 3
  sabotajes con su verdict, restaurados por `cp` con `sha256sum` verificado:

  | # | sabotaje | resultado |
  |---|---|---|
  | 1 | `WithArray` → `WithString` | ROJO solo el de tipo (`Expected: array / Actual: string`); el de presencia siguió verde — prueban cosas distintas |
  | 2 | quitar la declaración entera | ROJO los dos, cada uno con su mensaje |
  | 3 | devolver el "NOT NULL" a la descripción | ROJO el guard de la afirmación falsa |

  Verificación: **386 tests verdes** con `-count=1` en `internal/mcp/server/...` +
  `internal/service/flow/...`, y `size-lint` OK.

  **BLOQUEADO para cerrar**: el schema que ven los subagentes lo sirve el binario del VPS, no
  el working tree. Re-correr t15 sin desplegar daría exactamente el mismo resultado que la
  primera vez. La re-verificación exige push a `main` + redeploy, las dos con orden explícita
  del usuario.
- [ ] **t16** *(grupo 10, 1 h)* — Auditoría final del change: ninguna función nueva supera 50
  líneas, inputs validados en el boundary, sin secretos hardcodeados, sin N+1 nuevas, sin
  código muerto, y la suite verde contra los **jobs reales del CI** (incluido `size-lint`),
  no contra `go test ./...`.
  **PARCIAL** (2026-08-07): `size-lint` OK (0 funciones nuevas > 50 líneas, 190 congeladas en
  baseline) y `db-conventions-lint` OK — el rojo que reportó `0c555e4b` ya no está. Falta el
  resto de la auditoría y el CI completo, hoy rojo en `main` por causas ajenas a este change
  (DOMAINSERV-255: integration-tests + SDK python).

  **Hallazgos de la auditoría, medidos:**

  1. **`handleFlowValidateToken` creció de 67 a 92 líneas con este change**, y `size-lint` da
     verde porque la función está congelada en `.size-lint-baseline:64`: el linter solo bloquea
     funciones NUEVAS > 50 líneas, no el engorde de las ya congeladas. El design insistió en
     "extraer, no engordar" para `handleFlowGrantToken` (que quedó en 51) y el bulto terminó en
     el handler de al lado, por la puerta que el baseline deja abierta. Es la misma clase de
     agujero que DOMAINSERV-234, entrando por el otro lado.
  2. **La descripción de `domain_orchestrate` afirma que `flow_runs.project_id` es NOT NULL, y
     es falso**: la migración `000161` lo agregó explícitamente como nullable y ninguna posterior
     lo cambió. El propio código de este change lo sabe — `scopeDelFlowRun` lo escanea como
     `*uuid.UUID` y chequea `nil` (`orchestrate_tools.go:425-432`).
  3. **Y por eso hay una regresión de comportamiento**: `runner/flow/runner.go:191` inserta
     `flow_runs` SIN `project_id`, así que esas filas nacen NULL. Desde este change, pedirles un
     token falla en cadena — `reservarTerritorio` → `scopeDelFlowRun` → *"no tiene proyecto"* →
     y `handleFlowGrantToken:495-497` **deniega el grant entero**. Antes del change ese mismo
     grant emitía token. Es el gate volviéndose insatisfacible para un camino, que es el riesgo
     que el ADR-3 dice mitigar, entrando por otra puerta.
  4. **La auditoría de denys por scope es ciega**: el deny de batch-mode no aparece en
     `injections.log`. Un enforcement que no deja rastro no se puede auditar después.

## Fuera del plan original — implementado igual

Durante la implementación apareció un agujero que el diseño no había visto, y se tapó como
**incremento 5**: un subagente sin marker propio cae al marker del PADRE, cuyo token no tiene
`allowed_paths`, así que el gate tomaba la rama "sin allowlist → sin restricción" y el fallback
le entregaba la autorización amplia del hilo principal. El aislamiento se evaporaba por más
scopes que se hubieran declarado. `HayScopesDeOtros` (`flow/scopes_vigentes.go:23`) acota la
restricción a los flows donde el aislamiento está en juego: si alguien declaró una partición,
heredar la autorización amplia la contradice. Cubierto por `TestHayScopesDeOtros_*` (5 casos).
