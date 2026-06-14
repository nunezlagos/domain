# Design: issue-29.1-install-cwd-guard

## Contexto

`runInstall` en `cmd/domain/install_cli.go:56` no verifica el cwd antes de
efectuar side effects. Las llamadas peligrosas:

- `upsertEnvFile(".env", ...)` en líneas 566, 585, 719, 758 — path
  relativo al cwd, no absoluto.
- `install.BackupFile(p)` en línea 436 (vía `runBackupsCount`) — también
  relativo al cwd para `.env`.
- `StartDockerServices(...)` en línea 545 — compose files relativos al cwd.

Si el usuario corre `domain install` desde un directorio random (e.g.
`/tmp` o el home), `upsertEnvFile` escribe en el `.env` ajeno. Bug
detectado en sesión 2026-06-12: "domain install sin guard de cwd".

## Decisión arquitectónica

**Estrategia:** detección por default + override explícito.

1. **Default (sin flags):** `runInstall` valida que el cwd contenga
   simultáneamente `.env.example` y `docker-compose.yml`. Si falta alguno
   o ambos, abortar con exit 1 antes de cualquier side effect.
2. **Override con `--src`:** nuevo flag opcional que apunta a un path
   absoluto. Cuando se pasa, el guard valida ese path en vez del cwd.
   Los paths de `.env`, `opencode.json`, etc. se computan relativos al
   `--src`.
3. **Helper testeable:** `install.IsProjectRoot(path string) (bool, []string, error)`
   retorna `(true, nil, nil)` si ambos archivos existen, o
   `(false, []string{".env.example", "docker-compose.yml"}, nil)` con la
   lista de los faltantes. Error solo si el path no es accesible.

## Alternativas descartadas

| Alt | Idea | Por qué se descarta |
|-----|------|---------------------|
| A | Asumir siempre cwd=repo (status quo) | El bug que estamos arreglando. |
| B | Flag `--src` requerido, sin default | Rompe UX: el caso común (instalador en su propio repo) requiere tipear path extra. |
| C | Buscar hacia arriba hasta encontrar `.env.example` (walk up) | Puede encontrar `.env.example` de un repo ANCESTRO no relacionado. Frágil y confuso. |
| D | Variable de entorno `DOMAIN_PROJECT_ROOT` | Menos descubrible que un flag; choca con el patrón `--flag` del resto del CLI. |

## Por qué C (default + override) gana

- Costo de implementación bajo: 1 helper + 1 guard + 1 flag nuevo.
- UX: zero-config en el caso feliz (98% de los installs), escape hatch
  explícito cuando hace falta.
- Testeable: helper puro no toca FS real en los unit tests (mock con
  `os.Stat` o `fstest.MapFS`).
- No rompe instalaciones existentes: el guard solo aborta si FALTAN los
  archivos — un install dentro del repo pasa transparente.

## Riesgos

- **R1:** Si en el futuro el repo se renombra o se mueve, el guard
  podría romperse. Mitigación: el helper retorna la lista de archivos
  faltantes, no strings hardcodeados en el mensaje.
- **R2:** Tests que asumen cwd=repo podrían romperse. Mitigación:
  actualizar tests para usar `t.Chdir(testRepoRoot)` o setear
  `DOMAIN_PROJECT_ROOT` como variable de test.

## Compatibilidad

- Backward compatible: installs dentro del repo siguen funcionando igual.
- Forward compatible: `--src` queda disponible para CI / Dockerfiles
  donde el binario corre desde un dir distinto al repo montado.

## Sabotaje test (referencia, ver tasks.md para el detalle)

Romper el guard (comentar la verificación de `.env.example` +
`docker-compose.yml`) → el test del Escenario 1 (sabotaje explícito) debe
caer → restaurar el guard → test verde.
