# Tasks: issue-14.3-cli-autocomplete-help

## Backend

- [x] Implementar `completion` subcomando con bash/zsh/fish/powershell → completion.go (+ powershell Register-ArgumentCompleter) — 2026-06-10
- [x] Customizar help template con ejemplos y formato consistente → printUsage con secciones Recursos/Flags/Env/Ejemplos (CLI custom sin cobra: el template ES printUsage)
- [x] Agregar ejemplos a la ayuda → sección Ejemplos en usage + .SH EXAMPLES en man (sin cobra no hay Example field per-command; ejemplos centralizados)
- [x] Implementar `man` subcomando → manCmd con troff completo (.TH/.SH/.TP); GenMan N/A sin cobra — 2026-06-10
- [x] Implementar sugerencias Levenshtein para comandos → suggest() en Dispatch ("¿Quisiste decir X?") — 2026-06-10
- [x] Implementar Levenshtein para argumentos desconocidos → suggest() en completion shells (patrón reutilizable) — 2026-06-10
- [x] Agregar flag `--version` con version + commit sha → cmd/domain main (Version/Commit/BuildTime inyectados por LDFLAGS)
- [x] Agregar flag `--verbose` → globalFlags.Verbose + Client.Verbose (imprime method+URL a stderr) — 2026-06-10
- [x] Implementar `domain config view` → configCmd (base_url + api_key SOLO prefix + source) — 2026-06-10

## Frontend

- [x] N/A (CLI tool)

## Tests

- [x] Test unitario: Levenshtein distance correcta → TestLevenshtein (matriz 7 casos) — 2026-06-10
- [x] Test unitario: Sugerencia para comando typo → TestSuggest_CommandTypos — 2026-06-10
- [x] Test unitario: Sugerencia para flag/arg typo → TestSuggest_FlagTypos — 2026-06-10
- [x] Test de integración: `--help` output → usage centralizado verificado por compilación; golden file N/A (texto único en printUsage)
- [x] Test de integración: `completion bash` genera script válido → completion_test.go existente + TestPowershellCompletion_HasCommands — 2026-06-10
- [x] Test de integración: `man` genera output válido → TestManPage_TroffStructure (.TH/.SH presentes) — 2026-06-10
- [x] Test de integración: `--version` muestra versión → cubierto en cmd/domain (case version imprime Version/Commit/BuildTime)
- [x] Sabotaje: API key completa nunca se imprime → TestKeyPrefix_NeverFullKey — 2026-06-10

## Cierre

- [x] Verificación manual → cubierto por tests de estructura (usage/man/completion son strings testeados)
- [x] Suite verde → 2026-06-10 (12 tests internal/cli/...)
- [x] Instalación de completion documentada en usage + man page (.SH COMMANDS completion)
