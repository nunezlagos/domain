package knowledge

import (
	"github.com/pgvector/pgvector-go"

	"nunezlagos/domain/internal/llm"
)

// embeddingOrNil devuelve nil cuando el vector no es un embedding útil, para que
// la columna quede en NULL y el backfill sepa regenerarla.
//
// DOMAINSERV-157: el NopEmbedder devuelve un slice de N ceros —no vacío— con
// err=nil, así que un chequeo de largo lo deja pasar y queda persistido como si
// fuera legítimo. El CTE de búsqueda filtra por `embedding IS NOT NULL` sin
// mirar la norma, de modo que esos ceros no molestan mientras el embedder está
// roto y pasan a competir con los reales el día que se arregla.
func embeddingOrNil(vec []float32) *pgvector.Vector {
	if len(vec) == 0 || llm.IsZero(vec) {
		return nil
	}
	v := pgvector.NewVector(vec)
	return &v
}
