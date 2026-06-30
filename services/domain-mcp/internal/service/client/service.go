// Package client — service.go: lógica de negocio CRUD + validaciones + audit.
//
// REQ-28.1: Service depende de Repository (interfaz) en vez de *pgxpool.Pool.
// El campo Pool se mantiene público como deprecated para Strangler Fig
// (callers que construyen &Service{Pool: ...} siguen funcionando — el helper
// repository() inicializa pgRepository on-demand desde Pool).
package client

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"regexp"
	"strings"

	"github.com/google/uuid"
	"github.com/jackc/pgerrcode"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"

	"nunezlagos/domain/internal/audit"
	"nunezlagos/domain/internal/auth/rut"
)

// reSlug — kebab-case: minúsculas + dígitos + guiones, sin guión inicial/final,
// 2..100 chars. Mismo criterio que internal/service/project.
var reSlug = regexp.MustCompile(`^[a-z0-9]+(-[a-z0-9]+)*$`)

// reEmail — validación basic (no RFC 5322 full). Suficiente como guard.
// Pattern: <local>@<host>.<tld>, con al menos un punto en host.
var reEmail = regexp.MustCompile(`^[^\s@]+@[^\s@]+\.[^\s@]+$`)

// reTaxIDFallback — si el package rut no acepta el input, intentamos un
// regex laxo XX.XXX.XXX-Y para soportar tax IDs no-chilenos básicos.
// La validación primaria sigue siendo rut.Validate (módulo 11).
var reTaxIDFallback = regexp.MustCompile(`^[0-9A-Za-z.\-]{4,50}$`)

// validStatus mapea los valores enum permitidos.
var validStatus = map[string]struct{}{
	StatusActive:   {},
	StatusInactive: {},
	StatusArchived: {},
}

// Service expone las operaciones de negocio sobre clients.
type Service struct {


	Pool  *pgxpool.Pool
	Audit audit.Recorder



	repo Repository
}

// NewService construye el Service con dependencias explícitas. Si repo es nil,
// se construye un pgRepository wrappeando pool (back-compat).
func NewService(pool *pgxpool.Pool, rec audit.Recorder, repo Repository) *Service {
	if repo == nil && pool != nil {
		repo = NewPgRepository(pool)
	}
	return &Service{Pool: pool, Audit: rec, repo: repo}
}

// repository retorna la Repository inyectada o crea una pgRepository on-demand.
func (s *Service) repository() Repository {
	if s.repo != nil {
		return s.repo
	}
	s.repo = NewPgRepository(s.Pool)
	return s.repo
}

// CreateInput agrupa params del Create (Service-level, sin JSON serialization).
type CreateInput struct {
	Name         string
	Slug         string
	TaxID        string
	ContactEmail string
	ContactPhone string
	Address      string
	Metadata     map[string]any
	Status       string // default "active"
	ActorID      *uuid.UUID
}

// UpdateInput el caller patch-style: campos nil = no tocar.
type UpdateInput struct {
	Name         *string
	TaxID        *string
	ContactEmail *string
	ContactPhone *string
	Address      *string
	Metadata     map[string]any // nil = no tocar; non-nil = reemplazar completo
	Status       *string
	ActorID      *uuid.UUID
}

// Create valida + persiste + audita.
func (s *Service) Create(ctx context.Context, orgID uuid.UUID, in CreateInput) (*Client, error) {
	name := strings.TrimSpace(in.Name)
	if len(name) < 2 {
		return nil, ErrInvalidName
	}
	slug := strings.TrimSpace(strings.ToLower(in.Slug))
	if !reSlug.MatchString(slug) || len(slug) < 2 || len(slug) > 100 {
		return nil, ErrInvalidSlug
	}
	if in.Status == "" {
		in.Status = StatusActive
	}
	if _, ok := validStatus[in.Status]; !ok {
		return nil, ErrInvalidStatus
	}
	taxID, err := normalizeTaxID(in.TaxID)
	if err != nil {
		return nil, err
	}
	email := strings.TrimSpace(in.ContactEmail)
	if email != "" && !reEmail.MatchString(email) {
		return nil, ErrInvalidEmail
	}
	if in.Metadata == nil {
		in.Metadata = map[string]any{}
	}
	metaJSON, _ := json.Marshal(in.Metadata)

	c, err := s.repository().Insert(ctx, InsertParams{
		Name:           name,
		Slug:           slug,
		TaxID:          taxID,
		ContactEmail:   email,
		ContactPhone:   strings.TrimSpace(in.ContactPhone),
		Address:        strings.TrimSpace(in.Address),
		MetadataJSON:   metaJSON,
		Status:         in.Status,
	})
	if err != nil {
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) && pgErr.Code == pgerrcode.UniqueViolation {
			return nil, ErrClientSlugExists
		}
		return nil, fmt.Errorf("insert client: %w", err)
	}
	s.audit(ctx, orgID, in.ActorID, "client.created", c.ID, nil, map[string]any{
		"name":   c.Name,
		"slug":   c.Slug,
		"status": c.Status,
	})
	return c, nil
}

// Get resuelve por id (UUID) o por slug (caller pasa idOrSlug, el Service
// decide cuál usar parseando UUID).
func (s *Service) Get(ctx context.Context, orgID uuid.UUID, idOrSlug string) (*Client, error) {
	idOrSlug = strings.TrimSpace(idOrSlug)
	if id, err := uuid.Parse(idOrSlug); err == nil {
		return s.repository().GetByID(ctx, orgID, id)
	}
	return s.repository().GetBySlug(ctx, orgID, idOrSlug)
}

// List delega al repo con limit clamp.
func (s *Service) List(ctx context.Context, orgID uuid.UUID, filter ListFilter) ([]*Client, int64, error) {
	if filter.Limit <= 0 || filter.Limit > 200 {
		filter.Limit = 50
	}
	if filter.Offset < 0 {
		filter.Offset = 0
	}
	if filter.Status != "" {
		if _, ok := validStatus[filter.Status]; !ok {
			return nil, 0, ErrInvalidStatus
		}
	}
	return s.repository().List(ctx, orgID, filter)
}

// Update parchea campos. Recarga prev para mergear, valida, y delega al repo.
func (s *Service) Update(ctx context.Context, orgID uuid.UUID, id uuid.UUID, upd UpdateInput) (*Client, error) {
	prev, err := s.repository().GetByID(ctx, orgID, id)
	if err != nil {
		return nil, err
	}

	name := prev.Name
	if upd.Name != nil {
		name = strings.TrimSpace(*upd.Name)
		if len(name) < 2 {
			return nil, ErrInvalidName
		}
	}
	status := prev.Status
	if upd.Status != nil {
		status = *upd.Status
		if _, ok := validStatus[status]; !ok {
			return nil, ErrInvalidStatus
		}
	}
	taxID := prev.TaxID
	if upd.TaxID != nil {
		t, err := normalizeTaxID(*upd.TaxID)
		if err != nil {
			return nil, err
		}
		taxID = t
	}
	email := prev.ContactEmail
	if upd.ContactEmail != nil {
		email = strings.TrimSpace(*upd.ContactEmail)
		if email != "" && !reEmail.MatchString(email) {
			return nil, ErrInvalidEmail
		}
	}
	phone := prev.ContactPhone
	if upd.ContactPhone != nil {
		phone = strings.TrimSpace(*upd.ContactPhone)
	}
	address := prev.Address
	if upd.Address != nil {
		address = strings.TrimSpace(*upd.Address)
	}
	metadata := prev.Metadata
	if upd.Metadata != nil {
		metadata = upd.Metadata
	}
	if metadata == nil {
		metadata = map[string]any{}
	}
	metaJSON, _ := json.Marshal(metadata)

	c, err := s.repository().Update(ctx, orgID, id, UpdateParams{
		Name:         name,
		TaxID:        taxID,
		ContactEmail: email,
		ContactPhone: phone,
		Address:      address,
		MetadataJSON: metaJSON,
		Status:       status,
	})
	if err != nil {
		return nil, err
	}
	s.audit(ctx, orgID, upd.ActorID, "client.updated", c.ID,
		map[string]any{"name": prev.Name, "status": prev.Status},
		map[string]any{"name": c.Name, "status": c.Status},
	)
	return c, nil
}

// Delete = soft delete.
func (s *Service) Delete(ctx context.Context, orgID uuid.UUID, id uuid.UUID) error {
	if err := s.repository().SoftDelete(ctx, orgID, id); err != nil {
		return err
	}
	s.audit(ctx, orgID, nil, "client.deleted", id, nil, nil)
	return nil
}

// Restore limpia deleted_at.
func (s *Service) Restore(ctx context.Context, orgID uuid.UUID, id uuid.UUID) error {
	if err := s.repository().Restore(ctx, orgID, id); err != nil {
		return err
	}
	s.audit(ctx, orgID, nil, "client.restored", id, nil, nil)
	return nil
}

// SetStatus valida el enum + delega + audita como evento dedicado para
// que dashboards de compliance puedan filtrar cambios de estado.
func (s *Service) SetStatus(ctx context.Context, orgID uuid.UUID, id uuid.UUID, status string) (*Client, error) {
	if _, ok := validStatus[status]; !ok {
		return nil, ErrInvalidStatus
	}
	prev, err := s.repository().GetByID(ctx, orgID, id)
	if err != nil {
		return nil, err
	}
	c, err := s.repository().SetStatus(ctx, orgID, id, status)
	if err != nil {
		return nil, err
	}
	s.audit(ctx, orgID, nil, "client.status_changed", c.ID,
		map[string]any{"status": prev.Status},
		map[string]any{"status": c.Status},
	)
	return c, nil
}

// audit centraliza el RecordOrLog (nil-safe, action constante, entity_type
// "client"). Mantener todas las llamadas a audit acá garantiza que un nuevo
// método no se olvide del evento.
func (s *Service) audit(ctx context.Context, orgID uuid.UUID, actor *uuid.UUID, action string, entityID uuid.UUID, oldV, newV any) {
	if s.Audit == nil {
		return
	}
	audit.RecordOrLog(ctx, s.Audit, audit.Event{
		OrganizationID: &orgID,
		ActorID:        actor,
		ActorType:      audit.ActorUser,
		Action:         action,
		EntityType:     "client",
		EntityID:       &entityID,
		OldValues:      oldV,
		NewValues:      newV,
	})
}

// normalizeTaxID acepta "" (campo opcional). Intenta primero con
// internal/auth/rut (RUT chileno con dígito verificador módulo 11). Si no
// matchea ese formato, fallback a regex laxa para tax IDs no-chilenos.
func normalizeTaxID(raw string) (string, error) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return "", nil
	}
	if normalized, err := rut.Validate(raw); err == nil {
		return normalized, nil
	}



	if !reTaxIDFallback.MatchString(raw) {
		return "", ErrInvalidTaxID
	}
	return raw, nil
}
