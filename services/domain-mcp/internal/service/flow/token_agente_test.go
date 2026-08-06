package flow

import (
	"encoding/base64"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

// cuerpoFirmadoDe extrae el JSON que la firma cubre, que es lo que este test compara.
// Mirar el payload deserializado NO alcanza: un campo agregado sin omitempty aparece en el
// JSON aunque su valor sea el cero, y ahí es donde se rompe la compatibilidad.
func cuerpoFirmadoDe(t *testing.T, token string) string {
	t.Helper()
	raw, err := base64.RawURLEncoding.DecodeString(token)
	require.NoError(t, err)
	idx := strings.LastIndexByte(string(raw), '.')
	require.GreaterOrEqual(t, idx, 0)
	return string(raw[:idx])
}

// DOMAINSERV-218 REQ-218.6. El hilo principal no manda agent_id, y su token tiene que
// seguir siendo el de antes de este cambio. Es el invariante que hace seguro tocar el gate
// que autoriza mis propias ediciones: si lo rompo, me quedo sin poder editar para arreglarlo.
func TestFlowTokenService_GenerateToken_SinAgente_CuerpoFirmadoNoMencionaAlAgente(t *testing.T) {
	s := NewFlowTokenService([]byte("secret-key"))

	token, err := s.GenerateToken("flow-1", "sess-1", "org-1")
	require.NoError(t, err)

	cuerpo := cuerpoFirmadoDe(t, token)
	require.NotContains(t, cuerpo, `"a"`,
		"un grant sin agente agregó el campo del agente al cuerpo firmado: el token del hilo principal dejó de ser el de antes")
}

func TestFlowTokenService_GenerateTokenParaAgente_RoundTrip_ConservaElAgente(t *testing.T) {
	s := NewFlowTokenService([]byte("secret-key"))

	token, err := s.GenerateTokenParaAgente("flow-1", "sess-1", "org-1", "agente-A", []string{"services/**"})
	require.NoError(t, err)

	payload, err := s.ValidateToken(token)
	require.NoError(t, err)
	require.Equal(t, "agente-A", payload.AgentID)
	require.Equal(t, []string{"services/**"}, payload.AllowedPaths)
}

// El agente va DENTRO de la firma: si estuviera fuera, cualquiera podría reescribirlo y el
// deny por mismatch no valdría nada.
func TestFlowTokenService_ValidateToken_AgenteAlteradoEnElCuerpo_RetornaErrInvalid(t *testing.T) {
	s := NewFlowTokenService([]byte("secret-key"))

	token, err := s.GenerateTokenParaAgente("flow-1", "sess-1", "org-1", "agente-A", nil)
	require.NoError(t, err)

	raw, err := base64.RawURLEncoding.DecodeString(token)
	require.NoError(t, err)
	alterado := strings.Replace(string(raw), `"agente-A"`, `"agente-B"`, 1)
	require.NotEqual(t, string(raw), alterado, "el agente no aparece en el cuerpo, el test no está probando nada")

	_, err = s.ValidateToken(base64.RawURLEncoding.EncodeToString([]byte(alterado)))
	require.ErrorIs(t, err, ErrTokenInvalid)
}

func TestFlowTokenService_GenerateTokenParaAgente_AgenteVacio_EquivaleAUnGrantSinAgente(t *testing.T) {
	s := NewFlowTokenService([]byte("secret-key"))

	conVacio, err := s.GenerateTokenParaAgente("flow-1", "sess-1", "org-1", "", nil)
	require.NoError(t, err)

	// se compara la AUSENCIA del campo y no el cuerpo entero contra otro token: ExpiresAt
	// tiene resolución de un segundo, así que dos generaciones consecutivas difieren en el
	// borde y el test sería flaky
	require.NotContains(t, cuerpoFirmadoDe(t, conVacio), `"a"`,
		"pasar agente vacío agregó el campo: el fallback del incremento 2 emitiría un token distinto al del hilo principal")
}
