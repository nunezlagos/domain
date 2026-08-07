# Tasks: jerarquía de roles por nivel

## Tests (RED primero — es una función pura, no hay excusa para no escribirlos antes)

- [ ] Test: el orden es owner > admin > manager > developer
- [ ] Test: un manager puede administrar a un developer (REQ-4)
- [ ] Test: un manager NO puede administrar a otro manager — nivel igual no alcanza (REQ-4)
- [ ] Test: un admin NO puede administrar a otro admin (REQ-4)
- [ ] Test: el owner puede administrar a todos, incluidos los admin (REQ-4)
- [ ] Test: nadie puede administrarse a sí mismo (REQ-4)
- [ ] Test: un rol desconocido NO puede nada — ni siquiera lo de un developer
  — es el caso central: `users.role` es varchar libre y ya contiene un valor fuera del catálogo
- [ ] Test: string vacío y espacios en blanco se tratan como desconocido

## Código

- [ ] Los cuatro niveles como constantes ordenadas, con espacio numérico entre ellas
- [ ] `PuedeAdministrar(actor, objetivo)` → nivel estrictamente mayor
- [ ] `TieneNivel(actor, requerido)` → nivel mayor o igual
- [ ] Parseo que distingue "rol válido" de "rol irreconocible"; lo irreconocible NO cae a nivel 0

## Verify (auditoría — última task)

- [ ] Ninguna función nueva > 50 líneas
- [ ] `go test ./... -count=1` verde
- [ ] Sabotaje: cambiar `>` por `>=` en `PuedeAdministrar` → los tests de "no puede tocar a su
      igual" en rojo
  — es un solo carácter y cambia el modelo entero; si ningún test lo atrapa, la suite no sirve
- [ ] Sabotaje: hacer que el rol desconocido caiga a nivel 0 → su test en rojo
- [ ] Sin dependencias de base de datos en el paquete: la función es pura y debe poder correr sin
      Postgres

## Documentación

- [ ] CHANGELOG Unreleased
- [ ] `state.yaml` a `implemented`
