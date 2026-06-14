# Design: issue-03.6-dedup-privacy

## Decisión arquitectónica

**Dos capas de dedup (app + DB) + privacy stripping pre-insert + rolling window cleanup periódico.**

```
Flujo completo SaveObservation:

input: Observation{title, content, type, project_id, scope}
  │
  1. Privacy stripping
  │   content = StripPrivate(content)  → elimina <private>...</private>
  │   log: "Stripped 2 private blocks"
  │
  2. Normalización
  │   raw = fmt.Sprintf("%s|%s|%s|%s|%s", project_id, scope, type, title, content)
  │   raw = normalize(raw)  → lowercase, trim, collapse spaces
  │   hash = SHA-256(raw)
  │
  3. Rolling window check
  │   if hash exists in observation_hashes (with window):
  │     return ErrDuplicateObservation + original observation
  │
  4. INSERT observation (con hash column UNIQUE)
  │   if unique violation:
  │     return ErrDuplicateObservation (defense in depth)
  │
  5. INSERT observation_hash
  │
  6. Cleanup (every 50 inserts)
  │   DELETE FROM observation_hashes
  │   WHERE id IN (
  │     SELECT id FROM observation_hashes
  │     ORDER BY created_at ASC
  │     OFFSET 1000
  │   )
```

**Tablas:**
```
observations (ADD columna hash)
├── hash  TEXT UNIQUE            -- SHA-256 hex digest

observation_hashes
├── hash           TEXT PRIMARY KEY
├── observation_id UUID NOT NULL REFERENCES observations(id) ON DELETE CASCADE
└── created_at     TIMESTAMPTZ NOT NULL DEFAULT now()
```

**Privacy Stripping:**
```go
var privateRegex = regexp.MustCompile(`(?i)<private>.*?</private>`)

func StripPrivate(content string) (string, int) {
    matches := privateRegex.FindAllString(content, -1)
    cleaned := privateRegex.ReplaceAllString(content, "")
    return cleaned, len(matches)
}
```

## Alternativas descartadas

| Alternativa | Motivo de descarte |
|-------------|-------------------|
| SemHash o SimHash para near-dedup | Más complejo; el caso de uso es duplicado exacto normalizado |
| Solo unique constraint en DB | Mala experiencia (error de PG crudo); mejor detectar antes |
| Sin rolling window | Los hashes crecen indefinidamente |
| Privacy con encriptación | No es necesario; stripping es suficiente para el caso de uso |

## Diagrama

```
Input
  │
  ▼
┌────────────────┐
│ StripPrivate   │ → logs: "2 private blocks stripped"
└───────┬────────┘
        │
        ▼
┌────────────────┐
│ Normalize +    │ → SHA-256
│ Hash           │
└───────┬────────┘
        │
        ▼
┌──────────────────────┐
│ Rolling Window Check │ ── existe ──→ ErrDuplicateObservation
│ (observation_hashes) │
└───────┬──────────────┘
        │ no existe
        ▼
┌──────────────────────┐
│ INSERT observation   │ ── unique violation ──→ ErrDuplicateObservation
│ (con hash UNIQUE)    │                         (defense in depth)
└───────┬──────────────┘
        │ ok
        ▼
┌──────────────────────┐
│ INSERT hash          │
│ + cleanup (c/50)     │
└──────────────────────┘
```

## TDD plan

1. **Red**: Test: hash normalizado igual para "Fix" y "fix  "
2. **Green**: Implementar normalize + hash
3. **Red**: Test: StripPrivate elimina tags
4. **Green**: Implementar StripPrivate
5. **Red**: Test: insertar duplicado → ErrDuplicateObservation
6. **Green**: Implementar dedup check en app
7. **Red**: Test: insertar duplicado bypasseando app → unique constraint violada
8. **Green**: Agregar columna hash UNIQUE en observation
9. **Sabotaje**: modificar hash → unique constraint lo atrapa

## Riesgos y mitigación

| Riesgo | Mitigación |
|--------|------------|
| Privacy en contenido existente | Solo aplica a nuevos inserts; no se hace retroactivo |
| Rolling window muy agresivo | Configurable; default 1000 entradas |
| Hash collision | SHA-256 collision probability ≅ 0 para este volumen |
