# issue-29.2-backup-deduplication-by-hash

**Origen:** `REQ-29-install-quick-fixes`
**Prioridad tentativa:** alta
**Tipo:** fix

## Historia de usuario

**Como** developer corriendo `domain install` o `domain update` varias veces al día
**Quiero** que el sistema NO cree un nuevo `.bak.<timestamp>` cuando el archivo no cambió desde el último backup
**Para** evitar el spam de 60+ archivos backup idénticos que vi esta semana en el repo y en `~/.config/domain/`

## Criterios de aceptación

### Escenario 1: Sin cambios, un solo backup

```gherkin
Dado que el archivo `.env` ya tiene un backup `.env.bak.<ts_old>` con hash `H`
Y el `.env` actual tiene el mismo contenido (hash `H`)
Cuando corro `domain install`
Entonces NO se crea un nuevo `.env.bak.<ts_new>`
Y el último backup sigue siendo `.env.bak.<ts_old>`
```

### Escenario 2: Con cambios, nuevo backup

```gherkin
Dado que el archivo `.env` ya tiene un backup `.env.bak.<ts_old>` con hash `H_old`
Y el `.env` actual tiene contenido distinto (hash `H_new` != `H_old`)
Cuando corro `domain install`
Entonces SÍ se crea un nuevo `.env.bak.<ts_new>` con el contenido actual
```

### Escenario 3: Aplica a todos los archivos backup-eables

```gherkin
Dado que `install.BackupFile()` se invoca para `.env`, `opencode.json`, `.mcp.json`, `credentials.json`
Cuando todos esos archivos NO cambiaron desde su último backup
Y corro 10 veces seguidas `domain install`
Entonces el directorio tiene exactamente 1 `.bak.*` por archivo (no 10)
```

### Escenario 4: Sabotaje — sin dedup, 10 corridas = 10 backups

```gherkin
Dado que el archivo `.env` tiene contenido `X` y un backup previo con `X`
Cuando corro 10 veces `install.BackupFile(".env")` SIN la dedup
Entonces el directorio contiene 11 archivos `.env.bak.*` (1 previo + 10 nuevos)
Cuando aplico la dedup
Entonces el directorio contiene 1 archivo `.env.bak.*` (o 2 si se cuentan los
previos a la dedup, según estrategia)
```

### Escenario 5: Edge case — primer backup (no hay previo)

```gherkin
Dado que el archivo `.env` existe pero NO tiene ningún `.bak.*` previo
Cuando corro `install.BackupFile(".env")`
Entonces SÍ se crea `.env.bak.<ts>` (es el primero, no hay con qué comparar)
```

### Escenario 6: Edge case — keepLast=0 (sin prune) sigue funcionando

```gherkin
Dado que `BackupFile` se invoca con `keepLast=0` (caso usado para AGENTS.md injection)
Cuando el archivo cambia
Entonces SÍ se crea el nuevo backup (la dedup no aplica a este caller porque siempre inyecta)
Y NO se borran los backups viejos (keepLast=0 = no prune)
```

## Notas

- El cómputo de hash usa SHA-256 (función `install.FileChecksum` ya existe
  en `internal/cli/install/backup.go:259`).
- La dedup es OPT-IN por caller: las funciones `BackupCredentials`,
  `BackupEnv`, `BackupOpenCodeConfig` la activan. `BackupFile` (helper
  genérico para AGENTS.md injection) NO la activa (porque esos archivos
  son read-then-write y el contenido siempre cambia intencionalmente).
- Performance: leer el archivo + hashear es O(N) en bytes. Para un `.env`
  típico (~1KB) y `opencode.json` (~5KB) es despreciable. No requiere
  índice en DB ni cache en memoria.
