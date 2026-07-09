package openspec

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// R3: el parser debe aceptar tanto '## Scenario:' (H2) como '#### Scenario:' (H4),
// y Given/When/Then plano, con bullet '- ', y con bullet+bold '- **Given**'.

func TestParseScenarios_H4BulletsBold_Parsea(t *testing.T) {
	md := "# Feature X\n\n" +
		"#### Scenario: caso bold\n" +
		"- **Given** una precondición\n" +
		"- **When** una acción\n" +
		"- **Then** un resultado\n"
	got := ParseScenarios(md)
	require.Len(t, got, 1)
	assert.Equal(t, "caso bold", got[0].Scenario)
	assert.Equal(t, []string{"una precondición"}, got[0].Given)
	assert.Equal(t, "una acción", got[0].When)
	assert.Equal(t, []string{"un resultado"}, got[0].Then)
}

func TestParseScenarios_H2Plano_Parsea(t *testing.T) {
	md := "# Feature Y\n\n" +
		"## Scenario: caso plano\n" +
		"Given una precondición\n" +
		"When una acción\n" +
		"Then un resultado\n"
	got := ParseScenarios(md)
	require.Len(t, got, 1)
	assert.Equal(t, "caso plano", got[0].Scenario)
	assert.Equal(t, []string{"una precondición"}, got[0].Given)
	assert.Equal(t, "una acción", got[0].When)
	assert.Equal(t, []string{"un resultado"}, got[0].Then)
}

func TestParseScenarios_H4BulletSinBold_Parsea(t *testing.T) {
	md := "# Feature Z\n\n" +
		"#### Scenario: caso bullet simple\n" +
		"- Given precondición\n" +
		"- When acción\n" +
		"- Then resultado\n"
	got := ParseScenarios(md)
	require.Len(t, got, 1)
	assert.Equal(t, "caso bullet simple", got[0].Scenario)
	assert.Equal(t, []string{"precondición"}, got[0].Given)
	assert.Equal(t, "acción", got[0].When)
	assert.Equal(t, []string{"resultado"}, got[0].Then)
}

func TestParseScenarios_MultiplesVariantesJuntas(t *testing.T) {
	md := "# Feature Mix\n\n" +
		"## Scenario: uno\n- Given a\n- When b\n- Then c\n\n" +
		"#### Scenario: dos\n- **Given** d\n- **When** e\n- **Then** f\n"
	got := ParseScenarios(md)
	require.Len(t, got, 2)
	assert.Equal(t, "uno", got[0].Scenario)
	assert.Equal(t, "dos", got[1].Scenario)
	assert.Equal(t, []string{"d"}, got[1].Given)
}
