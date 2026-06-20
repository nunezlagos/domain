// Package ctxkeys define las keys tipadas que viajan en el context de un
// request autenticado. Centralizado acá para evitar drift entre middleware
// que inyecta y handler que extrae (HU-28.3).
//
// Convención: las keys son tipos no exportados; los accesores (Get/Set)
// se exportan así nadie puede leer/escribir las keys "a mano" y rompernos
// el contrato.
package ctxkeys

import (
	"context"

	"github.com/google/uuid"
)

// orgIDKey y userIDKey son tipos privados para evitar colisiones con
// otros packages que usen context.WithValue.
type orgIDKey struct{}
type userIDKey struct{}

// WithOrgID inyecta el OrgID tipado en el context.
func WithOrgID(ctx context.Context, id uuid.UUID) context.Context {
	return context.WithValue(ctx, orgIDKey{}, id)
}

// OrgID extrae el OrgID inyectado por el middleware principal.
// Retorna uuid.Nil si el middleware no corrió o el principal no era válido.
// Los handlers deben tratar uuid.Nil como "no autenticado" (el middleware
// apikey ya bloquea ese caso, pero defense-in-depth).
func OrgID(ctx context.Context) uuid.UUID {
	id, _ := ctx.Value(orgIDKey{}).(uuid.UUID)
	return id
}

// WithUserID inyecta el UserID tipado en el context.
func WithUserID(ctx context.Context, id uuid.UUID) context.Context {
	return context.WithValue(ctx, userIDKey{}, id)
}

// UserID extrae el UserID inyectado por el middleware principal.
// Retorna uuid.Nil si no fue seteado.
func UserID(ctx context.Context) uuid.UUID {
	id, _ := ctx.Value(userIDKey{}).(uuid.UUID)
	return id
}
