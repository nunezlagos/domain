# issue-56.1 — Cap de bytes del additionalContext en SessionStart

El hook SessionStart inyecta un additionalContext acotado (bajo un cap de bytes
configurable) y resumido, de modo que el agente arranca cada sesión sin saturar
su ventana de contexto.

## Requisitos

### Requirement: cap de bytes configurable
El hook `domain-session-start.sh` MUST limitar el tamaño total del
`additionalContext` inyectado a un tope configurable vía `DOMAIN_CTX_MAX_BYTES`
(default 12000 bytes).

#### Scenario: payload dentro del límite
- **Given** que la suma de bootstrap + code_graph + mem_context está por debajo de `DOMAIN_CTX_MAX_BYTES`
- **When** el hook construye el `additionalContext`
- **Then** el contenido se inyecta íntegro, sin truncar

#### Scenario: payload excede el límite
- **Given** que el payload supera `DOMAIN_CTX_MAX_BYTES`
- **When** el hook construye el `additionalContext`
- **Then** cada sección se trunca de forma determinista por líneas completas
- **And** el resultado incluye la marca `[recortado por DOMAIN_CTX_MAX_BYTES]`
- **And** el tamaño final en bytes MUST ser menor o igual al cap

### Requirement: las reglas de arranque nunca se recortan
El bloque de REGLAS DE ARRANQUE (R1–R6) MUST permanecer íntegro aunque el resto
del contexto se trunque.

#### Scenario: truncado agresivo preserva las reglas
- **Given** un `DOMAIN_CTX_MAX_BYTES` muy bajo (ej. 2000) y un payload grande
- **When** el hook trunca el contexto
- **Then** las reglas R1 a R6 aparecen completas en el `additionalContext`

### Requirement: reparto de presupuesto por sección
El cap MUST repartirse entre secciones: bootstrap 45%, code_graph 25%,
mem_context 30%.

#### Scenario: una sección grande no consume el presupuesto de las otras
- **Given** un code_graph muy extenso
- **When** el hook aplica el cap
- **Then** bootstrap y mem_context conservan su cuota y no quedan vacíos
