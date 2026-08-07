# Tasks: matriz tool→nivel con guard de cobertura

## Tests (RED primero)

- [ ] Test: una tool en el registry sin nivel declarado rompe la suite, nombrándola (REQ-5)
- [ ] Test: un nivel declarado para una tool que ya no existe rompe la suite
  — el guard mira en las DOS direcciones, como el de cobertura de ci-shell-guards.yml
- [ ] Test: el catálogo global (policies de plataforma, skills globales, marcos de compliance) es
      legible por un `developer` (REQ-5)
  — es el error más fácil de cometer y el que más duele: dejaría a un developer sin ver las reglas
    que lo gobiernan
- [ ] Test: ninguna tool sin noción de proyecto quedó como "pendiente de decidir" (REQ-5)

## Código

- [ ] Mapa explícito tool → nivel requerido para las 181 tools
  — explícito y NO por convención de nombre: una tool mal nombrada quedaría mal autorizada en
    silencio
- [ ] Guard de cobertura calcado de `TestAllToolsHaveChannel`, leyendo el registry REAL
- [ ] Clasificar las ~40 tools sin `project_slug` en una de tres categorías, con su razón escrita:
  - [ ] global-lectura (ej. `health`)
  - [ ] global-admin (ej. `cron_crud`, `client`)
  - [ ] necesita eje de proyecto — declarado como deuda, con ticket
- [ ] Comentario de cabecera que explique el criterio, como el de `tool_channels.go`

## Verify (auditoría — última task)

- [ ] `go test ./... -count=1` verde
- [ ] Sabotaje: agregar una tool al registry sin nivel → el guard en rojo nombrándola
- [ ] Sabotaje: renombrar una tool sin tocar la matriz → el guard en rojo por el lado inverso
- [ ] Revisión manual de los niveles de escritura y borrado: ante la duda, el nivel MÁS ALTO
  — un nivel de más rompe un flujo y se nota; uno de menos abre un agujero y no se nota
- [ ] La matriz cubre 181 tools, no 166: verificar contra el conteo real del registry

## Documentación

- [ ] CHANGELOG Unreleased
- [ ] `state.yaml` a `implemented`
- [ ] Ticket por cada tool de la categoría "necesita eje de proyecto"
