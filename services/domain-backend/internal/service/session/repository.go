// Package session — HU-28.1 Repository interface.
//
// Repository abstrae acceso a la tabla sessions. Service depende de esta
// interfaz, no de *pgxpool.Pool directo, lo cual permite unit-testear lógica
// de negocio con mocks.
package session

import (
	"context"
	"time"

	"github.com/google/uuid"
)

// Repository es el contrato de persistencia para sessions.
//
// End() tiene una particularidad: requiere lock pesimista (FOR UPDATE) +
// transacción propia si no hay tx-context. La implementación PG maneja eso
// internamente; los callers (Service) solo invocan EndAndLoad y reciben la
// session ya actualizada.
type Repository interface {
	Insert(ctx context.Context, in InsertParams) (*Session, error)

	// GetByID retorna la session sin importar status.
	GetByID(ctx context.Context, id uuid.UUID) (*Session, error)

	// GetActive devuelve la session activa más reciente del user en el
	// project. Si projectID == uuid.Nil filtra solo por user.
	GetActive(ctx context.Context, userID, projectID uuid.UUID) (*Session, error)

	// List devuelve sessions del user, más recientes primero.
	List(ctx context.Context, userID uuid.UUID, limit int) ([]Session, error)

	// EndAndLoad cierra la session bajo lock pesimista (FOR UPDATE). Retorna
	// ErrNotFound / ErrAlreadyEnded sin necesidad de que el caller las
	// interprete por sí mismo.
	EndAndLoad(ctx context.Context, id uuid.UUID, summary string, endedAt time.Time) (*Session, error)

	// CloseInactive cierra sesiones con updated_at < cutoff y retorna IDs.
	// Usado por cron leader.
	CloseInactive(ctx context.Context, cutoff time.Time, now time.Time) ([]uuid.UUID, error)
}

// InsertParams agrupa los campos del INSERT.
type InsertParams struct {
	OrganizationID uuid.UUID
	ProjectID      *uuid.UUID
	UserID         uuid.UUID
	Title          string
	Tags           []string
}
