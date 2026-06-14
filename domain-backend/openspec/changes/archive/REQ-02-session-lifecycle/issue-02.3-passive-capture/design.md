# Design: issue-02.3-passive-capture

## Decisión arquitectónica

### Parseo línea por línea con máquina de estados

Elegimos un parser stateful en lugar de regex global por:
1. **Legibilidad** — el flujo es explícito: BUSCANDO → DENTRO → FIN
2. **Robustez** — fácil ignorar code blocks, manejar anidación, etc.
3. **Performance** — O(n) con un solo scan lineal

Máquina de estados:

```
ESTADOS: searching, inSection, inCodeBlock

searching:
  - "^```" → inCodeBlock
  - "^## Key Learnings:" → inSection
  - otro → seguir searching

inCodeBlock:
  - "^```" → searching
  - otro → seguir inCodeBlock

inSection:
  - "^```" → inCodeBlock (dentro de sección)
  - "^## " → searching (nueva sección, salir)
  - match item → recolectar
  - línea vacía → seguir inSection
  - otro → seguir inSection
```

### Formato de items soportados

```
- item                          → bullet
- [x] item                      → checklist checked
- [ ] item                      → checklist unchecked
1. item                         → numbered
1) item                         → numbered paren
* item                          → asterisk bullet
```

Todos se normalizan al mismo formato: el texto después del prefijo, trim.

### Dedup: query por contenido exacto

```go
func (s *ObservationStore) ExistsByContent(ctx context.Context, sessionID, content string) (bool, error) {
    var count int
    err := s.db.QueryRowContext(ctx,
        `SELECT COUNT(*) FROM observations WHERE content = ? AND session_id = ? AND deleted_at IS NULL`,
        content, sessionID,
    ).Scan(&count)
    return count > 0, err
}
```

Usamos búsqueda exacta (no FTS5) porque es más rápida y el dedup es por igualdad literal. Si en el futuro queremos "casi duplicados", se agrega con un threshold de Levenshtein.

### Integración con ObservationStore

`CapturePassive` recibe un `*ObservationStore` (o interfaz mínima) y llama a `AddObservation` por cada item nuevo. El caller es responsable de crear la sesión antes de llamar.

```go
type observationWriter interface {
    AddObservation(ctx context.Context, o Observation) (int64, error)
    ExistsByContent(ctx context.Context, sessionID, content string) (bool, error)
}

func CapturePassive(store observationWriter, ctx context.Context, sessionID, text string) (int, error)
```

## Alternativas descartadas

| Alternativa | Razón de descarte |
|-------------|-------------------|
| Regex global `(?m)^## Key Learnings:\s*\n((?:^[-\*\d].+\n?)+)` | No permite ignorar code blocks; difícil de debuggear; captura context no deseado |
| Parser Markdown completo (gomarkdown) | Dependencia pesada para una feature pequeña; nuestro formato es muy específico |
| Dedup por hash SHA-256 del contenido | Overkill; comparación exacta de string es suficientemente rápida y más simple |
| Captura automática en cada tool call | El agente decide cuándo llamar a `domain_mem_capture_passive` explícitamente |

## Diagrama

```
texto
  │
  ▼
┌──────────────────┐
│  ExtractLearnings│
│  (state machine) │
└──────┬───────────┘
       │
       ├─ no "## Key Learnings:" → ErrNoLearningsSection
       ├─ sección vacía → ErrEmptyLearningsSection
       └─ items →
            ▼
┌───────────────────────────────┐
│  CapturePassive               │
│  for each item:               │
│    ¿existe por contenido?     │
│    ├─ sí → skip               │
│    └─ no → AddObservation     │
│         type="learning"       │
│         scope="session"       │
└───────────────────────────────┘
       │
       ▼
  retorna count de items nuevos
```

## TDD plan

1. **Red:** `TestExtractBullets` — texto con bullets → 3 items → falla
2. **Green:** Implementar `ExtractLearnings` mínimo → pasa
3. **Red:** `TestExtractNumbered` — items numerados → parse correcto → falla
4. **Green:** Agregar soporte para `\d+\. ` → pasa
5. **Red:** `TestExtractChecklist` — `- [x]` items → prefijo limpiado → falla
6. **Green:** Agregar soporte bullet/checklist → pasa
7. **Red:** `TestExtractNoSection` — texto sin sección → error → falla
8. **Green:** Detectar ausencia de sección → pasa
9. **Red:** `TestExtractEmptySection` — `## Key Learnings:` sin items → error → falla
10. **Green:** Si no hay items → error → pasa
11. **Red:** `TestCapturePassiveDedup` — mismo texto dos veces → 3 luego 0 → falla
12. **Green:** Implementar `ExistsByContent` + skip → pasa
13. **Red:** `TestExtractIgnoreCodeBlock` — sección dentro de ``` → ignorada → falla
14. **Green:** Estado inCodeBlock → pasa
15. **Red:** `TestExtractMultipleSections` — dos secciones → items combinados → falla
16. **Green:** Resetear estado en cada sección → pasa
17. **Sabotaje:** Items con solo espacios → no deben crear observaciones → romper trim → test cae → restaurar

## Riesgos y mitigación

| Riesgo | Mitigación |
|--------|------------|
| Falsos positivos con "## Key Learnings:" en comentarios de código | Ignorar dentro de code blocks (```); si aparece en línea regular sin code block, es intencional |
| Item con contenido vacío después del prefijo | Trim y validar longitud > 0 antes de agregar; si vacío, skip con log debug |
| SessionID inválido pasado a CapturePassive | El store layer rechazará por FK constraint; error claro para debugging |
| Texto muy grande (>1MB) | No hay límite explícito; el parseo es O(n) y no usa regex complejos; si hay riesgo, agregar límite en caller |
