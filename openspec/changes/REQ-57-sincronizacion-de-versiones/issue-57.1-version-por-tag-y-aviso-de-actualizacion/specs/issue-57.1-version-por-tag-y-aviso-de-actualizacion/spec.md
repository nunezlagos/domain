# Spec: versión por tag y aviso de actualización

**Issue:** `issue-57.1-version-por-tag-y-aviso-de-actualizacion`
**REQ padre:** `REQ-57-sincronizacion-de-versiones`
**Estado:** proposed

## Contexto

Tres artefactos se mueven por separado y nada los sincroniza ni detecta cuando divergen:

1. el **MCP server** del VPS, que se redespliega a mano;
2. el **binario y los hooks del usuario**, que se instalan clonando el repo y compilando con Go;
3. la **release de GitHub**, que solo se publica con un tag.

Si el server avanza y un cliente queda viejo, **nada lo detecta**. El síntoma sería una tool que no
existe o un response con shape distinto — y la policy `mcp-response-shape-contract` dice justamente
que ese shape es un contrato.

### Bloqueante medido

`install-user/Makefile` compila con `LDFLAGS := -s -w`, **sin `-X`**: el binario del usuario no sabe
qué versión es. Sin eso no hay nada que comparar. El server sí tiene `Version/Commit/BuildTime`.

### Lo que ya existe y se reusa

- `release-installer.yml` construye 5 targets y publica en Releases al pushear un tag `v*`. **Bajar
  de Releases es anónimo**: un usuario que solo clonó la solución no necesita estar logueado.
- `scripts/deploy.sh` tiene 5 fases con rollback, dry-run y test propio, y **no usa `sudo`**
  (medido: 0 coincidencias). El usuario `sysadmin` del VPS ya está en el grupo docker.
- El hook `SessionStart` **ya hace un round-trip al VPS en cada sesión** e inyecta
  `additionalContext`.

## Requisitos

### REQ-1 — El binario del usuario MUST conocer su versión

#### Scenario: El binario reporta la versión con la que se compiló
- **Given** un binario construido desde el tag `v1.2.0`
- **When** se le pide su versión
- **Then** reporta `1.2.0` y el commit desde el que se construyó

#### Scenario: Un build local sin tag no miente sobre su versión
- **Given** un binario compilado fuera del pipeline de release
- **When** se le pide su versión
- **Then** reporta un valor que lo identifica como build de desarrollo
- **And** NO reporta una versión de release que no le corresponde

### REQ-2 — El server MUST exponer su versión en el bootstrap

El hook ya llama a `domain_session_bootstrap` en cada sesión: la versión viaja en ese round-trip.

#### Scenario: El bootstrap incluye la versión del server
- **Given** una sesión que arranca
- **When** el hook llama a `domain_session_bootstrap`
- **Then** el response incluye la versión del server y la versión mínima de cliente que soporta

### REQ-3 — El SessionStart MUST avisar cuando el cliente quedó viejo, sin bloquear

#### Scenario: Cliente desactualizado
- **Given** un cliente en `1.1.0` y un server en `1.2.0`
- **When** arranca una sesión
- **Then** el bloque de inicio dice que hay una versión nueva y cuál es el comando para actualizar
- **And** la sesión arranca igual: el aviso NO bloquea

#### Scenario: Cliente al día
- **Given** un cliente y un server en la misma versión
- **When** arranca una sesión
- **Then** no se agrega ninguna línea de actualización al bloque

#### Scenario: El server no responde
- **Given** un VPS caído o inalcanzable
- **When** arranca una sesión
- **Then** el hook no rompe ni bloquea, y la sesión arranca sin el aviso
- **And** el comportamiento es el mismo que hoy ante un fallo del bootstrap

### REQ-4 — El cliente MUST poder actualizarse en un paso, sin Git ni Go

#### Scenario: Actualizar desde un binario de release
- **Given** un usuario con el cliente desactualizado
- **When** ejecuta el comando de actualización
- **Then** se descarga el binario de la release desde GitHub, **sin credenciales**
- **And** se reinstalan los hooks
- **And** al terminar, la versión reportada coincide con la de la release

#### Scenario: Nunca se auto-actualiza solo
- **Given** un cliente desactualizado
- **When** arranca una sesión
- **Then** NO se descarga ni se reemplaza ningún binario sin que el usuario lo pida

#### Scenario: Una actualización fallida no deja al usuario sin MCP
- **Given** una actualización que falla a mitad (descarga corrupta, disco lleno)
- **When** termina el intento
- **Then** el binario anterior sigue funcionando

### REQ-5 — El VPS MUST desplegarse solo ante un tag nuevo, sin runner ni credenciales

#### Scenario: Un tag nuevo dispara el deploy
- **Given** el VPS desplegado en el tag `v1.1.0`
- **And** un tag `v1.2.0` publicado en el remoto
- **When** corre el ciclo de verificación del cron
- **Then** se ejecuta `scripts/deploy.sh` y el VPS queda en `v1.2.0`

#### Scenario: Sin tag nuevo no pasa nada
- **Given** el VPS desplegado en el último tag existente
- **When** corre el ciclo del cron
- **Then** no se ejecuta ningún deploy y no se reinicia ningún container

#### Scenario: Un push a main sin tag NO despliega
- **Given** commits nuevos en `main` sin tag
- **When** corre el ciclo del cron
- **Then** no se despliega: el deploy es un acto deliberado

#### Scenario: Un deploy fallido revierte
- **Given** un tag cuyo build o verificación falla
- **When** el cron ejecuta el deploy
- **Then** aplica el rollback que `scripts/deploy.sh` ya implementa
- **And** el VPS sigue sirviendo la versión anterior

#### Scenario: Dos ciclos no se pisan
- **Given** un deploy en curso que tarda más que el intervalo del cron
- **When** llega el siguiente ciclo
- **Then** no arranca un segundo deploy en paralelo

### REQ-6 — El server MUST declarar la versión mínima de cliente que soporta

Con deploy automático y un cliente que **no** se auto-actualiza, el server siempre corre lo último y
los clientes quedan atrás por definición. La divergencia deja de ser un riesgo y pasa a ser la
consecuencia esperada, así que el piso de compatibilidad tiene que ser explícito.

#### Scenario: Cliente por debajo del mínimo soportado
- **Given** un server que declara soportar clientes desde `1.2.0`
- **And** un cliente en `1.0.0`
- **When** arranca una sesión
- **Then** el aviso es explícito sobre que la versión ya no está soportada, no un "hay algo nuevo"
- **And** la sesión arranca igual: avisar no es romper

#### Scenario: Cliente viejo pero soportado
- **Given** un server que declara soportar desde `1.2.0` y un cliente en `1.2.0`, con el server en `1.4.0`
- **When** arranca una sesión
- **Then** el aviso es informativo, no urgente

## Fuera de alcance

- Auto-actualizar el cliente sin intervención. Decisión explícita: se avisa y el usuario decide.
- Deployar el VPS con un runner de GitHub Actions. Se descartó por no sumar un agente con acceso al
  host; `deploy.yml` queda como está, manual por `workflow_dispatch`.
- Usar el webhook inbound (`/receive`) como disparador del deploy. **Evaluado y descartado**: el MCP
  corre en un container y el deploy toca el host, así que requeriría montarle el socket de docker —
  darle a un container el control de su propio host.
- Rollback de versión del lado del cliente.
