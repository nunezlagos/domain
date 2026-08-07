# Tasks: enforcement en el wrapper

## Tests (RED primero)

- [ ] Test: un developer invocando una tool de nivel admin recibe error de autorización (REQ-6)
- [ ] Test: la operación rechazada NO llega a leer ni escribir (REQ-6)
- [ ] Test: sin membresía en un proyecto visible, el error dice "sin permiso", no "no existe" (REQ-6)
- [ ] Test: un proyecto personal ajeno responde igual que uno inexistente (REQ-6)
  — un "no tenés permiso" sobre un proyecto invisible CONFIRMA que existe: eso ya es una fuga
- [ ] Test: los caminos internos (hooks, agentes del orquestador, service token ACP) siguen
      funcionando
- [ ] Test: una tool migrada a `withProjectTxHandler` sin `project_slug` falla cerrado

## Código

- [ ] Chequeo de nivel en `ResilientWrapper`, junto al allowlist que ya aplica (`server.go:167`)
- [ ] Orden explícito: primero visibilidad, después nivel
  — invertirlo filtra la existencia de proyectos invisibles
- [ ] Migrar las tools con `project_slug` de `withOrgTxHandler` a `withProjectTxHandler`
  — hoy son 8 de 181 las que usan el wrapper correcto; el resto resuelve el proyecto a mano
- [ ] Exención explícita y acotada para los caminos internos, con su razón escrita
- [ ] Verificar que el rechazo ocurre donde el rollback de la tx está garantizado

## Verify (auditoría — última task)

- [ ] Ninguna función nueva > 50 líneas
- [ ] `go test ./... -count=1` verde
- [ ] Suite de integración verde: es la única que ve el comportamiento real del scope
- [ ] Sabotaje: quitar el chequeo de nivel del wrapper → los tests de REQ-6 en rojo
- [ ] Sabotaje: invertir el orden visibilidad/nivel → el test del proyecto invisible en rojo
- [ ] Medir cuántas tools quedaron bajo `withProjectTxHandler` y compararlo con las que reciben
      `project_slug`: la diferencia es deuda, y va documentada
- [ ] Verificar contra el servidor corriendo que un flujo real de sesión no se rompió

## Documentación

- [ ] CHANGELOG Unreleased
- [ ] `state.yaml` a `implemented`
- [ ] Documentar la exención de los caminos internos: qué se exime y por qué
