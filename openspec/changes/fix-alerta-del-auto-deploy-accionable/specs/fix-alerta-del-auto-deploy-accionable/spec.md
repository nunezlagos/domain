# Spec — El aviso del auto-deploy pasa a ser accionable

Extensión de issue-57.2. Los tres MUST se corresponden uno a uno con los tres hallazgos
del 4R de ese issue (R3-1, R4-1 y R1-1).

## MUST-1 — El aviso nombra la causa del fallo, no solo que falló

#### Scenario: la causa está en el log del deploy y las últimas líneas del journal son de systemd
- **Given** un ciclo fallido cuyo `.deploy.log` termina en `validate: .env writable — abort`
- **And** un journal cuyas últimas líneas son `Main process exited` y `Failed with result`
- **When** se dispara la unit de alerta
- **Then** el cuerpo del aviso contiene `validate: .env writable — abort`

#### Scenario: las líneas de systemd no desplazan a la causa
- **Given** el mismo ciclo
- **When** se arma el cuerpo
- **Then** el cuerpo NO contiene `Failed with result` — si esa línea aparece, es señal de
  que volvió a quedarse con el final del journal, que es el hallazgo original

## MUST-2 — Un fallo repetido no vuelve a notificar, pero un cambio de causa sí

#### Scenario: el mismo motivo dos ciclos seguidos
- **Given** un fallo ya notificado
- **When** el ciclo siguiente falla por el MISMO motivo
- **Then** no se emite una segunda notificación, y queda registrado en el journal por qué

#### Scenario: el motivo cambia
- **Given** un fallo ya notificado
- **When** el ciclo siguiente falla por un motivo DISTINTO
- **Then** SÍ se notifica — un cambio de causa es información nueva, y silenciarlo sería
  perder justo lo que importa

#### Scenario: el primer fallo siempre avisa
- **Given** que no hay estado previo de throttle
- **When** falla un ciclo
- **Then** se notifica: el throttle nunca puede comerse el aviso inicial

## MUST-3 — El cuerpo del aviso está acotado antes de salir a un canal público

#### Scenario: una línea larga en el motivo
- **Given** un motivo con una línea de 2000 caracteres
- **When** se arma el cuerpo del aviso
- **Then** ninguna línea del aviso supera los 512 caracteres

#### Scenario: el recorte no tira la parte útil
- **Given** el mismo motivo
- **When** se recorta
- **Then** el principio del motivo sigue presente — un recorte que deje el aviso vacío
  reintroduce el MUST-1 por otra vía

## Lo que este change NO puede romper

Dos comportamientos de issue-57.2 que ya tienen test y siguen siendo criterio:

- sin `NTFY_TOPIC` la alerta sale 0 y deja rastro en el journal (degradado silencioso)
- el topic no viaja fuera de la URL del POST, ni en cuerpo, headers, flags o journal —
  el topic ES la credencial del canal

## Verificación

No se cierra leyendo el script. Se cierra mirando el canal real con `&since=all` después
de un fallo: es como se detectaron los tres hallazgos, y es el único camino que
discrimina entre "el aviso se envía" y "el aviso sirve".
