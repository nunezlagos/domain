package main

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"
)

// TestCompareToGolden_SinDrift verifica que el modo prod-safe (--expected-schema)
// no reporta drift cuando el dump real coincide con el golden versionado, SIN
// crear DB temporal ni requerir CREATE DATABASE. DOMAINSERV-105.
func TestCompareToGolden_SinDrift(t *testing.T) {
	schema := "r public.foo id integer NOT NULL\nr public.foo name text\n"
	golden := filepath.Join(t.TempDir(), "schema.golden")
	require.NoError(t, os.WriteFile(golden, []byte(normalize(schema)), 0o644))

	diff, err := compareToGolden(schema, golden)
	require.NoError(t, err)
	require.Empty(t, diff, "schema idéntico al golden no debe reportar drift")
}

// TestCompareToGolden_DetectaDrift verifica que una diferencia real prod↔golden
// se detecta (columna que el golden espera y el schema real no tiene).
func TestCompareToGolden_DetectaDrift(t *testing.T) {
	golden := filepath.Join(t.TempDir(), "schema.golden")
	require.NoError(t, os.WriteFile(golden,
		[]byte("r public.foo id integer NOT NULL\nr public.foo name text\nr public.foo extra boolean\n"), 0o644))

	realSchema := "r public.foo id integer NOT NULL\nr public.foo name text\n"
	diff, err := compareToGolden(realSchema, golden)
	require.NoError(t, err)
	require.NotEmpty(t, diff, "columna faltante respecto al golden debe reportar drift")
	require.Contains(t, diff, "extra")
}

// TestCompareToGolden_GoldenInexistente_Error verifica fail-closed: un golden
// ausente es error explícito, nunca un falso OK.
func TestCompareToGolden_GoldenInexistente_Error(t *testing.T) {
	_, err := compareToGolden("r public.foo id integer", filepath.Join(t.TempDir(), "no-existe.golden"))
	require.Error(t, err, "golden ausente debe ser error, no falso OK")
}
