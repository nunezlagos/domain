# Proposal: issue-10.1-conflict-lexical-scan

## Intención

Implementar FindCandidates, un algoritmo que usa FTS5 para detectar observaciones con contenido léxicamente similar, candidateándolas como potenciales conflictos/duplicados en la tabla memory_relations. Control de ejecución via flags.

## Scope

**Incluye:**
- `FindCandidates(ctx, db, opts ScanOpts) (*ScanReport, error)`
- FTS5 query que busca overlapping lexical content entre observaciones
- Escritura de candidates en memory_relations con relation="candidate"
- Flags: --dry-run (no write), --apply (write), --max-insert (limit), --since (time filter)
- ScanReport con estadísticas
- FTS5 tokenizer config para camelCase, snake_case, números

**No incluye:**
- Semantic judge (issue-10.2)
- CLI conflicts (issue-10.3)
- Search annotations (issue-10.4)

## Enfoque técnico

| Aspecto | Decisión |
|---------|----------|
| Algoritmo | Por cada observation, FTS5 MATCH query con términos relevantes; candidates con score > threshold |
| Threshold | Configurable; default 0.3 (normalized overlap score) |
| Tokenizer | FTS5 unicode61 tokenizer (maneja Unicode, números, símbolos) |
| Score | Número de tokens compartidos / total de tokens únicos en ambas |
| Flags | struct ScanOpts pasado al scan; CLI flags mapean a struct |
| Paginación | Procesar en batches de 100 observaciones para no saturar memoria |

