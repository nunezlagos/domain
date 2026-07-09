package phases

import (
	"context"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// R5-B: sdd-spec.Validate debe acumular TODOS los campos faltantes en un solo
// error, no retornar al primero.

func TestSddSpecValidate_MultiplesCamposFaltantes_Juntos(t *testing.T) {
	h := &sddSpecHandler{}
	// Output vacío: faltan issue_slug Y issue_md.
	err := h.Validate(context.Background(), nil, ClientResult{Output: map[string]any{}})
	require.Error(t, err)
	msg := err.Error()
	assert.Contains(t, msg, "issue_slug", "debe listar issue_slug")
	assert.Contains(t, msg, "issue_md", "debe listar issue_md en el MISMO error")
}

func TestSddSpecValidate_UnCampoFaltante(t *testing.T) {
	h := &sddSpecHandler{}
	// Solo falta issue_md.
	err := h.Validate(context.Background(), nil, ClientResult{
		Output: map[string]any{"issue_slug": "issue-x"},
	})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "issue_md")
	assert.NotContains(t, err.Error(), "issue_slug", "issue_slug estaba presente, no debe listarse")
}

func TestSddSpecValidate_Completo_SinError(t *testing.T) {
	h := &sddSpecHandler{}
	err := h.Validate(context.Background(), nil, ClientResult{
		Output: map[string]any{"issue_slug": "issue-x", "issue_md": "# spec"},
	})
	assert.NoError(t, err)
}

func TestSddSpecValidate_ListaTodosSeparados(t *testing.T) {
	h := &sddSpecHandler{}
	err := h.Validate(context.Background(), nil, ClientResult{Output: map[string]any{}})
	require.Error(t, err)
	// Deben aparecer ambos separados por coma (un solo mensaje agregado).
	assert.True(t, strings.Contains(err.Error(), ","), "los faltantes van juntos, separados por coma")
}
