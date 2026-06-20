package domain

import (
	"errors"
	"fmt"
)

// Sentinels para errores comunes. Se exponen como variables para que callers
// puedan usar errors.Is(err, ErrUnauthorized).
var (
	ErrUnauthorized = errors.New("unauthorized")
	ErrNotFound     = errors.New("not found")
	ErrConflict     = errors.New("conflict")
	ErrRateLimited  = errors.New("rate limited")
	ErrValidation   = errors.New("validation failed")
	ErrQuota        = errors.New("quota exceeded")
)

// ErrorDetail describe un error de validación a nivel campo.
type ErrorDetail struct {
	Field   string `json:"field,omitempty"`
	Code    string `json:"code,omitempty"`
	Message string `json:"message,omitempty"`
}

// APIError es el shape de un error devuelto por el API. Implementa Unwrap
// para integrarse con errors.Is/As contra los sentinels.
type APIError struct {
	Code       string
	Message    string
	StatusCode int
	RequestID  string
	RetryAfter int
	Details    []ErrorDetail

	sentinel error
}

func (e *APIError) Error() string {
	if e.Code != "" {
		return fmt.Sprintf("domain api: %s (%s, status=%d, request_id=%s)",
			e.Message, e.Code, e.StatusCode, e.RequestID)
	}
	return fmt.Sprintf("domain api: %s (status=%d, request_id=%s)",
		e.Message, e.StatusCode, e.RequestID)
}

// Unwrap devuelve el sentinel para que errors.Is funcione.
func (e *APIError) Unwrap() error { return e.sentinel }

// IsUnauthorized reporta si el error proviene de 401/403.
func IsUnauthorized(err error) bool { return errors.Is(err, ErrUnauthorized) }

// IsNotFound reporta si el error proviene de 404.
func IsNotFound(err error) bool { return errors.Is(err, ErrNotFound) }

// IsConflict reporta si el error proviene de 409.
func IsConflict(err error) bool { return errors.Is(err, ErrConflict) }

// IsRateLimited reporta si el error proviene de 429.
func IsRateLimited(err error) bool { return errors.Is(err, ErrRateLimited) }

// IsValidation reporta si el error proviene de 422.
func IsValidation(err error) bool { return errors.Is(err, ErrValidation) }

// IsQuotaExceeded reporta si el error proviene de 402.
func IsQuotaExceeded(err error) bool { return errors.Is(err, ErrQuota) }

// AsAPIError extrae *APIError de err si existe. Conveniencia sobre errors.As.
func AsAPIError(err error) (*APIError, bool) {
	var apiErr *APIError
	if errors.As(err, &apiErr) {
		return apiErr, true
	}
	return nil, false
}
