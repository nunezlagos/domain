# Tasks: administración de miembros

## Tests (RED primero)

- [ ] Test: un manager agrega un developer (REQ-4)
- [ ] Test: un manager NO puede agregar otro manager (REQ-4)
- [ ] Test: un admin NO puede quitar a otro admin; el owner sí (REQ-4)
- [ ] Test: nadie puede cambiar su propio rol hacia arriba (REQ-4)
  — la regla debe evaluarse contra el rol ACTUAL del actor, no contra el objetivo pedido
- [ ] Test: quitar un miembro deja su próxima invocación sin permiso
- [ ] Test: cada cambio de membresía queda en audit, con identificadores y sin emails
- [ ] Test: quitar al último miembro de nivel alto de un proyecto compartido emite advertencia

## Código

- [ ] Tool: agregar miembro con rol
- [ ] Tool: quitar miembro
- [ ] Tool: cambiar el rol de un miembro
- [ ] Tool: listar miembros de un proyecto
- [ ] Las tres de escritura invocan `PuedeAdministrar` de 66.2 — la regla NO se reimplementa
- [ ] Registro en audit de cada cambio, sin PII
- [ ] Registrar las 4 tools en la matriz de 66.3 y en `tool_channels.go`
  — son tres registros distintos en este repo y olvidarse de uno tiene su propio guard

## Verify (auditoría — última task)

- [ ] Ninguna función nueva > 50 líneas
- [ ] `go test ./... -count=1` verde
- [ ] Suite de integración con al menos 3 usuarios de distinto nivel
  — con un solo actor, todo modelo de permisos parece correcto
- [ ] Sabotaje: evaluar la regla contra el rol objetivo en vez del actual → el test de
      auto-promoción en rojo
- [ ] Sabotaje: quitar la llamada a `PuedeAdministrar` → los tests de nivel en rojo
- [ ] Verificar que el audit no contiene emails ni ningún campo de la lista bloqueada

## Documentación

- [ ] CHANGELOG Unreleased
- [ ] `state.yaml` a `implemented`
- [ ] Issue aparte para transferir `owner_id`: es la única operación que puede dejar a alguien sin
      acceso a lo que creó
