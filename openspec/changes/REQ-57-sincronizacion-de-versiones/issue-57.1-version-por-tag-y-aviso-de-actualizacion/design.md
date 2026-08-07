# Design: versión por tag y aviso de actualización

## La idea central

El **tag `v*` es el único evento** que sincroniza los tres artefactos. Hoy cada uno se mueve por su
cuenta; con esto, un solo número de versión los ata.

```
git tag v1.2.0 && git push --tags
        │
        ├── Actions construye y publica el binario   (release-installer.yml — YA existe)
        ├── el cron del VPS ve el tag y despliega    (scripts/deploy.sh — YA existe, con rollback)
        └── el cliente compara y avisa               (SessionStart — YA hace el round-trip)
```

Las tres piezas ya están construidas. Lo que falta es el **número de versión** que las une.

## ADR-1 — Pull, no push

**Decisión:** el cliente pregunta en cada sesión; el server no notifica a nadie.

**Alternativa rechazada:** que el server empuje la actualización. Exigiría que conozca a todos sus
clientes, mantener un registro de instalaciones y un canal de vuelta. El cliente ya llama a
`domain_session_bootstrap` en cada arranque: la versión viaja gratis en ese round-trip.

**Tradeoff:** el aviso llega al iniciar sesión, no en el instante del release. Es exactamente cuando
sirve — nadie actualiza en medio de una tarea.

## ADR-2 — Avisar, nunca auto-actualizar

**Decisión:** el hook informa; actualizar es un comando explícito.

**Por qué:** reemplazar el binario que está corriendo es riesgoso, y el usuario puede estar a mitad
de algo. Una actualización silenciosa que falla deja a alguien sin MCP sin entender por qué.

**Tradeoff:** habrá clientes desactualizados por elección. Se acota con REQ-6: el server declara el
piso de compatibilidad y por debajo el aviso cambia de tono.

## ADR-3 — Cron que pollea, no runner de Actions

**Decisión:** un cron en el VPS compara el último tag `v*` del remoto contra el desplegado.

**Alternativas evaluadas:**

- **Runner self-hosted.** Es la de menos código nuevo: `deploy.yml` y `scripts/deploy.sh` ya están
  escritos y el propio comentario del workflow dice que *"registrar un runner lo habría activado sin
  tocar una línea"*. Se descartó porque suma un agente de GitHub con acceso al host — superficie
  nueva en el único borde HTTP del VPS.
- **Webhook inbound → domain-mcp.** Tentadora: se implementó hoy (DOMAINSERV-240) y ya soporta firma
  de GitHub. **No funciona**: el MCP corre en un container y el deploy toca el host. Requeriría
  montarle el socket de docker, o sea darle a un container el control de su propio host.

**Tradeoff:** la latencia es el intervalo del cron en vez del instante del push. Para un deploy
deliberado por tag, es irrelevante.

## ADR-4 — El disparador es el tag, no el push a main

**Decisión:** `main` no despliega; un tag `v*` sí.

**Por qué:** hoy se commitea directo a `main` sin PR, así que "cada push despliega" pondría en prod
cualquier commit intermedio. Y el tag ya es el disparador de la release del binario: usar el mismo
evento mantiene alineadas la versión del server y la del cliente por construcción.

## ADR-5 — La distribución va por GitHub Releases, no por clone + compile

**Decisión:** el instalador baja el binario publicado; el clone + Go queda como fallback.

**Por qué:** hoy el instalador clona el repo **y compila con Go**, así que exige Go instalado y
varios minutos. Bajar de Releases es **anónimo** —no requiere login ni token— y cumple la
restricción de "usuarios que solo clonan la solución". El fallback se conserva para arquitecturas sin
binario publicado.

## Riesgos

1. **La divergencia se acelera.** Con deploy automático y cliente manual, el server siempre corre lo
   último. Es la consecuencia esperada, no un accidente: por eso REQ-6 (versión mínima soportada) es
   obligatorio y no opcional.
2. **Un cron que despliega solo puede sorprender.** Mitigación: solo tags, `deploy.sh` ya tiene
   rollback, y dos ciclos no se pisan (REQ-5).
3. **El aviso se vuelve ruido.** Si aparece en cada sesión sin que nadie actualice, se ignora.
   Mitigación: el tono escala solo por debajo del mínimo soportado.
4. **Un binario de release corrupto.** Mitigación: la actualización no reemplaza el binario anterior
   hasta que el nuevo esté verificado (REQ-4).

## Orden de implementación

Las fases 0-1 solas resuelven el 80% —saber que estás desactualizado— y son casi gratis. El
auto-deploy va **después**: automatizar antes de tener detección acelera la divergencia sin red.

| Fase | Qué | REQ |
|---|---|---|
| 0 | Versión con `-X` en el binario + expuesta en el bootstrap | 1, 2 |
| 1 | El SessionStart compara y avisa | 3 |
| 2 | El instalador baja de Releases | 4 |
| 3 | Comando de actualización en un paso | 4 |
| 4 | Versión mínima de cliente soportada | 6 |
| 5 | El cron del VPS | 5 |

## TDD plan

- **Red primero** en la comparación de versiones: es lógica pura y se testea sin red ni BD.
- El hook se prueba con el server simulado devolviendo versión mayor, igual, menor y sin responder.
- **Sabotajes:** (a) quitar el `-X` del build → el guard de versión debe fallar; (b) hacer que el
  aviso bloquee la sesión → el escenario de "no bloquea" debe ponerse en rojo; (c) forzar dos ciclos
  del cron solapados → el guard de concurrencia debe atraparlo.
