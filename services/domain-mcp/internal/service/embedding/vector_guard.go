package embedding

import (
	"github.com/pgvector/pgvector-go"

	"nunezlagos/domain/internal/llm"
)

// VectorOrNil devuelve nil cuando el vector no es un embedding útil, para que la
// columna quede en NULL y el barrido de pendientes sepa regenerarla.
//
// DOMAINSERV-157: el NopEmbedder devuelve un slice de N ceros —no vacío— con err=nil,
// así que un chequeo de largo lo deja pasar y queda persistido como si fuera legítimo.
// El CTE de búsqueda filtra por `embedding IS NOT NULL` sin mirar la norma, de modo que
// esos ceros no molestan mientras el embedder está roto y pasan a competir con los
// reales el día que se arregla.
//
// Vivía en internal/service/knowledge como embeddingOrNil. Al diferir el embedding
// (DOMAINSERV-227) el insert de chunks dejó de escribir vectores y esa copia se quedó
// sin llamador de producción: solo la llamaba su propio test. El invariante se mudó acá,
// donde la escritura realmente ocurre.
func VectorOrNil(vec []float32) *pgvector.Vector {
	if len(vec) == 0 || llm.IsZero(vec) {
		return nil
	}
	v := pgvector.NewVector(vec)
	return &v
}
