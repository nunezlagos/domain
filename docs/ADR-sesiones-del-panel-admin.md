# ADR — El estado de sesión del panel admin

- **Estado**: propuesta, pendiente de decisión. NO implementada.
- **Fecha**: 2026-08-02
- **Ticket**: DOMAINSERV-204 (criterio 3), split de DOMAINSERV-203
- **Alcance**: solo el mecanismo de sesión de `services/domain-admin`. No toca
  la autenticación del MCP ni las API keys.

## Contexto

Hoy, verificado en el código:

- `app/config/settings/base.py:126` —
  `SESSION_ENGINE = "django.contrib.sessions.backends.signed_cookies"`.
- `app/config/views.py` — el login hace `request.session["authenticated"] = True`
  y cada vista lo lee con `request.session.get("authenticated")`.

O sea: **toda la autenticación del panel es un booleano dentro de una cookie
firmada con `SECRET_KEY`**. No hay ninguna fila en ninguna tabla que represente
la sesión. El servidor no sabe quién está adentro.

Consecuencias que NO se arreglan rotando nada:

1. **No se puede revocar una sesión.** La única manera de echar a alguien es
   rotar `SECRET_KEY`, que cierra todas las sesiones y además invalida los
   tokens CSRF en vuelo (los firma la misma clave).
2. **Una sola clave sostiene toda la superficie.** `SECRET_KEY` firma la sesión,
   el CSRF y cualquier otro valor firmado. Una filtración devuelve el bypass
   completo de login que se cerró en `f78eb21c`.
3. **No hay auditoría de acceso.** No existe registro de sesiones activas ni de
   su último uso.

Restricción estructural relevante: los modelos del admin son `managed=False`
contra el Postgres de domain-mcp, y `services/domain-admin` **no tiene ninguna
migración de Django** (verificado: `git ls-files services/domain-admin | grep
migration` no devuelve nada). La tabla `django_session` no existe ni está en
ninguna migración del server (la última es `000278`). Tampoco hay Redis ni
Valkey en el stack.

## Opciones

### A. Dejarlo como está (`signed_cookies`)

- **A favor**: cero trabajo. Sin estado en DB: el panel arranca aunque Postgres
  esté caído. Con `SECRET_KEY` fuera del repo, ya no se puede forjar una sesión
  offline, que era el agujero real de 203.
- **En contra**: se aceptan las tres consecuencias de arriba. La revocación
  sigue siendo "rotar la clave y que se caigan todos".
- **Cuándo alcanza**: panel single-user, un solo operador, sin requisito de
  revocación ni de auditoría. Es exactamente el estado actual.

### B. `django.contrib.sessions.backends.db`

- **A favor**: sesión revocable de a una (borrar la fila). La cookie pasa a ser
  un identificador opaco en vez de portar el estado, así que una filtración de
  `SECRET_KEY` ya no basta para entrar. Habilita "sesiones activas" y
  `SESSION_COOKIE_AGE` real del lado del servidor. Es el default de Django y no
  requiere infraestructura nueva.
- **En contra**: cada request agrega un round-trip a Postgres, y si Postgres se
  cae el panel deja de autenticar (hoy no). Requiere crear `django_session` y
  decidir quién es su dueño — ver la sub-decisión.
- **Costo**: chico en código (una línea de settings), no trivial en ownership de
  schema.

### C. `cached_db` o backend en Redis

- Descartada por ahora: **no hay Redis ni Valkey en el stack**. Sumar un
  servicio nuevo al VPS por el panel admin no se justifica frente a B.

## Sub-decisión de B: quién crea `django_session`

Esta es la parte que realmente hay que decidir, no el `SESSION_ENGINE`.

- **B1. La crea Django** (`INSTALLED_APPS` ya tiene `django.contrib.sessions`,
  así que `migrate sessions` la crea).
  - A favor: es el camino soportado; el schema lo mantiene Django al actualizar.
  - En contra: mete un segundo dueño de DDL en el mismo Postgres que administra
    `internal/migrate` del server. El admin necesitaría poder hacer DDL y correr
    `migrate` en el arranque, y hoy no lo hace. Rompe el invariante de que el
    schema tiene una sola fuente de verdad.
- **B2. La crea una migración del server** (la próxima libre al momento de
  implementarlo), y el admin sigue con `managed=False` como todo lo demás.
  - A favor: una sola fuente de DDL, consistente con el resto del repo.
    Ninguna migración ya aplicada se toca.
  - En contra: el schema de `django_session` queda copiado a mano y hay que
    re-verificarlo si Django lo cambia en un upgrade mayor. Hay que sumarle
    política de RLS explícita o dejarla fuera de RLS a propósito, con su razón.

## Recomendación

**B + B2**, y en un ticket aparte: cambia el modelo de amenaza del panel y
excede el presupuesto de este change (<100 líneas). No se implementa acá.

Mientras no se implemente, lo que aplica es la opción A y sus tres
consecuencias quedan **aceptadas de forma explícita**, no ignoradas.

## Lo que este change SÍ hizo (DOMAINSERV-204, criterio 1)

- `ALLOWED_HOSTS` y `CSRF_TRUSTED_ORIGINS` salen del entorno; ninguna IP de
  deployment queda en `base.py`.
- `SESSION_COOKIE_SECURE` pasó de **inalcanzable** a alcanzable: el settings la
  leía pero el compose nunca la pasaba, así que en prod la variable no existía y
  ponerla en el `.env` no tenía efecto.
- Se agregó `CSRF_COOKIE_SECURE`, que no existía en ningún settings.
- Ambos flags quedan en `0`: prenderlos sin TLS en el origen deja el panel
  inalcanzable. Ver el criterio 2 del ticket, bloqueado por falta de dominio DNS.
