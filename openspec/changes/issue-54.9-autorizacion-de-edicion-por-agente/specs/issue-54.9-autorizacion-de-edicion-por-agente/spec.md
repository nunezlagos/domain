# Spec — Autorización de edición por agente

DOMAINSERV-218, incremento 3. Cierra los 4 criterios de aceptación del ticket.

## ADDED Requirements

### REQ-218.1 — El token MUST firmar el agente, y el mismatch se deniega en los dos sentidos

El `FlowTokenPayload` MUST incluir un campo de agente cubierto por la firma HMAC.
`domain_flow_validate_token` MUST rechazar toda combinación en la que el agente del token
y el de quien valida no sean idénticos.

#### Scenario: un agente presenta el token de otro
- **Given** un token emitido para el agente A del flow F
- **When** el agente B presenta ese token en `domain_flow_validate_token`
- **Then** el response es `{valid:false, reason:"agent_mismatch"}` y no incluye `allowed_paths`

#### Scenario: el hilo principal presenta el token de un subagente
- **Given** un token emitido para el agente A del flow F
- **When** quien valida no manda `agent_id`
- **Then** el response es `{valid:false, reason:"agent_mismatch"}`

#### Scenario: un subagente presenta un token sin agente
- **Given** un token emitido SIN `agent_id`
- **When** el agente A lo presenta junto con su `agent_id`
- **Then** el response es `{valid:false, reason:"agent_mismatch"}`
- **And** queda cerrada la vía de pedir un token sin agente para saltear el aislamiento

### REQ-218.2 — Dos agentes con scopes disjuntos MUST editar en paralelo sin desactivar el gate

Criterios 1 y 4 del ticket.

#### Scenario: paralelismo verificado con una orquestación real
- **Given** un flow con dos subagentes, A con `["services/domain-mcp/**"]` y B con `["install-user/**"]`, cada uno con su token
- **When** ambos editan un archivo dentro de su propio scope en paralelo
- **Then** las dos ediciones pasan el gate sin bypass ni desactivación
- **And** la evidencia proviene de ejecutar la orquestación, no de leer el hook

### REQ-218.3 — Un agente MUST ser denegado fuera de SU allowlist aunque el path esté en la de otro

Criterio 2 del ticket.

#### Scenario: A intenta editar el territorio de B
- **Given** A con scope `services/domain-mcp/**` y B con scope `install-user/**`, ambos vigentes en el mismo flow
- **When** A intenta editar `install-user/hooks/domain-pre-edit.sh`
- **Then** el gate deniega la edición
- **And** la razón nombra el scope de A, no un "no autorizado" genérico

### REQ-218.4 — Dos allowlists solapadas MUST rechazarse al emitir, no al editar

Criterio 3 del ticket. `flow.ValidarParticionDisjunta` deja de ser un guard inerte.

#### Scenario: el segundo grant reclama territorio ya reservado
- **Given** un token vigente del agente A con `["services/domain-mcp/**"]` en el flow F
- **When** se pide un token para el agente B en el mismo flow con `["services/domain-mcp/internal/**"]`
- **Then** `domain_flow_grant_token` devuelve un error de solapamiento que nombra los dos globs en conflicto
- **And** no se emite ningún token

#### Scenario: el guard tiene un caller en producción
- **Given** la función `flow.ValidarParticionDisjunta`
- **When** se buscan sus callers en `services/` e `install-user/` excluyendo tests
- **Then** existe al menos uno en el camino de producción de `handleFlowGrantToken`

### REQ-218.5 — La renovación del propio agente MUST NO auto-bloquearse

#### Scenario: A renueva su token al cerrar una fase
- **Given** el agente A con un token vigente y su scope registrado en el flow F
- **When** A pide un token nuevo para el mismo flow con el mismo scope
- **Then** el grant tiene éxito
- **And** la fila de A se actualiza en vez de duplicarse
- **And** el chequeo de solapamiento excluye la fila propia de A

### REQ-218.6 — La compatibilidad hacia atrás del hilo principal MUST conservarse

#### Scenario: un grant sin agente emite el token de hoy
- **Given** una llamada a `domain_flow_grant_token` sin `agent_id`
- **When** se compara la parte firmada del payload con la de antes de este cambio
- **Then** es equivalente
- **And** el hilo principal sigue autorizado a editar

#### Scenario: un subagente sin token propio no queda bloqueado
- **Given** un subagente al que el orquestador todavía no le emitió token
- **When** intenta editar bajo el flow que abrió el padre
- **Then** el fallback al marker de sesión lo autoriza, como en el incremento 2

### REQ-218.7 — El TTL MUST medir inactividad, no duración de la tarea

El TTL se mantiene en 30 minutos.

#### Scenario: una fase larga no pierde la autorización mientras edita
- **Given** un token con menos de 15 minutos de vigencia restante
- **When** el agente lo valida en un pre-edit
- **Then** la validación extiende la expiración de su fila
- **And** devuelve un token renovado sin que el agente tenga que cerrar una fase

#### Scenario: un agente inactivo pierde la autorización
- **Given** un token cuya última validación fue hace más de 30 minutos
- **When** el agente intenta editar
- **Then** el gate no lo autoriza
- **And** cae al camino de reposición existente (`flow_status` o cierre de fase)
