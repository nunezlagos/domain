// Package observation — HU-28.1 Repository interface.
//
// Repository abstrae el acceso a la tabla observations + queries de búsqueda
// híbrida. Service depende de esta interfaz (no de *pgxpool.Pool directo),
// lo cual permite unit-testear lógica de negocio con mocks.
//
// La implementación concreta vive en pg_repository.go y honra la tx-context
// (issue-25.14): si el ctx trae una tx inyectada por el middleware HTTP, las
// queries corren contra esa tx (RLS activa). Si no, fallback al pool.
package observation

import (
	"context"
	"time"

	"github.com/google/uuid"
)

// Repository es el contrato de persistencia para observations.
//
// Los métodos Save/Get/List/SoftDelete/SearchHybrid envuelven las queries SQL
// que antes vivían inline en service.go. SearchInput agrupa los params de
// SearchHybrid para no explotar la firma.
type Repository interface {
	// Insert persiste una observation nueva. content_hash + UNIQUE constraint
	// dispara ErrDuplicate desde el caller cuando el constraint
	// observations_dedup_hash_uniq se viola.
	Insert(ctx context.Context, in InsertParams) (*Observation, error)

	// Get retorna la observation por id. ErrNotFound si no existe o está
	// soft-deleted.
	Get(ctx context.Context, id uuid.UUID) (*Observation, error)

	// List devuelve las observations del project, más recientes primero.
	List(ctx context.Context, projectID uuid.UUID, limit int) ([]Observation, error)

	// ListPaginated implementa keyset pagination estable por (created_at, id).
	// El bool de retorno es hasMore (true si quedan más rows).
	ListPaginated(ctx context.Context, in ListPageInput) ([]Observation, bool, error)

	// SoftDelete marca deleted_at. Retorna ErrNotFound si ya estaba borrada.
	SoftDelete(ctx context.Context, id uuid.UUID) error

	// SearchHybrid combina BM25 + cosine con RRF. Si useVector es false,
	// degrada a tsvector-only.
	SearchHybrid(ctx context.Context, in SearchInput) ([]SearchResult, error)
}

// InsertParams agrupa todos los campos requeridos por el INSERT, ya
// pre-procesados por el Service (privacy strip, embedding, hash).
type InsertParams struct {
	OrganizationID  uuid.UUID
	ProjectID       uuid.UUID
	CreatedBy       *uuid.UUID
	SessionID       *uuid.UUID
	Content         string
	EmbeddingLit    string // literal '[v1,v2,...]' ya formateado
	ObservationType string
	Tags            []string
	MetadataJSON    []byte
	ContentHash     []byte
}

// SearchInput agrupa params del SearchHybrid.
type SearchInput struct {
	OrgID        uuid.UUID
	Query        string
	EmbeddingLit string // '[v1,v2,...]' o "" si UseVector=false
	UseVector    bool
	Limit        int
	Candidates   int // por modalidad
	RRFK         int
}

// statelessFields documentación: la pasa de Time a *Time se gestiona en Scan.
var _ = time.Time{}
