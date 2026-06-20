// Package llm — helpers pgvector para cosine similarity (issue-06.5).
package llm

import "fmt"

// VectorLiteral convierte []float32 a literal pgvector '[v1,v2,...]'.
func VectorLiteral(v []float32) string {
	if len(v) == 0 {
		return "'[]'::vector"
	}
	buf := make([]byte, 0, len(v)*6+4) // estimación: ~6 chars por float
	buf = append(buf, '\'')
	buf = append(buf, '[')
	for i, f := range v {
		if i > 0 {
			buf = append(buf, ',')
		}
		buf = fmt.Appendf(buf, "%.6f", f)
	}
	buf = append(buf, ']')
	buf = append(buf, '\'')
	buf = append(buf, ':')
	buf = append(buf, ':')
	buf = append(buf, 'v')
	buf = append(buf, 'e')
	buf = append(buf, 'c')
	buf = append(buf, 't')
	buf = append(buf, 'o')
	buf = append(buf, 'r')
	return string(buf)
}

// CosineSimilaritySQL retorna fragmento SQL para ordenar por cosine similarity.
// Usa el operador <=> de pgvector. El placeholder debe ser un vector literal.
func CosineSimilaritySQL(placeholder string) string {
	return fmt.Sprintf("1 - (embedding <=> %s::vector) AS similarity", placeholder)
}

// CosineSimilarityOrder retorna ORDER BY clause para cosine similarity DESC.
func CosineSimilarityOrder(placeholder string) string {
	return fmt.Sprintf("ORDER BY embedding <=> %s::vector ASC", placeholder)
}
