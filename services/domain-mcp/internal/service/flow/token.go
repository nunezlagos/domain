package flow

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"time"
)

const FlowTokenTTL = 30 * time.Minute

var (
	ErrTokenNotConfigured = errors.New("flow token: HMAC secret not configured (set DOMAIN_FLOW_TOKEN_SECRET)")
	ErrTokenInvalid       = errors.New("flow token: invalid signature")
	ErrTokenExpired       = errors.New("flow token: expired")
)

type FlowTokenPayload struct {
	FlowRunID string `json:"f"`
	SessionID string `json:"s"`
	OrgID     string `json:"o"`
	ExpiresAt int64  `json:"e"`
	// AllowedPaths (DOMAINSERV-110): batch-mode. Si no está vacío, el gate
	// pre-edit solo autoriza ediciones cuyo path matchee uno de estos globs
	// (scope por sub-tarea en multiagent paralelo). Vacío = sin restricción de
	// path (comportamiento histórico, backward-compatible).
	AllowedPaths []string `json:"p,omitempty"`
	// AgentID (DOMAINSERV-218): a QUIÉN autoriza este token. El SessionID no
	// alcanza porque se hereda del padre, así que N subagentes de una sesión
	// compartían un único allowed_paths y no había forma de denegarle a uno el
	// territorio de otro.
	//
	// omitempty NO es cosmético: sin él, un grant sin agente agregaría `"a":""` al
	// cuerpo firmado y el token del hilo principal dejaría de ser el de antes de
	// este cambio. Ese es el invariante que hace seguro tocar el gate que autoriza
	// las propias ediciones.
	AgentID string `json:"a,omitempty"`
}

type FlowTokenService struct {
	secret []byte
	ttl    time.Duration
}

func NewFlowTokenService(secret []byte) *FlowTokenService {
	return &FlowTokenService{
		secret: secret,
		ttl:    FlowTokenTTL,
	}
}

func (s *FlowTokenService) IsConfigured() bool {
	return len(s.secret) > 0
}

func (s *FlowTokenService) GenerateToken(flowRunID, sessionID, orgID string, allowedPaths ...string) (string, error) {
	return s.GenerateTokenParaAgente(flowRunID, sessionID, orgID, "", allowedPaths)
}

// GenerateTokenParaAgente emite un token atado a UN agente (DOMAINSERV-218). Se agrega como
// constructor hermano y no como parámetro de GenerateToken porque su último parámetro es
// variádico: un posicional nuevo rompería a todos los callers, incluidos los tests verdes.
//
// agentID vacío produce exactamente el token de antes de este cambio, que es lo que mantiene
// autorizado al hilo principal y al subagente que todavía no tiene token propio.
func (s *FlowTokenService) GenerateTokenParaAgente(flowRunID, sessionID, orgID, agentID string, allowedPaths []string) (string, error) {
	if !s.IsConfigured() {
		return "", ErrTokenNotConfigured
	}

	payload := FlowTokenPayload{
		FlowRunID:    flowRunID,
		SessionID:    sessionID,
		OrgID:        orgID,
		ExpiresAt:    time.Now().UTC().Add(s.ttl).Unix(),
		AllowedPaths: allowedPaths,
		AgentID:      agentID,
	}

	body, err := json.Marshal(payload)
	if err != nil {
		return "", fmt.Errorf("flow token: marshal payload: %w", err)
	}

	mac := hmac.New(sha256.New, s.secret)
	mac.Write(body)
	sig := mac.Sum(nil)

	token := make([]byte, 0, len(body)+1+base64.RawURLEncoding.EncodedLen(len(sig)))
	token = append(token, body...)
	token = append(token, '.')
	token = append(token, base64.RawURLEncoding.EncodeToString(sig)...)

	return base64.RawURLEncoding.EncodeToString(token), nil
}

func (s *FlowTokenService) ValidateToken(encoded string) (*FlowTokenPayload, error) {
	if !s.IsConfigured() {
		return nil, ErrTokenNotConfigured
	}

	raw, err := base64.RawURLEncoding.DecodeString(encoded)
	if err != nil {
		return nil, fmt.Errorf("flow token: base64: %w", err)
	}

	// separador = ÚLTIMO '.': el body es JSON y puede contener '.' (ej. paths con
	// extensión en AllowedPaths, DOMAINSERV-110); la firma es base64url (sin '.'),
	// así que el último '.' delimita body|sig sin ambigüedad.
	idx := -1
	for i, b := range raw {
		if b == '.' {
			idx = i
		}
	}
	if idx < 0 {
		return nil, ErrTokenInvalid
	}

	body := raw[:idx]
	sigB64 := string(raw[idx+1:])

	sig, err := base64.RawURLEncoding.DecodeString(sigB64)
	if err != nil {
		return nil, fmt.Errorf("flow token: decode sig: %w", err)
	}

	mac := hmac.New(sha256.New, s.secret)
	mac.Write(body)
	expected := mac.Sum(nil)

	if !hmac.Equal(sig, expected) {
		return nil, ErrTokenInvalid
	}

	var payload FlowTokenPayload
	if err := json.Unmarshal(body, &payload); err != nil {
		return nil, fmt.Errorf("flow token: unmarshal payload: %w", err)
	}

	if payload.ExpiresAt < time.Now().UTC().Unix() {
		return nil, ErrTokenExpired
	}

	return &payload, nil
}
