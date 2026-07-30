package phases

import (
	"context"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

// DOMAINSERV-161: con project_policy_list proyectada sin body_md, el gate solo puede
// evaluar de verdad si hace fan-out de policy_get por slug. Y un verdict 'compliant'
// emitido sin haber contrastado nada tiene que dejar de pasar: era el agujero que hacía
// del gate un sello de goma.

func TestSDDReview_Build_OrdenaFanOutDePolicyGet(t *testing.T) {
	h := &sddReviewHandler{}
	out, err := h.Build(context.Background(), Input{RawText: "un cambio cualquiera"})
	require.NoError(t, err)

	require.Contains(t, out.UserPrompt, "domain_policy_get",
		"sin fan-out el agente evalúa contra slugs sin cuerpo")
	require.Contains(t, out.RequiredToolCalls, "domain_policy_get",
		"si no es contrato verificable, vuelve a ser una tool huérfana instruida en prosa")
	require.Contains(t, out.RequiredToolCalls, "domain_project_policy_list")
}

func TestSDDReview_Validate_VerdictViolations_BloqueaElAvance(t *testing.T) {
	h := &sddReviewHandler{}
	err := h.Validate(context.Background(), nil, ClientResult{Output: map[string]any{
		"verdict":         "violations_found",
		"policies_checked": 11,
	}})
	require.ErrorIs(t, err, ErrPolicyReviewFailed)
}

func TestSDDReview_Validate_CompliantConPoliciesEvaluadas_Pasa(t *testing.T) {
	h := &sddReviewHandler{}
	err := h.Validate(context.Background(), nil, ClientResult{Output: map[string]any{
		"verdict":          "compliant",
		"policies_checked": 11,
	}})
	require.NoError(t, err)
}

// el falso compliant: el agente dice que todo cumple sin haber mirado ninguna policy
func TestSDDReview_Validate_CompliantSinPoliciesEvaluadas_NoPasa(t *testing.T) {
	h := &sddReviewHandler{}

	for _, caso := range []struct {
		nombre string
		output map[string]any
	}{
		{"cero explícito", map[string]any{"verdict": "compliant", "policies_checked": 0}},
		{"campo ausente", map[string]any{"verdict": "compliant"}},
		{"float cero", map[string]any{"verdict": "compliant", "policies_checked": float64(0)}},
	} {
		err := h.Validate(context.Background(), nil, ClientResult{Output: caso.output})
		require.Errorf(t, err, "caso %q: un compliant sin policies evaluadas debe rechazarse", caso.nombre)
		require.Containsf(t, strings.ToLower(err.Error()), "policies_checked", "caso %q", caso.nombre)
	}
}

// json.Unmarshal entrega números como float64: si el chequeo solo aceptara int, el gate
// rechazaría reviews legítimos venidos del cliente real
func TestSDDReview_Validate_PoliciesCheckedComoFloat_SeAcepta(t *testing.T) {
	h := &sddReviewHandler{}
	err := h.Validate(context.Background(), nil, ClientResult{Output: map[string]any{
		"verdict":          "compliant",
		"policies_checked": float64(7),
	}})
	require.NoError(t, err)
}

func TestSDDReview_Validate_VerdictDesconocido_NoPasa(t *testing.T) {
	h := &sddReviewHandler{}
	err := h.Validate(context.Background(), nil, ClientResult{Output: map[string]any{"verdict": "casi"}})
	require.Error(t, err)
}
