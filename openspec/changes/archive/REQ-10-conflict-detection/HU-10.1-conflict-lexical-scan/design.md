# Design: HU-10.1-conflict-lexical-scan

## Decisión arquitectónica

### ScanOpts y ScanReport

```go
type ScanOpts struct {
    DryRun    bool          `json:"dry_run"`
    Apply     bool          `json:"apply"`
    MaxInsert int           `json:"max_insert"`  // 0 = unlimited
    Since     time.Duration `json:"since"`        // 0 = all
    Threshold float64       `json:"threshold"`    // default 0.3
    BatchSize int           `json:"batch_size"`   // default 100
}

type ScanReport struct {
    TotalScanned      int     `json:"total_scanned"`
    CandidatesFound   int     `json:"candidates_found"`
    CandidatesInserted int    `json:"candidates_inserted"`
    Duration          string  `json:"duration"`
    DryRun            bool    `json:"dry_run"`
    Threshold         float64 `json:"threshold"`
}
```

### FindCandidates algorithm

```go
func FindCandidates(ctx context.Context, db *sql.DB, opts ScanOpts) (*ScanReport, error) {
    start := time.Now()
    report := &ScanReport{DryRun: opts.DryRun, Threshold: opts.Threshold}

    // Build time filter
    whereClause := "WHERE deleted_at IS NULL"
    args := []interface{}{}
    if opts.Since > 0 {
        whereClause += " AND created_at > datetime('now', ?)"
        args = append(args, fmt.Sprintf("-%d seconds", int(opts.Since.Seconds())))
    }

    // Count total
    db.QueryRowContext(ctx, "SELECT COUNT(*) FROM observations "+whereClause, args...).Scan(&report.TotalScanned)

    // Batch process
    offset := 0
    for {
        rows, err := db.QueryContext(ctx,
            "SELECT id, title, content FROM observations "+whereClause+" LIMIT ? OFFSET ?",
            append(args, opts.BatchSize, offset)...)
        if err != nil { return nil, err }
        defer rows.Close()

        var batch []Observation
        for rows.Next() {
            var o Observation
            rows.Scan(&o.ID, &o.Title, &o.Content)
            batch = append(batch, o)
        }
        if len(batch) == 0 { break }

        for _, obs := range batch {
            candidates, err := findLexicalCandidates(ctx, db, obs, opts.Threshold)
            if err != nil { continue }
            report.CandidatesFound += len(candidates)

            if opts.Apply && !opts.DryRun {
                inserted, err := insertCandidates(ctx, db, obs.ID, candidates, opts.MaxInsert)
                if err != nil { continue }
                report.CandidatesInserted += inserted
            }
        }

        offset += len(batch)
    }

    report.Duration = time.Since(start).Round(time.Millisecond).String()
    return report, nil
}
```

### FTS5 lexical overlap

```go
func findLexicalCandidates(ctx context.Context, db *sql.DB, obs Observation, threshold float64) ([]Candidate, error) {
    // Extraer términos significativos del título y contenido
    terms := extractSignificantTerms(obs.Title + " " + obs.Content)
    if len(terms) == 0 {
        return nil, nil
    }

    // Build FTS5 query: términos unidos con OR, limitados a los más relevantes
    query := buildFTSQuery(terms[:min(len(terms), 10)])

    rows, err := db.QueryContext(ctx,
        `SELECT o.id, o.title, o.content, rank
         FROM observations_fts f
         JOIN observations o ON o.id = f.rowid
         WHERE observations_fts MATCH ?
           AND o.id != ?
           AND o.deleted_at IS NULL
         ORDER BY rank DESC
         LIMIT 20`,
        query, obs.ID)
    if err != nil { return nil, err }
    defer rows.Close()

    var candidates []Candidate
    for rows.Next() {
        var c Candidate
        rows.Scan(&c.TargetID, &c.TargetTitle, &c.TargetContent, &c.Score)
        // Normalizar score a 0-1
        c.Score = normalizeScore(c.Score, len(terms))
        if c.Score >= threshold {
            c.Evidence = "lexical:FTS5"
            candidates = append(candidates, c)
        }
    }
    return candidates, nil
}
```

### Tokenizer and term extraction

```go
// FTS5 unicode61 tokenizer handles:
// - Unicode characters
// - Numbers (separate tokens)
// - camelCase → camel, case (via tokenizer)
// We pre-process to improve matching:
func extractSignificantTerms(text string) []string {
    // Normalize: lowercase, strip punctuation
    normalized := strings.ToLower(text)
    normalized = punctRegex.ReplaceAllString(normalized, " ")

    // Tokenize
    tokens := strings.Fields(normalized)

    // Filter stop words and short tokens
    var significant []string
    for _, t := range tokens {
        if len(t) < 3 { continue }
        if isStopWord(t) { continue }
        significant = append(significant, t)
    }
    return unique(significant)
}
```

### Memory relations insert

```go
type Candidate struct {
    TargetID      int64   `json:"target_id"`
    TargetTitle   string  `json:"target_title"`
    TargetContent string  `json:"target_content"`
    Score         float64 `json:"score"`
    Evidence      string  `json:"evidence"`
}

func insertCandidates(ctx context.Context, db *sql.DB, sourceID int64, candidates []Candidate, maxInsert int) (int, error) {
    inserted := 0
    for i, c := range candidates {
        if maxInsert > 0 && i >= maxInsert { break }
        syncID := fmt.Sprintf("lex-%d-%d", sourceID, c.TargetID)
        _, err := db.ExecContext(ctx,
            `INSERT OR IGNORE INTO memory_relations
             (sync_id, source_id, target_id, relation, judgment_status, confidence, evidence, session_id)
             VALUES (?, ?, ?, 'candidate', 'pending', ?, 'lexical:FTS5', '')`,
            syncID, sourceID, c.TargetID, c.Score)
        if err != nil { return inserted, err }
        inserted++
    }
    return inserted, nil
}
```

## Alternativas descartadas

| Alternativa | Razón de descarte |
|-------------|-------------------|
| Levenshtein directo entre todas las observaciones | O(n²) computacional; FTS5 es más eficiente con índices |
| Embeddings vectoriales | Requiere dependencia externa y modelo; FTS5 es built-in y zero-dependency |
| Sin threshold | Generaría demasiados falsos positivos |

## TDD plan

1. **Red:** FindCandidates encuentra overlap léxico → falla
2. **Green:** Implement FTS5 query + score → pasa
3. **Red:** --dry-run no inserta → falla
4. **Green:** Implement DryRun check → pasa
5. **Red:** --max-insert limita candidates → falla
6. **Green:** Implement max insert logic → pasa
7. **Sabotaje:** No excluir self-match → candidate se apunta a sí mismo → test cae → restaurar

## Riesgos y mitigación

| Riesgo | Mitigación |
|--------|------------|
| FTS5 query lenta con muchas observaciones | Batch processing + LIMIT en candidates query; índice FTS5 es eficiente |
| Stop words inadecuadas para tech content | Lista custom de stop words incluyendo términos técnicos comunes |
| Falsos positivos por términos genéricos | Threshold configurable; semantic judge (HU-10.2) filtra después |
