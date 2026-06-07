# Tasks: HU-14.3-cli-autocomplete-help

## Backend

- [ ] Implementar `completion` subcomando con bash/zsh/fish/powershell
- [ ] Customizar help template con ejemplos y formato consistente
- [ ] Agregar `Example` field a todos los comandos y subcomandos
- [ ] Implementar `man` subcomando con GenMan
- [ ] Implementar `SuggestFor` con Levenshtein distance para comandos
- [ ] Implementar Levenshtein distance para flags desconocidos
- [ ] Agregar flag `--version` con version + commit sha (inyectado en build)
- [ ] Agregar flag `--verbose` con debug info (config, request duration)
- [ ] Implementar `domain config view` para ver config actual

## Frontend

- [ ] N/A (CLI tool)

## Tests

- [ ] Test unitario: Levenshtein distance correcta
- [ ] Test unitario: Sugerencia para comando typo
- [ ] Test unitario: Sugerencia para flag typo
- [ ] Test de integración: `--help` output compara con golden file
- [ ] Test de integración: `completion bash` genera script válido
- [ ] Test de integración: `man` genera output válido
- [ ] Test de integración: `--version` muestra versión
- [ ] Sabotaje: sacar Example de un comando → test detecta

## Cierre

- [ ] Verificación manual: `domain --help`, `domain memory save --help`
- [ ] Verificación manual: `source <(domain completion bash)` y probar Tab
- [ ] Suite verde: `go test ./cmd/domain/...`
- [ ] Documentar instalación de completion en README
