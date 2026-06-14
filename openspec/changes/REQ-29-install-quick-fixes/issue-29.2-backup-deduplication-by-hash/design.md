# Design: issue-29.2-backup-deduplication-by-hash

## Contexto

`backupFile` en `internal/cli/install/backup.go:50` SIEMPRE escribe un
nuevo `.bak.<RFC3339>`, sin comparar con el último backup existente.
Cada corrida de `domain install` o `domain update` genera 1 `.bak` por
archivo (`.env`, `opencode.json`, `credentials.json`), aunque el
contenido no haya cambiado. Resultado observado en sesión 2026-06-12:
**60+ archivos `.env.bak.*` en una semana** de uso.

El helper `FileChecksum` (línea 259) ya existe y calcula SHA-256 de un
archivo — solo falta la lógica de comparación.

## Decisión arquitectónica

**Estrategia:** dedup por hash del último backup existente.

1. Antes de `os.WriteFile(backupPath, data, 0o600)` (línea 60), listar
   los backups previos con `ListBackups(originalPath)` (función
   existente, línea 245) y tomar el más reciente (último elemento del
   slice ya ordenado lexicograficamente).
2. Calcular SHA-256 del backup más reciente y compararlo con SHA-256
   del contenido actual (`data`).
3. Si coinciden: skip (no escribir nuevo backup). Retornar
   `*BackupResult` con `Path`/`Bytes` populated pero `Backup` apuntando
   al backup previo (para que el caller sepa que fue dedupeado).
4. Si difieren (o no hay backup previo): escribir normalmente.

## Alternativas descartadas

| Alt | Idea | Por qué se descarta |
|-----|------|---------------------|
| A | Hardlink en vez de copiar (`os.Link`) | No funciona cross-filesystem; el `.env` local y `~/.config/opencode/opencode.json` suelen estar en filesystems distintos. |
| B | Comparar mtime en vez de hash | mtime cambia con cualquier touch; un `git pull` no cambia mtime del `.env` no-trackeado. Hash es la verdad. |
| C | Cachear último hash en SQLite o sidecar file (`.bak.hash`) | Complica: hay que limpiarlo, sincronizarlo, manejar el caso de que se borre el `.bak` a mano. Re-derivar del FS es trivial y siempre correcto. |
| D | Dedupe en DB (`backups` table con UNIQUE constraint) | Refactor enorme. La doctrina F5 ya dice que .md van a DB, pero los .env/opencode.json son archivos de config LOCALES del user — seguir en FS es correcto. |

## Por qué A (dedup por hash del último backup) gana

- **Implementación trivial:** ~10 líneas en `backupFile`. Cero
  infraestructura nueva.
- **Correcta por construcción:** lee el último backup del FS, que es la
  fuente de verdad. No hay race conditions ni caches que invalidar.
- **Performance aceptable:** 1 read + 1 hash por archivo backup-eable
  por install. Para 3 archivos de ≤10KB = microsegundos.
- **Observable:** retornamos `Backup` apuntando al archivo previo, así
  el caller puede loggear "skipped dedup" o "wrote new backup".

## Detalle de implementación

```go
// Pseudo-Go para la nueva lógica en backupFile(path, keepLast):
data := readFile(path)
if data == nil && isNotExist { return nil, nil }  // skip silencioso

// Dedup: si el último backup tiene el mismo hash, no escribas.
if matches, prev := lastBackupMatchesHash(path, data); matches {
    return &BackupResult{Path: path, Backup: prev, Bytes: int64(len(data)), Deduplicated: true}, nil
}

ts := time.Now().UTC().Format(...)
backupPath := path + ".bak." + ts
os.WriteFile(backupPath, data, 0o600)
... prune si keepLast > 0 ...
return &BackupResult{...}, nil
```

Nuevo struct field `Deduplicated bool` en `BackupResult` para que el
caller pueda distinguir "no backup porque no hubo cambios" de "escribí
uno nuevo".

## Riesgos

- **R1:** Si el último backup se borró a mano, perdemos la dedup
  histórica (se crea uno nuevo). **Aceptable:** es lo correcto, no
  podemos dedupear contra un archivo borrado.
- **R2:** Performance con archivos grandes. **Mitigación:** el spec
  aplica solo a 4 archivos conocidos (`.env`, `opencode.json`, `.mcp.json`,
  `credentials.json`), todos chicos. Si en el futuro se agregan más,
  el caller decide si opt-in o no.

## Sabotaje test (referencia)

Romper la dedup (comentar la rama `if matches { return dedup }`) →
correr el test de "10 corridas sin cambios" → DEBE ver 10 archivos
`.bak.*` → restaurar dedup → test verde.
