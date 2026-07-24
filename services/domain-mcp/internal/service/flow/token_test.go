package flow

import (
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestFlowTokenService_GenerateValidate_RoundTrip_DevuelvePayload(t *testing.T) {
	s := NewFlowTokenService([]byte("secret-key"))

	tok, err := s.GenerateToken("flow-1", "sess-1", "org-1")
	require.NoError(t, err)

	p, err := s.ValidateToken(tok)
	require.NoError(t, err)
	require.Equal(t, "flow-1", p.FlowRunID)
	require.Equal(t, "sess-1", p.SessionID)
	require.Equal(t, "org-1", p.OrgID)
}

func TestFlowTokenService_GenerateValidate_AllowedPaths_RoundTrip(t *testing.T) {
	s := NewFlowTokenService([]byte("secret-key"))

	tok, err := s.GenerateToken("flow-1", "sess-1", "org-1", "services/domain-mcp/**", "docs/*.md")
	require.NoError(t, err)

	p, err := s.ValidateToken(tok)
	require.NoError(t, err)
	require.Equal(t, []string{"services/domain-mcp/**", "docs/*.md"}, p.AllowedPaths)
}

func TestFlowTokenService_GenerateToken_SinPaths_AllowedPathsVacio_BackwardCompat(t *testing.T) {
	s := NewFlowTokenService([]byte("secret-key"))

	tok, err := s.GenerateToken("flow-1", "sess-1", "org-1")
	require.NoError(t, err)

	p, err := s.ValidateToken(tok)
	require.NoError(t, err)
	require.Empty(t, p.AllowedPaths, "sin paths declarados, AllowedPaths debe venir vacío (gate sin restricción de path)")
}

func TestFlowTokenService_ValidateToken_FirmaAlterada_RetornaErrInvalid(t *testing.T) {
	s := NewFlowTokenService([]byte("secret-key"))
	tok, err := s.GenerateToken("flow-1", "sess-1", "org-1")
	require.NoError(t, err)

	// validar con OTRO secret: la firma HMAC no coincide
	other := NewFlowTokenService([]byte("otra-key"))
	_, err = other.ValidateToken(tok)
	require.ErrorIs(t, err, ErrTokenInvalid)
}

func TestFlowTokenService_ValidateToken_Expirado_RetornaErrExpired(t *testing.T) {
	// ttl negativo → el token nace expirado (firma válida, expiry vencido)
	s := &FlowTokenService{secret: []byte("secret-key"), ttl: -time.Minute}
	tok, err := s.GenerateToken("flow-1", "sess-1", "org-1")
	require.NoError(t, err)

	_, err = s.ValidateToken(tok)
	require.ErrorIs(t, err, ErrTokenExpired)
}

func TestFlowTokenService_SinSecret_RetornaErrNotConfigured(t *testing.T) {
	s := NewFlowTokenService(nil)
	require.False(t, s.IsConfigured())

	_, err := s.GenerateToken("f", "s", "o")
	require.ErrorIs(t, err, ErrTokenNotConfigured)

	_, err = s.ValidateToken("cualquier-cosa")
	require.ErrorIs(t, err, ErrTokenNotConfigured)
}
