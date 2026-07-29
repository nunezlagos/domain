# acp-rolling-model

## ADDED Requirements

### Requirement: roster automático de free models
El provider MUST descubrir el roster de modelos free automáticamente vía `opencode models --verbose` filtrando `cost==0`; NO usa lista hardcodeada.

#### Scenario: descubrimiento al arranque
- **Given** el provider ACP inicializado
- **When** arranca
- **Then** parsea el output de `opencode models --verbose` y arma el roster con los modelos de cost 0

### Requirement: rotación round-robin con cooldown
El provider MUST rotar el modelo por `Complete()` (round-robin) y poner en cooldown un modelo que falle.

#### Scenario: modelo falla
- **Given** un modelo que devuelve rate-limit/timeout/connection-drop
- **When** falla en un Complete
- **Then** entra en cooldown TTL y la llamada rota al siguiente sano

### Requirement: fallback y refresh
El provider MUST conservar el último roster conocido si el descubrimiento falla, y refrescarlo periódicamente en background.

#### Scenario: descubrimiento falla
- **Given** un refresh que falla o devuelve vacío
- **When** ocurre
- **Then** se conserva y usa el último roster cacheado
