# Diseño — Autorización de edición por agente

DOMAINSERV-218, incremento 3. Cuatro ADRs y el plan TDD.

## ADR-1 — El agente es un campo opcional firmado del token

**Decisión.** `FlowTokenPayload` gana `AgentID string \`json:"a,omitempty"\``, cubierto por
el HMAC. `GenerateToken` **no** cambia de firma: el agente entra por un constructor hermano
o un struct de opciones.

**Alternativas.**

- *Parámetro posicional nuevo* — rechazada. La firma actual es
  `GenerateToken(flowRunID, sessionID, orgID string, allowedPaths ...string)`; un posicional
  antes del variádico rompe a todos los callers, incluidos los tests verdes, y convierte un
  cambio aditivo en uno masivo justo en el camino que autoriza las propias ediciones.
- *El agente solo en la tabla* — rechazada. El token dejaría de ser autocontenido y un token
  robado seguiría sirviendo para cualquier agente: nada en su firma lo ata a uno.

**Tradeoff.** `omitempty` hace que un grant sin agente produzca un JSON idéntico al de hoy:
el token del hilo principal es el mismo en su parte firmada. Eso es lo que hace seguro tocar
este código — si el cambio sale mal, el hilo principal conserva la autorización con la que
arreglarlo. El costo es que "sin agente" y "agente vacío" son indistinguibles, aceptable
porque el ADR-3 trata ambos igual.

**Seguridad del cambio.** El campo entra a la firma: no es manipulable sin el secreto. No
agrega PII — el `agent_id` es un identificador efímero del runtime, no un dato de persona.

**Patrón.** Extensión aditiva de payload firmado (mismo criterio que el campo 6 del marker
de commit-gate en `06097069`).

## ADR-2 — Una fila por `(flow_run_id, agent_id)` con UPSERT, no una por token emitido

**Decisión.** Migración `000289` crea `flow_agent_scopes` con UNIQUE
`(flow_run_id, agent_id)`; el grant hace `INSERT ... ON CONFLICT DO UPDATE`. El chequeo de
solapamiento compara contra las filas vigentes del mismo flow **excluyendo la propia**.

**Alternativas.**

- *Log de emisiones (una fila por token)* — rechazada por una razón medida. El matcher de
  `claude_hook.go:41` dispara el grant en `orchestrate_phase_result`, `flow_status`,
  `orchestrate_confirm` y `flow_cancel`, no solo en `orchestrate`: **cada cierre de fase
  re-emite**. Con una fila por emisión, la segunda emisión de A choca con la fila anterior
  de A y el rechazo por solapamiento **lo bloquea a sí mismo**. El bug aparecería recién en
  la segunda fase de cualquier flow real.
- *Sin persistencia* — es el estado actual, y es lo que deja a `ValidarParticionDisjunta`
  sin nada contra qué comparar.

**Tradeoff.** Se pierde el historial de emisiones: la tabla dice qué scope tiene cada agente
ahora, no cuántas veces renovó. Aceptable — el `flow_run` ya tiene su historial de fases, y
un log de emisiones crecería sin cota con la renovación por fase.

**Seguridad del cambio.** RLS por `project_id` siguiendo la `000288`: `current_project_id()`
con `CREATE OR REPLACE` y `nullif`, nunca `current_org_id()` (eliminada en la 143 — asumirla
revienta el migrate en un deploy limpio). El eje org es decorativo en esta instancia
(`canonicalOrgID` fijo en `internal/auth/apikey/store.go:30`), así que un RLS por org no
aislaría nada. Índice sin `CONCURRENTLY` con el escape `domain-lint-ignore-next` y la razón
escrita: golang-migrate manda el archivo entero en un `Exec` y `CONCURRENTLY` falla con
25001 (precedentes `000161`, `000272:13`, `000288`).

**Patrón.** Upsert idempotente sobre clave natural.

## ADR-3 — Deny simétrico ante mismatch, sin romper el fallback

**Decisión.** `validate_token` deniega con `reason:"agent_mismatch"` ante cualquier
diferencia: A con token de B, hilo principal con token de subagente, y subagente con token
sin agente.

**Alternativas.**

- *Asimétrico* (token con agente exige match; token sin agente lo usa cualquiera) —
  rechazada: deja una escalación trivial. Un subagente que quiere salirse de su scope pide
  un token sin `agent_id` y recupera la autorización amplia.
- *Warning sin deny* — auditoría sin enforcement; el criterio 2 exige denegar.

**Tradeoff, y es el riesgo principal del change.** Un gate que deniega lo legítimo empuja al
bypass permanente (DOMAINSERV-111/175/195). La simetría es segura **solo** por una distinción
que hay que sostener en el código: el deny aplica cuando **hay** un token de otro agente, no
cuando **no hay ninguno**. Un subagente sin token propio sigue cayendo al fallback del marker
de sesión del padre (`pre-edit.sh:114-121`) y edita bajo el flow del padre. Si alguien
"simplifica" eso convirtiendo la ausencia de token en un deny, el gate se vuelve
insatisfacible para todo subagente. Está escrito como MUST-6 con escenario propio para que un
test lo sostenga.

**Seguridad del cambio.** Cierra el replay cross-agente dentro de una misma sesión, que es el
hueco que el incremento 2 no podía cerrar: el `session_id` se hereda del padre, así que el
check de sesión de DOMAINSERV-98 no discrimina entre agentes de la misma sesión.

**Patrón.** Verificación de audiencia sobre token firmado.

## ADR-4 — El TTL mide inactividad: renovación deslizante, y 30 min se mantiene

**Decisión.** Una validación exitosa con menos de 15 min de vigencia restante extiende
`expires_at` y devuelve un token nuevo en el response. `FlowTokenTTL` sigue en 30 min.

**Alternativas.**

- *TTL de 2 h sin renovación* — evaluada con el usuario y descartada al ver la medición.
  Cuadruplica la ventana de un token robado y no resuelve el caso que la motivaba: una fase
  de 2 h 1 min se queda igual sin autorización. Un TTL fijo siempre elige mal porque mide la
  variable equivocada.
- *Heartbeat periódico* — camino nuevo que puede fallar en silencio, y ya existe uno natural.

**Lo medido que habilita esto.** Cada cierre de fase ya re-emite el token (ver ADR-2), así
que una tarea de 5 horas nunca se quedó sin autorización. El único hueco real es la fase
**única** que pasa más de un TTL sin cerrar — un `sdd-apply` largo. La renovación deslizante
cubre exactamente ese hueco sin agrandar la ventana.

**Tradeoff.** `validate_token` deja de ser solo-lectura: escribe en el camino del pre-edit,
que es caliente. Se acota escribiendo solo bajo el umbral de 15 min — en el peor caso un
UPDATE cada 15 min por agente, no uno por edición. Si el hook no consume el token renovado,
el comportamiento degrada al de hoy (vence y cae al gate), no a algo peor.

**Patrón.** Sliding expiration con refresco anticipado.

## Plan TDD

Orden de escritura. Cada test se escribe en rojo, se implementa el mínimo, y después se
sabotea para confirmar que atrapa la regresión. El sabotaje se restaura por `cp` y se
verifica con `sha256sum` idéntico: `git checkout --` dentro de un `.sh` evade el git-guard.

| # | Test | Qué prueba | Sabotaje concreto |
|---|---|---|---|
| 1 | `TestFlowToken_GenerateToken_SinAgente_PayloadEsElDeHoy` | La compat hacia atrás, ANTES de tocar nada más | Quitar `omitempty` del tag de `AgentID` en `token.go` → el JSON gana `"a":""` y el payload deja de ser idéntico |
| 2 | `TestFlowToken_ValidateToken_AgenteDistinto_DevuelveAgentMismatch` | El deny cuando A presenta el token de B | Cambiar el `!=` del check de agente por `==` en `orchestrate_tools.go` |
| 3 | `TestFlowToken_ValidateToken_HiloPrincipalConTokenDeSubagente_DevuelveAgentMismatch` | La simetría, sentido subagente→principal | Añadir `if quienValida == "" { return ok }` antes del check |
| 4 | `TestFlowToken_ValidateToken_SubagenteConTokenSinAgente_DevuelveAgentMismatch` | La simetría, sentido principal→subagente: cierra la escalación | Añadir `if payload.AgentID == "" { return ok }` antes del check |
| 5 | `TestFlowToken_ValidateToken_SubagenteSinTokenPropio_CaeAlFallbackYNoQuedaBloqueado` | Que el gate siga satisfacible — el guardián del ADR-3 | Convertir la ausencia de token en deny |
| 6 | `TestFlowAgentScopes_Grant_MismoAgenteMismoScope_ActualizaYNoDuplica` | El UPSERT | Cambiar `ON CONFLICT DO UPDATE` por `INSERT` a secas → falla por unique violation |
| 7 | `TestFlowAgentScopes_Grant_MismoAgenteRenovando_NoSeAutoBloquea` | El bug que el ADR-2 previene | Quitar la exclusión de la fila propia del `WHERE` del chequeo de solapamiento |
| 8 | `TestFlowGrantToken_AllowlistSolapadaConOtroAgente_RechazaYNoEmite` | El criterio 3, con el guard ya conectado | Comentar la llamada a `ValidarParticionDisjunta` en el service |
| 9 | `TestFlowValidateToken_VigenciaBajoUmbral_ExtiendeYDevuelveTokenNuevo` | La renovación deslizante | Cambiar el umbral de `< 15*time.Minute` a `< 0` → nunca renueva |
| 10 | `TestFlowValidateToken_VigenciaHolgada_NoEscribeEnLaBase` | Que el camino caliente no escriba de más | Quitar la guarda del umbral → escribe en cada validación |
| 11 | `TestHookPreEdit_AgenteFueraDeSuAllowlist_DenyNombraSuScope` | El criterio 2 end-to-end en el hook | Reemplazar el scope propio por un literal genérico en el mensaje de deny |
| 12 | `TestValidarParticionDisjunta_TieneCallerEnProduccion` | Que el guard deje de ser inerte — guard sobre el guard | Quitar el caller: el test falla nombrando la policy `guards-deben-ejecutarse` |

**Ejecución.** Los tests de `install-user/` corren con `-count=1`: leen archivos de fuera del
módulo y el cache de `go test` da falso verde. La verificación no se cierra con
`go test ./...` sino contra los jobs reales del CI, incluido `size-lint`, que `go test` no
corre y que ya dejó dos jobs en rojo sin que nadie lo notara (DOMAINSERV-234, `5a705a2c`).

**Cierre del criterio 4.** Ninguno de los 12 tests lo cumple: exige una orquestación real de
2+ agentes editando paths disjuntos. Va como task propia en `sdd-verify`, no como test.
