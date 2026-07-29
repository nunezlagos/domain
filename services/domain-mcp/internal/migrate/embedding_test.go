package migrate

import (
	"os"
	"regexp"
	"strconv"
	"testing"

	"github.com/stretchr/testify/require"
)

// La dimensión vivía duplicada en llm.DefaultDim (1536) y en cmd/domain
// (embeddingDim, 1024): la migración 000275 movió el esquema y solo una de las dos
// la siguió, así que todo INSERT de embedding falló con "expected 1024 dimensions,
// not 1536". warnIfSchemaDimDiffers ya cubre esa deriva, pero necesita una base
// viva; este test la detecta en la suite -short, que es la que alguien corre.
func TestEmbeddingDim_ContraLaMigracion000275_Coinciden(t *testing.T) {
	const path = "migrations/000275_embedding_dim_1024.up.sql"
	sql, err := os.ReadFile(path)
	require.NoError(t, err, "la migración que fija la dimensión tiene que existir: si se renombró, este guard queda ciego")

	m := regexp.MustCompile(`target_dim\s+CONSTANT\s+int\s*:=\s*(\d+)`).FindSubmatch(sql)
	require.Len(t, m, 2, "no se pudo leer target_dim de %s; el guard depende de esa declaración", path)

	dimMigracion, err := strconv.Atoi(string(m[1]))
	require.NoError(t, err)
	require.Equal(t, dimMigracion, EmbeddingDim,
		"EmbeddingDim no coincide con la migración: el binario escribiría vectores que la columna rechaza")
}
