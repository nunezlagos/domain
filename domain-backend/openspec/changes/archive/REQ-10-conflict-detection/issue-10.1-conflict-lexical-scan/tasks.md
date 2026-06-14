# Tasks: issue-10.1-conflict-lexical-scan

## Backend

- [ ] **B1: Crear paquete `internal/conflict/`**
      - `lexical.go` — FindCandidates, findLexicalCandidates
      - `types.go` — ScanOpts, ScanReport, Candidate

- [ ] **B2: Implementar extractSignificantTerms**
      - Normalizar (lowercase, strip punctuation)
      - Tokenizar con strings.Fields
      - Filtrar stop words y tokens < 3 chars

- [ ] **B3: Implementar buildFTSQuery**
      - Tomar top N términos significativos (max 10)
      - Unir con OR para FTS5 MATCH

- [ ] **B4: Implementar findLexicalCandidates**
      - Query FTS5 MATCH + JOIN observations
      - Excluir self-match (o.id != source)
      - Excluir deleted
      - Calcular score normalizado
      - Filtrar por threshold

- [ ] **B5: Implementar insertCandidates**
      - INSERT OR IGNORE en memory_relations
      - sync_id = "lex-{source}-{target}"
      - relation="candidate", judgment_status="pending"
      - Respetar maxInsert

- [ ] **B6: Implementar FindCandidates pipeline**
      - Batch processing (default 100)
      - Time filter con --since
      - Report con estadísticas

- [ ] **B7: Implementar stop words list**
      - Lista custom para contenido técnico: artículos, preposiciones, verbos comunes

## CLI (integración con issue-10.3)

- [ ] **B8: Integrar flags del scan**
      - flags: --dry-run, --apply, --max-insert, --since, --threshold
      - Mapear a ScanOpts

## Tests

- [ ] **T1: FindCandidates encuentra overlap léxico**
- [ ] **T2: No hay self-match**
- [ ] **T3: Dry-run no inserta candidates**
- [ ] **T4: Apply inserta candidates**
- [ ] **T5: MaxInsert limita inserciones**
- [ ] **T6: Since filtra por tiempo**
- [ ] **T7: Threshold filtra low-score**
- [ ] **T8: Report contiene estadísticas correctas**
- [ ] **T9: Sabotaje — no excluir self-match → test T2 falla → restaurar**

## Cierre

- [ ] `go build ./...` sin errores
- [ ] `go vet ./...` sin warnings
- [ ] `go test ./internal/conflict/... -v`
- [ ] Commit: `feat: lexical conflict detection via FTS5 with FindCandidates`
