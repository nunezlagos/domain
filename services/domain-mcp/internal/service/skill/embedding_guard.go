package skill

import (
	"github.com/pgvector/pgvector-go"

	"nunezlagos/domain/internal/llm"
)

// embeddingOrNil devuelve nil cuando el vector no es un embedding útil, para que
// la columna quede en NULL y el backfill sepa regenerarla.
//
// DOMAINSERV-157: skills era el peor de los cuatro caminos de escritura. No
// tenía ningún guard —ni el de largo que sí tenían los chunks— y tampoco estaba
// en backfillTargets, así que sus ceros quedaban para siempre. Son tres los
// puntos de escritura, y por eso el guard vive acá: tres copias de la misma
// condición son tres oportunidades de olvidarse una.
func embeddingOrNil(vec []float32) *pgvector.Vector {
	if len(vec) == 0 || llm.IsZero(vec) {
		return nil
	}
	v := pgvector.NewVector(vec)
	return &v
}
