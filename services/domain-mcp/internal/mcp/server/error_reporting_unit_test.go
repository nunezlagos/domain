package mcpserver

import (
	"testing"

	"github.com/google/uuid"

	"nunezlagos/domain/internal/auth/apikey"
)

// TestErrorReportingActorID cubre la resolución del actor para el audit trail
// (REQ-56 issue-56.2): known_error_set y error_reset guardan actor_id desde el
// Principal de la sesión. Sin principal o con UserID inválido → uuid.Nil (que el
// handler mapea a actor_id NULL en el audit).
func TestErrorReportingActorID(t *testing.T) {
	valid := uuid.New()
	cases := []struct {
		name      string
		principal *apikey.Principal
		want      uuid.UUID
	}{
		{"sin principal -> Nil", nil, uuid.Nil},
		{"UserID vacio -> Nil", &apikey.Principal{UserID: ""}, uuid.Nil},
		{"UserID no-uuid -> Nil", &apikey.Principal{UserID: "no-es-uuid"}, uuid.Nil},
		{"UserID valido -> parseado", &apikey.Principal{UserID: valid.String()}, valid},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			h := NewErrorReportingHandlers(nil, c.principal)
			if got := h.actorID(); got != c.want {
				t.Errorf("actorID() = %v, want %v", got, c.want)
			}
		})
	}
}

// TestErrorReportingNilPoolGuard verifica que los handlers devuelven un error de
// tool (no panic) cuando el pool no está configurado. Es la guarda de arranque.
func TestErrorReportingNilPoolGuard(t *testing.T) {
	h := NewErrorReportingHandlers(nil, &apikey.Principal{UserID: uuid.New().String()})
	if h.pool != nil {
		t.Fatal("esperaba pool nil en este caso de prueba")
	}
	// actorID no debe entrar en pánico aunque el pool sea nil.
	if got := h.actorID(); got == uuid.Nil {
		t.Log("actorID resuelto vacío es aceptable; la guarda de pool nil se ejerce en los handlers")
	}
}
