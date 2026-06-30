package ticket

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/jackc/pgx/v5/pgxpool"

	"nunezlagos/domain/internal/service/ticket/ticketdb"
	"nunezlagos/domain/internal/store/txctx"
)

type pgRepository struct {
	pool *pgxpool.Pool
}

func NewPgRepository(pool *pgxpool.Pool) Repository {
	return &pgRepository{pool: pool}
}

func (r *pgRepository) q(ctx context.Context) *ticketdb.Queries {
	if tx := txctx.TxFromContext(ctx); tx != nil {
		return ticketdb.New(tx)
	}
	return ticketdb.New(r.pool)
}

// querier se usa solo para las queries dinámicas (List, Update, ChangeStatus)
// que no se pueden expresar con sqlc por tener SET/WHERE dinámicos.
type querier interface {
	QueryRow(ctx context.Context, sql string, args ...any) pgx.Row
	Query(ctx context.Context, sql string, args ...any) (pgx.Rows, error)
	Exec(ctx context.Context, sql string, args ...any) (pgconn.CommandTag, error)
}

func (r *pgRepository) rawQ(ctx context.Context) querier {
	if tx := txctx.TxFromContext(ctx); tx != nil {
		return tx
	}
	return r.pool
}

// --- helpers de conversión pgtype → tipos del dominio ---

func pgtypeTimestamptzToPtr(ts pgtype.Timestamptz) *time.Time {
	if !ts.Valid {
		return nil
	}
	t := ts.Time
	return &t
}

func pgtypeDateToPtr(d pgtype.Date) *time.Time {
	if !d.Valid {
		return nil
	}
	t := d.Time
	return &t
}

func pgtypeNumericToPtr(n pgtype.Numeric) *float64 {
	if !n.Valid {
		return nil
	}
	v, _ := n.Float64Value()
	if !v.Valid {
		return nil
	}
	fv := v.Float64
	return &fv
}

// fromRow convierte cualquier row de ticketdb que tiene los campos
// estándar del ticket a *Ticket del dominio.
type ticketRow interface {
	getID() uuid.UUID
	getProjectID() uuid.UUID
	getClientID() *uuid.UUID
	getKey() string
	getNumber() int32
	getTitle() string
	getDescriptionMd() string
	getIssueType() string
	getStatus() string
	getPriority() string
	getAssigneeID() *uuid.UUID
	getReporterID() uuid.UUID
	getLabels() []string
	getExternalProvider() string
	getExternalID() string
	getExternalUrl() string
	getExternalSyncedAt() pgtype.Timestamptz
	getParentID() *uuid.UUID
	getLinkedIssueID() *uuid.UUID
	getEstimatedHours() pgtype.Numeric
	getActualHours() pgtype.Numeric
	getDueDate() pgtype.Date
	getStartedAt() pgtype.Timestamptz
	getCompletedAt() pgtype.Timestamptz
	getLockedBy() *uuid.UUID
	getLockedUntil() pgtype.Timestamptz
	getVersion() int32
	getCreatedAt() time.Time
	getUpdatedAt() time.Time
	getDeletedAt() pgtype.Timestamptz
}

func rowToTicket(r ticketRow) *Ticket {
	t := &Ticket{
		ID:               r.getID(),
		ProjectID:        r.getProjectID(),
		ClientID:         r.getClientID(),
		Key:              r.getKey(),
		Number:           int(r.getNumber()),
		Title:            r.getTitle(),
		DescriptionMD:    r.getDescriptionMd(),
		IssueType:        r.getIssueType(),
		Status:           r.getStatus(),
		Priority:         r.getPriority(),
		AssigneeID:       r.getAssigneeID(),
		ReporterID:       r.getReporterID(),
		Labels:           r.getLabels(),
		ExternalProvider: r.getExternalProvider(),
		ExternalID:       r.getExternalID(),
		ExternalURL:      r.getExternalUrl(),
		ExternalSyncedAt: pgtypeTimestamptzToPtr(r.getExternalSyncedAt()),
		ParentID:         r.getParentID(),
		LinkedIssueID:    r.getLinkedIssueID(),
		EstimatedHours:   pgtypeNumericToPtr(r.getEstimatedHours()),
		ActualHours:      pgtypeNumericToPtr(r.getActualHours()),
		DueDate:          pgtypeDateToPtr(r.getDueDate()),
		StartedAt:        pgtypeTimestamptzToPtr(r.getStartedAt()),
		CompletedAt:      pgtypeTimestamptzToPtr(r.getCompletedAt()),
		LockedBy:         r.getLockedBy(),
		LockedUntil:      pgtypeTimestamptzToPtr(r.getLockedUntil()),
		Version:          int(r.getVersion()),
		CreatedAt:        r.getCreatedAt(),
		UpdatedAt:        r.getUpdatedAt(),
		DeletedAt:        pgtypeTimestamptzToPtr(r.getDeletedAt()),
	}
	t.DisplayKey = t.Key
	if t.ExternalID != "" {
		t.DisplayKey = t.ExternalID
	}
	return t
}

// --- Adaptadores para cada Row type generado ---

type getTicketByIDRowAdapter ticketdb.GetTicketByIDRow

func (a getTicketByIDRowAdapter) getID() uuid.UUID                        { return a.ID }
func (a getTicketByIDRowAdapter) getProjectID() uuid.UUID                 { return a.ProjectID }
func (a getTicketByIDRowAdapter) getClientID() *uuid.UUID                 { return a.ClientID }
func (a getTicketByIDRowAdapter) getKey() string                          { return a.Key }
func (a getTicketByIDRowAdapter) getNumber() int32                        { return a.Number }
func (a getTicketByIDRowAdapter) getTitle() string                        { return a.Title }
func (a getTicketByIDRowAdapter) getDescriptionMd() string                { return a.DescriptionMd }
func (a getTicketByIDRowAdapter) getIssueType() string                    { return a.IssueType }
func (a getTicketByIDRowAdapter) getStatus() string                       { return a.Status }
func (a getTicketByIDRowAdapter) getPriority() string                     { return a.Priority }
func (a getTicketByIDRowAdapter) getAssigneeID() *uuid.UUID               { return a.AssigneeID }
func (a getTicketByIDRowAdapter) getReporterID() uuid.UUID                { return a.ReporterID }
func (a getTicketByIDRowAdapter) getLabels() []string                     { return a.Labels }
func (a getTicketByIDRowAdapter) getExternalProvider() string             { return a.ExternalProvider }
func (a getTicketByIDRowAdapter) getExternalID() string                   { return a.ExternalID }
func (a getTicketByIDRowAdapter) getExternalUrl() string                  { return a.ExternalUrl }
func (a getTicketByIDRowAdapter) getExternalSyncedAt() pgtype.Timestamptz { return a.ExternalSyncedAt }
func (a getTicketByIDRowAdapter) getParentID() *uuid.UUID                 { return a.ParentID }
func (a getTicketByIDRowAdapter) getLinkedIssueID() *uuid.UUID            { return a.LinkedIssueID }
func (a getTicketByIDRowAdapter) getEstimatedHours() pgtype.Numeric       { return a.EstimatedHours }
func (a getTicketByIDRowAdapter) getActualHours() pgtype.Numeric          { return a.ActualHours }
func (a getTicketByIDRowAdapter) getDueDate() pgtype.Date                 { return a.DueDate }
func (a getTicketByIDRowAdapter) getStartedAt() pgtype.Timestamptz        { return a.StartedAt }
func (a getTicketByIDRowAdapter) getCompletedAt() pgtype.Timestamptz      { return a.CompletedAt }
func (a getTicketByIDRowAdapter) getLockedBy() *uuid.UUID                 { return a.LockedBy }
func (a getTicketByIDRowAdapter) getLockedUntil() pgtype.Timestamptz      { return a.LockedUntil }
func (a getTicketByIDRowAdapter) getVersion() int32                       { return a.Version }
func (a getTicketByIDRowAdapter) getCreatedAt() time.Time                 { return a.CreatedAt }
func (a getTicketByIDRowAdapter) getUpdatedAt() time.Time                 { return a.UpdatedAt }
func (a getTicketByIDRowAdapter) getDeletedAt() pgtype.Timestamptz        { return a.DeletedAt }

type getTicketByKeyRowAdapter ticketdb.GetTicketByKeyRow

func (a getTicketByKeyRowAdapter) getID() uuid.UUID                        { return a.ID }
func (a getTicketByKeyRowAdapter) getProjectID() uuid.UUID                 { return a.ProjectID }
func (a getTicketByKeyRowAdapter) getClientID() *uuid.UUID                 { return a.ClientID }
func (a getTicketByKeyRowAdapter) getKey() string                          { return a.Key }
func (a getTicketByKeyRowAdapter) getNumber() int32                        { return a.Number }
func (a getTicketByKeyRowAdapter) getTitle() string                        { return a.Title }
func (a getTicketByKeyRowAdapter) getDescriptionMd() string                { return a.DescriptionMd }
func (a getTicketByKeyRowAdapter) getIssueType() string                    { return a.IssueType }
func (a getTicketByKeyRowAdapter) getStatus() string                       { return a.Status }
func (a getTicketByKeyRowAdapter) getPriority() string                     { return a.Priority }
func (a getTicketByKeyRowAdapter) getAssigneeID() *uuid.UUID               { return a.AssigneeID }
func (a getTicketByKeyRowAdapter) getReporterID() uuid.UUID                { return a.ReporterID }
func (a getTicketByKeyRowAdapter) getLabels() []string                     { return a.Labels }
func (a getTicketByKeyRowAdapter) getExternalProvider() string             { return a.ExternalProvider }
func (a getTicketByKeyRowAdapter) getExternalID() string                   { return a.ExternalID }
func (a getTicketByKeyRowAdapter) getExternalUrl() string                  { return a.ExternalUrl }
func (a getTicketByKeyRowAdapter) getExternalSyncedAt() pgtype.Timestamptz { return a.ExternalSyncedAt }
func (a getTicketByKeyRowAdapter) getParentID() *uuid.UUID                 { return a.ParentID }
func (a getTicketByKeyRowAdapter) getLinkedIssueID() *uuid.UUID            { return a.LinkedIssueID }
func (a getTicketByKeyRowAdapter) getEstimatedHours() pgtype.Numeric       { return a.EstimatedHours }
func (a getTicketByKeyRowAdapter) getActualHours() pgtype.Numeric          { return a.ActualHours }
func (a getTicketByKeyRowAdapter) getDueDate() pgtype.Date                 { return a.DueDate }
func (a getTicketByKeyRowAdapter) getStartedAt() pgtype.Timestamptz        { return a.StartedAt }
func (a getTicketByKeyRowAdapter) getCompletedAt() pgtype.Timestamptz      { return a.CompletedAt }
func (a getTicketByKeyRowAdapter) getLockedBy() *uuid.UUID                 { return a.LockedBy }
func (a getTicketByKeyRowAdapter) getLockedUntil() pgtype.Timestamptz      { return a.LockedUntil }
func (a getTicketByKeyRowAdapter) getVersion() int32                       { return a.Version }
func (a getTicketByKeyRowAdapter) getCreatedAt() time.Time                 { return a.CreatedAt }
func (a getTicketByKeyRowAdapter) getUpdatedAt() time.Time                 { return a.UpdatedAt }
func (a getTicketByKeyRowAdapter) getDeletedAt() pgtype.Timestamptz        { return a.DeletedAt }

type insertTicketRowAdapter ticketdb.InsertTicketRow

func (a insertTicketRowAdapter) getID() uuid.UUID                        { return a.ID }
func (a insertTicketRowAdapter) getProjectID() uuid.UUID                 { return a.ProjectID }
func (a insertTicketRowAdapter) getClientID() *uuid.UUID                 { return a.ClientID }
func (a insertTicketRowAdapter) getKey() string                          { return a.Key }
func (a insertTicketRowAdapter) getNumber() int32                        { return a.Number }
func (a insertTicketRowAdapter) getTitle() string                        { return a.Title }
func (a insertTicketRowAdapter) getDescriptionMd() string                { return a.DescriptionMd }
func (a insertTicketRowAdapter) getIssueType() string                    { return a.IssueType }
func (a insertTicketRowAdapter) getStatus() string                       { return a.Status }
func (a insertTicketRowAdapter) getPriority() string                     { return a.Priority }
func (a insertTicketRowAdapter) getAssigneeID() *uuid.UUID               { return a.AssigneeID }
func (a insertTicketRowAdapter) getReporterID() uuid.UUID                { return a.ReporterID }
func (a insertTicketRowAdapter) getLabels() []string                     { return a.Labels }
func (a insertTicketRowAdapter) getExternalProvider() string             { return a.ExternalProvider }
func (a insertTicketRowAdapter) getExternalID() string                   { return a.ExternalID }
func (a insertTicketRowAdapter) getExternalUrl() string                  { return a.ExternalUrl }
func (a insertTicketRowAdapter) getExternalSyncedAt() pgtype.Timestamptz { return a.ExternalSyncedAt }
func (a insertTicketRowAdapter) getParentID() *uuid.UUID                 { return a.ParentID }
func (a insertTicketRowAdapter) getLinkedIssueID() *uuid.UUID            { return a.LinkedIssueID }
func (a insertTicketRowAdapter) getEstimatedHours() pgtype.Numeric       { return a.EstimatedHours }
func (a insertTicketRowAdapter) getActualHours() pgtype.Numeric          { return a.ActualHours }
func (a insertTicketRowAdapter) getDueDate() pgtype.Date                 { return a.DueDate }
func (a insertTicketRowAdapter) getStartedAt() pgtype.Timestamptz        { return a.StartedAt }
func (a insertTicketRowAdapter) getCompletedAt() pgtype.Timestamptz      { return a.CompletedAt }
func (a insertTicketRowAdapter) getLockedBy() *uuid.UUID                 { return a.LockedBy }
func (a insertTicketRowAdapter) getLockedUntil() pgtype.Timestamptz      { return a.LockedUntil }
func (a insertTicketRowAdapter) getVersion() int32                       { return a.Version }
func (a insertTicketRowAdapter) getCreatedAt() time.Time                 { return a.CreatedAt }
func (a insertTicketRowAdapter) getUpdatedAt() time.Time                 { return a.UpdatedAt }
func (a insertTicketRowAdapter) getDeletedAt() pgtype.Timestamptz        { return a.DeletedAt }

type claimTicketRowAdapter ticketdb.ClaimTicketRow

func (a claimTicketRowAdapter) getID() uuid.UUID                        { return a.ID }
func (a claimTicketRowAdapter) getProjectID() uuid.UUID                 { return a.ProjectID }
func (a claimTicketRowAdapter) getClientID() *uuid.UUID                 { return a.ClientID }
func (a claimTicketRowAdapter) getKey() string                          { return a.Key }
func (a claimTicketRowAdapter) getNumber() int32                        { return a.Number }
func (a claimTicketRowAdapter) getTitle() string                        { return a.Title }
func (a claimTicketRowAdapter) getDescriptionMd() string                { return a.DescriptionMd }
func (a claimTicketRowAdapter) getIssueType() string                    { return a.IssueType }
func (a claimTicketRowAdapter) getStatus() string                       { return a.Status }
func (a claimTicketRowAdapter) getPriority() string                     { return a.Priority }
func (a claimTicketRowAdapter) getAssigneeID() *uuid.UUID               { return a.AssigneeID }
func (a claimTicketRowAdapter) getReporterID() uuid.UUID                { return a.ReporterID }
func (a claimTicketRowAdapter) getLabels() []string                     { return a.Labels }
func (a claimTicketRowAdapter) getExternalProvider() string             { return a.ExternalProvider }
func (a claimTicketRowAdapter) getExternalID() string                   { return a.ExternalID }
func (a claimTicketRowAdapter) getExternalUrl() string                  { return a.ExternalUrl }
func (a claimTicketRowAdapter) getExternalSyncedAt() pgtype.Timestamptz { return a.ExternalSyncedAt }
func (a claimTicketRowAdapter) getParentID() *uuid.UUID                 { return a.ParentID }
func (a claimTicketRowAdapter) getLinkedIssueID() *uuid.UUID            { return a.LinkedIssueID }
func (a claimTicketRowAdapter) getEstimatedHours() pgtype.Numeric       { return a.EstimatedHours }
func (a claimTicketRowAdapter) getActualHours() pgtype.Numeric          { return a.ActualHours }
func (a claimTicketRowAdapter) getDueDate() pgtype.Date                 { return a.DueDate }
func (a claimTicketRowAdapter) getStartedAt() pgtype.Timestamptz        { return a.StartedAt }
func (a claimTicketRowAdapter) getCompletedAt() pgtype.Timestamptz      { return a.CompletedAt }
func (a claimTicketRowAdapter) getLockedBy() *uuid.UUID                 { return a.LockedBy }
func (a claimTicketRowAdapter) getLockedUntil() pgtype.Timestamptz      { return a.LockedUntil }
func (a claimTicketRowAdapter) getVersion() int32                       { return a.Version }
func (a claimTicketRowAdapter) getCreatedAt() time.Time                 { return a.CreatedAt }
func (a claimTicketRowAdapter) getUpdatedAt() time.Time                 { return a.UpdatedAt }
func (a claimTicketRowAdapter) getDeletedAt() pgtype.Timestamptz        { return a.DeletedAt }

type releaseTicketRowAdapter ticketdb.ReleaseTicketRow

func (a releaseTicketRowAdapter) getID() uuid.UUID                        { return a.ID }
func (a releaseTicketRowAdapter) getProjectID() uuid.UUID                 { return a.ProjectID }
func (a releaseTicketRowAdapter) getClientID() *uuid.UUID                 { return a.ClientID }
func (a releaseTicketRowAdapter) getKey() string                          { return a.Key }
func (a releaseTicketRowAdapter) getNumber() int32                        { return a.Number }
func (a releaseTicketRowAdapter) getTitle() string                        { return a.Title }
func (a releaseTicketRowAdapter) getDescriptionMd() string                { return a.DescriptionMd }
func (a releaseTicketRowAdapter) getIssueType() string                    { return a.IssueType }
func (a releaseTicketRowAdapter) getStatus() string                       { return a.Status }
func (a releaseTicketRowAdapter) getPriority() string                     { return a.Priority }
func (a releaseTicketRowAdapter) getAssigneeID() *uuid.UUID               { return a.AssigneeID }
func (a releaseTicketRowAdapter) getReporterID() uuid.UUID                { return a.ReporterID }
func (a releaseTicketRowAdapter) getLabels() []string                     { return a.Labels }
func (a releaseTicketRowAdapter) getExternalProvider() string             { return a.ExternalProvider }
func (a releaseTicketRowAdapter) getExternalID() string                   { return a.ExternalID }
func (a releaseTicketRowAdapter) getExternalUrl() string                  { return a.ExternalUrl }
func (a releaseTicketRowAdapter) getExternalSyncedAt() pgtype.Timestamptz { return a.ExternalSyncedAt }
func (a releaseTicketRowAdapter) getParentID() *uuid.UUID                 { return a.ParentID }
func (a releaseTicketRowAdapter) getLinkedIssueID() *uuid.UUID            { return a.LinkedIssueID }
func (a releaseTicketRowAdapter) getEstimatedHours() pgtype.Numeric       { return a.EstimatedHours }
func (a releaseTicketRowAdapter) getActualHours() pgtype.Numeric          { return a.ActualHours }
func (a releaseTicketRowAdapter) getDueDate() pgtype.Date                 { return a.DueDate }
func (a releaseTicketRowAdapter) getStartedAt() pgtype.Timestamptz        { return a.StartedAt }
func (a releaseTicketRowAdapter) getCompletedAt() pgtype.Timestamptz      { return a.CompletedAt }
func (a releaseTicketRowAdapter) getLockedBy() *uuid.UUID                 { return a.LockedBy }
func (a releaseTicketRowAdapter) getLockedUntil() pgtype.Timestamptz      { return a.LockedUntil }
func (a releaseTicketRowAdapter) getVersion() int32                       { return a.Version }
func (a releaseTicketRowAdapter) getCreatedAt() time.Time                 { return a.CreatedAt }
func (a releaseTicketRowAdapter) getUpdatedAt() time.Time                 { return a.UpdatedAt }
func (a releaseTicketRowAdapter) getDeletedAt() pgtype.Timestamptz        { return a.DeletedAt }

type linkTicketIssueRowAdapter ticketdb.LinkTicketIssueRow

func (a linkTicketIssueRowAdapter) getID() uuid.UUID                        { return a.ID }
func (a linkTicketIssueRowAdapter) getProjectID() uuid.UUID                 { return a.ProjectID }
func (a linkTicketIssueRowAdapter) getClientID() *uuid.UUID                 { return a.ClientID }
func (a linkTicketIssueRowAdapter) getKey() string                          { return a.Key }
func (a linkTicketIssueRowAdapter) getNumber() int32                        { return a.Number }
func (a linkTicketIssueRowAdapter) getTitle() string                        { return a.Title }
func (a linkTicketIssueRowAdapter) getDescriptionMd() string                { return a.DescriptionMd }
func (a linkTicketIssueRowAdapter) getIssueType() string                    { return a.IssueType }
func (a linkTicketIssueRowAdapter) getStatus() string                       { return a.Status }
func (a linkTicketIssueRowAdapter) getPriority() string                     { return a.Priority }
func (a linkTicketIssueRowAdapter) getAssigneeID() *uuid.UUID               { return a.AssigneeID }
func (a linkTicketIssueRowAdapter) getReporterID() uuid.UUID                { return a.ReporterID }
func (a linkTicketIssueRowAdapter) getLabels() []string                     { return a.Labels }
func (a linkTicketIssueRowAdapter) getExternalProvider() string             { return a.ExternalProvider }
func (a linkTicketIssueRowAdapter) getExternalID() string                   { return a.ExternalID }
func (a linkTicketIssueRowAdapter) getExternalUrl() string                  { return a.ExternalUrl }
func (a linkTicketIssueRowAdapter) getExternalSyncedAt() pgtype.Timestamptz { return a.ExternalSyncedAt }
func (a linkTicketIssueRowAdapter) getParentID() *uuid.UUID                 { return a.ParentID }
func (a linkTicketIssueRowAdapter) getLinkedIssueID() *uuid.UUID            { return a.LinkedIssueID }
func (a linkTicketIssueRowAdapter) getEstimatedHours() pgtype.Numeric       { return a.EstimatedHours }
func (a linkTicketIssueRowAdapter) getActualHours() pgtype.Numeric          { return a.ActualHours }
func (a linkTicketIssueRowAdapter) getDueDate() pgtype.Date                 { return a.DueDate }
func (a linkTicketIssueRowAdapter) getStartedAt() pgtype.Timestamptz        { return a.StartedAt }
func (a linkTicketIssueRowAdapter) getCompletedAt() pgtype.Timestamptz      { return a.CompletedAt }
func (a linkTicketIssueRowAdapter) getLockedBy() *uuid.UUID                 { return a.LockedBy }
func (a linkTicketIssueRowAdapter) getLockedUntil() pgtype.Timestamptz      { return a.LockedUntil }
func (a linkTicketIssueRowAdapter) getVersion() int32                       { return a.Version }
func (a linkTicketIssueRowAdapter) getCreatedAt() time.Time                 { return a.CreatedAt }
func (a linkTicketIssueRowAdapter) getUpdatedAt() time.Time                 { return a.UpdatedAt }
func (a linkTicketIssueRowAdapter) getDeletedAt() pgtype.Timestamptz        { return a.DeletedAt }

type linkTicketExternalRowAdapter ticketdb.LinkTicketExternalRow

func (a linkTicketExternalRowAdapter) getID() uuid.UUID                        { return a.ID }
func (a linkTicketExternalRowAdapter) getProjectID() uuid.UUID                 { return a.ProjectID }
func (a linkTicketExternalRowAdapter) getClientID() *uuid.UUID                 { return a.ClientID }
func (a linkTicketExternalRowAdapter) getKey() string                          { return a.Key }
func (a linkTicketExternalRowAdapter) getNumber() int32                        { return a.Number }
func (a linkTicketExternalRowAdapter) getTitle() string                        { return a.Title }
func (a linkTicketExternalRowAdapter) getDescriptionMd() string                { return a.DescriptionMd }
func (a linkTicketExternalRowAdapter) getIssueType() string                    { return a.IssueType }
func (a linkTicketExternalRowAdapter) getStatus() string                       { return a.Status }
func (a linkTicketExternalRowAdapter) getPriority() string                     { return a.Priority }
func (a linkTicketExternalRowAdapter) getAssigneeID() *uuid.UUID               { return a.AssigneeID }
func (a linkTicketExternalRowAdapter) getReporterID() uuid.UUID                { return a.ReporterID }
func (a linkTicketExternalRowAdapter) getLabels() []string                     { return a.Labels }
func (a linkTicketExternalRowAdapter) getExternalProvider() string             { return a.ExternalProvider }
func (a linkTicketExternalRowAdapter) getExternalID() string                   { return a.ExternalID }
func (a linkTicketExternalRowAdapter) getExternalUrl() string                  { return a.ExternalUrl }
func (a linkTicketExternalRowAdapter) getExternalSyncedAt() pgtype.Timestamptz { return a.ExternalSyncedAt }
func (a linkTicketExternalRowAdapter) getParentID() *uuid.UUID                 { return a.ParentID }
func (a linkTicketExternalRowAdapter) getLinkedIssueID() *uuid.UUID            { return a.LinkedIssueID }
func (a linkTicketExternalRowAdapter) getEstimatedHours() pgtype.Numeric       { return a.EstimatedHours }
func (a linkTicketExternalRowAdapter) getActualHours() pgtype.Numeric          { return a.ActualHours }
func (a linkTicketExternalRowAdapter) getDueDate() pgtype.Date                 { return a.DueDate }
func (a linkTicketExternalRowAdapter) getStartedAt() pgtype.Timestamptz        { return a.StartedAt }
func (a linkTicketExternalRowAdapter) getCompletedAt() pgtype.Timestamptz      { return a.CompletedAt }
func (a linkTicketExternalRowAdapter) getLockedBy() *uuid.UUID                 { return a.LockedBy }
func (a linkTicketExternalRowAdapter) getLockedUntil() pgtype.Timestamptz      { return a.LockedUntil }
func (a linkTicketExternalRowAdapter) getVersion() int32                       { return a.Version }
func (a linkTicketExternalRowAdapter) getCreatedAt() time.Time                 { return a.CreatedAt }
func (a linkTicketExternalRowAdapter) getUpdatedAt() time.Time                 { return a.UpdatedAt }
func (a linkTicketExternalRowAdapter) getDeletedAt() pgtype.Timestamptz        { return a.DeletedAt }

type findTicketByExternalRowAdapter ticketdb.FindTicketByExternalRow

func (a findTicketByExternalRowAdapter) getID() uuid.UUID                        { return a.ID }
func (a findTicketByExternalRowAdapter) getProjectID() uuid.UUID                 { return a.ProjectID }
func (a findTicketByExternalRowAdapter) getClientID() *uuid.UUID                 { return a.ClientID }
func (a findTicketByExternalRowAdapter) getKey() string                          { return a.Key }
func (a findTicketByExternalRowAdapter) getNumber() int32                        { return a.Number }
func (a findTicketByExternalRowAdapter) getTitle() string                        { return a.Title }
func (a findTicketByExternalRowAdapter) getDescriptionMd() string                { return a.DescriptionMd }
func (a findTicketByExternalRowAdapter) getIssueType() string                    { return a.IssueType }
func (a findTicketByExternalRowAdapter) getStatus() string                       { return a.Status }
func (a findTicketByExternalRowAdapter) getPriority() string                     { return a.Priority }
func (a findTicketByExternalRowAdapter) getAssigneeID() *uuid.UUID               { return a.AssigneeID }
func (a findTicketByExternalRowAdapter) getReporterID() uuid.UUID                { return a.ReporterID }
func (a findTicketByExternalRowAdapter) getLabels() []string                     { return a.Labels }
func (a findTicketByExternalRowAdapter) getExternalProvider() string             { return a.ExternalProvider }
func (a findTicketByExternalRowAdapter) getExternalID() string                   { return a.ExternalID }
func (a findTicketByExternalRowAdapter) getExternalUrl() string                  { return a.ExternalUrl }
func (a findTicketByExternalRowAdapter) getExternalSyncedAt() pgtype.Timestamptz { return a.ExternalSyncedAt }
func (a findTicketByExternalRowAdapter) getParentID() *uuid.UUID                 { return a.ParentID }
func (a findTicketByExternalRowAdapter) getLinkedIssueID() *uuid.UUID            { return a.LinkedIssueID }
func (a findTicketByExternalRowAdapter) getEstimatedHours() pgtype.Numeric       { return a.EstimatedHours }
func (a findTicketByExternalRowAdapter) getActualHours() pgtype.Numeric          { return a.ActualHours }
func (a findTicketByExternalRowAdapter) getDueDate() pgtype.Date                 { return a.DueDate }
func (a findTicketByExternalRowAdapter) getStartedAt() pgtype.Timestamptz        { return a.StartedAt }
func (a findTicketByExternalRowAdapter) getCompletedAt() pgtype.Timestamptz      { return a.CompletedAt }
func (a findTicketByExternalRowAdapter) getLockedBy() *uuid.UUID                 { return a.LockedBy }
func (a findTicketByExternalRowAdapter) getLockedUntil() pgtype.Timestamptz      { return a.LockedUntil }
func (a findTicketByExternalRowAdapter) getVersion() int32                       { return a.Version }
func (a findTicketByExternalRowAdapter) getCreatedAt() time.Time                 { return a.CreatedAt }
func (a findTicketByExternalRowAdapter) getUpdatedAt() time.Time                 { return a.UpdatedAt }
func (a findTicketByExternalRowAdapter) getDeletedAt() pgtype.Timestamptz        { return a.DeletedAt }

// --- helpers ---

func keyPrefix(projectSlug string) string {
	s := strings.ToUpper(strings.TrimSpace(projectSlug))
	if s == "" {
		return "TKT"
	}
	var sb strings.Builder
	for _, r := range s {
		if (r >= 'A' && r <= 'Z') || (r >= '0' && r <= '9') {
			sb.WriteRune(r)
		}
	}
	out := sb.String()
	if out == "" {
		return "TKT"
	}
	if len(out) > 10 {
		out = out[:10]
	}
	return out
}

func isUniqueViolation(err error) bool {
	var pgErr *pgconn.PgError
	return errors.As(err, &pgErr) && pgErr.Code == "23505"
}

func isExternalUniqueViolation(err error) bool {
	var pgErr *pgconn.PgError
	if !errors.As(err, &pgErr) || pgErr.Code != "23505" {
		return false
	}
	return strings.Contains(pgErr.ConstraintName, "external_unique") ||
		strings.Contains(pgErr.Detail, "external_id")
}

func nullIfEmpty(s string) any {
	if s == "" {
		return nil
	}
	return s
}

// floatToNumeric convierte *float64 a pgtype.Numeric
func floatToNumeric(f *float64) pgtype.Numeric {
	if f == nil {
		return pgtype.Numeric{Valid: false}
	}
	var n pgtype.Numeric
	_ = n.Scan(fmt.Sprintf("%g", *f))
	return n
}

// timeToDate convierte *time.Time a pgtype.Date
func timeToDate(t *time.Time) pgtype.Date {
	if t == nil {
		return pgtype.Date{Valid: false}
	}
	return pgtype.Date{Time: *t, Valid: true}
}

// timeToTimestamptz convierte *time.Time a pgtype.Timestamptz
func timeToTimestamptz(t *time.Time) pgtype.Timestamptz {
	if t == nil {
		return pgtype.Timestamptz{Valid: false}
	}
	return pgtype.Timestamptz{Time: *t, Valid: true}
}

// stringToPtr convierte string a *string (nil si vacío)
func stringToPtr(s string) *string {
	if s == "" {
		return nil
	}
	return &s
}

// --- selectCols y scanTicket para las queries dinámicas ---

const selectCols = `id, project_id, client_id, key, number,
		title, COALESCE(description_md,''), issue_type, status, priority,
		assignee_id, reporter_id, labels,
		COALESCE(external_provider,''), COALESCE(external_id,''),
		COALESCE(external_url,''), external_synced_at,
		parent_id, linked_issue_id, estimated_hours, actual_hours,
		due_date, started_at, completed_at,
		locked_by, locked_until, version,
		created_at, updated_at, deleted_at`

func scanTicket(row pgx.Row) (*Ticket, error) {
	var t Ticket
	var (
		extSyncedAt pgtype.Timestamptz
		startedAt   pgtype.Timestamptz
		completedAt pgtype.Timestamptz
		lockedUntil pgtype.Timestamptz
		deletedAt   pgtype.Timestamptz
		estimHours  pgtype.Numeric
		actualHours pgtype.Numeric
		dueDate     pgtype.Date
	)
	if err := row.Scan(
		&t.ID, &t.ProjectID, &t.ClientID, &t.Key, &t.Number,
		&t.Title, &t.DescriptionMD, &t.IssueType, &t.Status, &t.Priority,
		&t.AssigneeID, &t.ReporterID, &t.Labels,
		&t.ExternalProvider, &t.ExternalID, &t.ExternalURL, &extSyncedAt,
		&t.ParentID, &t.LinkedIssueID, &estimHours, &actualHours,
		&dueDate, &startedAt, &completedAt,
		&t.LockedBy, &lockedUntil, &t.Version,
		&t.CreatedAt, &t.UpdatedAt, &deletedAt,
	); err != nil {
		return nil, err
	}
	t.ExternalSyncedAt = pgtypeTimestamptzToPtr(extSyncedAt)
	t.StartedAt = pgtypeTimestamptzToPtr(startedAt)
	t.CompletedAt = pgtypeTimestamptzToPtr(completedAt)
	t.LockedUntil = pgtypeTimestamptzToPtr(lockedUntil)
	t.DeletedAt = pgtypeTimestamptzToPtr(deletedAt)
	t.EstimatedHours = pgtypeNumericToPtr(estimHours)
	t.ActualHours = pgtypeNumericToPtr(actualHours)
	t.DueDate = pgtypeDateToPtr(dueDate)

	t.DisplayKey = t.Key
	if t.ExternalID != "" {
		t.DisplayKey = t.ExternalID
	}
	return &t, nil
}

// --- Implementaciones del Repository ---

// LinkIssue setea o limpia linked_issue_id. issueID=nil → desvinculación.
func (r *pgRepository) LinkIssue(ctx context.Context, orgID, ticketID uuid.UUID, issueID *uuid.UUID) (*Ticket, error) {
	row, err := r.q(ctx).LinkTicketIssue(ctx, ticketdb.LinkTicketIssueParams{
		ID:            ticketID,
		LinkedIssueID: issueID,
	})
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, err
	}
	return rowToTicket(linkTicketIssueRowAdapter(row)), nil
}

func (r *pgRepository) Insert(ctx context.Context, in CreateInput) (*Ticket, error) {
	prefix := keyPrefix(in.ProjectSlug)
	for attempt := 0; attempt < 2; attempt++ {
		num, err := r.q(ctx).TicketNextNumber(ctx, in.ProjectID)
		if err != nil {
			return nil, fmt.Errorf("next number: %w", err)
		}
		key := fmt.Sprintf("%s-%d", prefix, num)

		var extSyncedAt pgtype.Timestamptz
		if in.ExternalProvider != "" {
			extSyncedAt = pgtype.Timestamptz{Time: time.Now(), Valid: true}
		}

		row, err := r.q(ctx).InsertTicket(ctx, ticketdb.InsertTicketParams{
			ProjectID:        in.ProjectID,
			ClientID:         in.ClientID,
			Key:              key,
			Number:           num,
			Title:            in.Title,
			DescriptionMd:    in.DescriptionMD,
			IssueType:        in.IssueType,
			Priority:         in.Priority,
			AssigneeID:       in.AssigneeID,
			ReporterID:       in.ReporterID,
			Labels:           in.Labels,
			ParentID:         in.ParentID,
			EstimatedHours:   floatToNumeric(in.EstimatedHours),
			DueDate:          timeToDate(in.DueDate),
			ExternalProvider: in.ExternalProvider,
			ExternalID:       in.ExternalID,
			ExternalUrl:      in.ExternalURL,
			ExternalSyncedAt: extSyncedAt,
		})
		if isExternalUniqueViolation(err) {
			return nil, ErrExternalAlreadyLinked
		}
		if isUniqueViolation(err) {
			continue // race en (org, project, number) — retry
		}
		if err != nil {
			return nil, fmt.Errorf("insert ticket: %w", err)
		}
		t := rowToTicket(insertTicketRowAdapter(row))
		_ = r.q(ctx).InsertStatusHistory(ctx, ticketdb.InsertStatusHistoryParams{
			TicketID:   t.ID,
			FromStatus: nil,
			ToStatus:   t.Status,
			ChangedBy:  in.ReporterID,
			Note:       stringToPtr("created"),
		})
		return t, nil
	}
	return nil, fmt.Errorf("insert ticket: tras 2 reintentos sigue habiendo race condition")
}

func (r *pgRepository) Get(ctx context.Context, orgID, id uuid.UUID) (*Ticket, error) {
	row, err := r.q(ctx).GetTicketByID(ctx, id)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, err
	}
	return rowToTicket(getTicketByIDRowAdapter(row)), nil
}

func (r *pgRepository) GetByKey(ctx context.Context, orgID, projectID uuid.UUID, key string) (*Ticket, error) {
	row, err := r.q(ctx).GetTicketByKey(ctx, ticketdb.GetTicketByKeyParams{
		ProjectID: projectID,
		Key:       strings.ToUpper(key),
	})
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, err
	}
	return rowToTicket(getTicketByKeyRowAdapter(row)), nil
}

// List usa raw SQL porque tiene filtros dinámicos que no se expresan en sqlc.
func (r *pgRepository) List(ctx context.Context, orgID uuid.UUID, filter ListFilter) ([]*Ticket, int64, error) {
	conds := []string{"deleted_at IS NULL"}
	args := []any{}
	idx := 1
	add := func(cond string, val any) {
		conds = append(conds, fmt.Sprintf(cond, idx))
		args = append(args, val)
		idx++
	}
	if filter.ProjectID != nil {
		add("project_id = $%d", *filter.ProjectID)
	}
	if filter.Status != "" {
		add("status = $%d", filter.Status)
	}
	if filter.IssueType != "" {
		add("issue_type = $%d", filter.IssueType)
	}
	if filter.Priority != "" {
		add("priority = $%d", filter.Priority)
	}
	if filter.AssigneeID != nil {
		add("assignee_id = $%d", *filter.AssigneeID)
	}
	if filter.ReporterID != nil {
		add("reporter_id = $%d", *filter.ReporterID)
	}
	if filter.ParentID != nil {
		add("parent_id = $%d", *filter.ParentID)
	}
	if filter.Label != "" {
		add("$%d = ANY(labels)", filter.Label)
	}
	if filter.Query != "" {
		add("description_tsv @@ plainto_tsquery('spanish', $%d)", filter.Query)
	}
	where := "WHERE " + strings.Join(conds, " AND ")

	var total int64
	if err := r.rawQ(ctx).QueryRow(ctx,
		`SELECT COUNT(*) FROM project_tickets `+where, args...,
	).Scan(&total); err != nil {
		return nil, 0, fmt.Errorf("count tickets: %w", err)
	}

	limit := filter.Limit
	if limit <= 0 || limit > 200 {
		limit = 50
	}
	args = append(args, limit, filter.Offset)
	rows, err := r.rawQ(ctx).Query(ctx,
		`SELECT `+selectCols+` FROM project_tickets `+where+
			fmt.Sprintf(" ORDER BY updated_at DESC LIMIT $%d OFFSET $%d", idx, idx+1),
		args...,
	)
	if err != nil {
		return nil, 0, fmt.Errorf("list tickets: %w", err)
	}
	defer rows.Close()
	out := make([]*Ticket, 0, limit)
	for rows.Next() {
		t, err := scanTicket(rows)
		if err != nil {
			return nil, 0, err
		}
		out = append(out, t)
	}
	return out, total, rows.Err()
}

// Update usa raw SQL porque tiene SET dinámico.
func (r *pgRepository) Update(ctx context.Context, orgID, id uuid.UUID, in UpdateInput) (*Ticket, error) {
	sets := []string{}
	args := []any{id}
	idx := 2
	add := func(col string, v any) {
		sets = append(sets, fmt.Sprintf("%s = $%d", col, idx))
		args = append(args, v)
		idx++
	}
	if in.Title != nil {
		add("title", *in.Title)
	}
	if in.DescriptionMD != nil {
		add("description_md", nullIfEmpty(*in.DescriptionMD))
	}
	if in.IssueType != nil {
		add("issue_type", *in.IssueType)
	}
	if in.Priority != nil {
		add("priority", *in.Priority)
	}
	if in.AssigneeID != nil {
		if *in.AssigneeID == uuid.Nil {
			add("assignee_id", nil)
		} else {
			add("assignee_id", *in.AssigneeID)
		}
	}
	if in.Labels != nil {
		add("labels", *in.Labels)
	}
	if in.ParentID != nil {
		if *in.ParentID == uuid.Nil {
			add("parent_id", nil)
		} else if *in.ParentID == id {
			return nil, ErrSelfParent
		} else {
			add("parent_id", *in.ParentID)
		}
	}
	if in.EstimatedHours != nil {
		add("estimated_hours", *in.EstimatedHours)
	}
	if in.ActualHours != nil {
		add("actual_hours", *in.ActualHours)
	}
	if in.DueDate != nil {
		add("due_date", *in.DueDate)
	}
	if len(sets) == 0 {
		return r.Get(ctx, orgID, id)
	}
	q := `UPDATE project_tickets SET ` + strings.Join(sets, ", ") +
		` WHERE id = $1 AND deleted_at IS NULL
		  RETURNING ` + selectCols
	row := r.rawQ(ctx).QueryRow(ctx, q, args...)
	t, err := scanTicket(row)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrNotFound
	}
	return t, err
}

// ChangeStatus usa raw SQL porque los SET son condicionales en runtime.
func (r *pgRepository) ChangeStatus(ctx context.Context, orgID, id uuid.UUID, toStatus string, changedBy uuid.UUID, note string) (*Ticket, error) {
	curr, err := r.Get(ctx, orgID, id)
	if err != nil {
		return nil, err
	}
	if curr.Status == toStatus {
		return curr, nil
	}
	startSet, completeSet := "", ""
	if curr.StartedAt == nil && toStatus == "in_progress" {
		startSet = ", started_at = NOW()"
	}
	if toStatus == "done" || toStatus == "cancelled" {
		completeSet = ", completed_at = NOW()"
	} else if curr.Status == "done" || curr.Status == "cancelled" {
		completeSet = ", completed_at = NULL"
	}
	row := r.rawQ(ctx).QueryRow(ctx,
		`UPDATE project_tickets SET status = $2`+startSet+completeSet+`
		   WHERE id = $1 AND deleted_at IS NULL
		   RETURNING `+selectCols,
		id, toStatus,
	)
	t, err := scanTicket(row)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("change status: %w", err)
	}
	_ = r.q(ctx).InsertStatusHistory(ctx, ticketdb.InsertStatusHistoryParams{
		TicketID:   id,
		FromStatus: &curr.Status,
		ToStatus:   toStatus,
		ChangedBy:  changedBy,
		Note:       stringToPtr(note),
	})
	return t, nil
}

func (r *pgRepository) SoftDelete(ctx context.Context, orgID, id uuid.UUID) error {
	n, err := r.q(ctx).SoftDeleteTicket(ctx, id)
	if err != nil {
		return fmt.Errorf("soft-delete ticket: %w", err)
	}
	if n == 0 {
		return ErrNotFound
	}
	return nil
}

func (r *pgRepository) AddComment(ctx context.Context, ticketID, authorID uuid.UUID, body string) (*Comment, error) {
	row, err := r.q(ctx).InsertComment(ctx, ticketdb.InsertCommentParams{
		TicketID: ticketID,
		AuthorID: authorID,
		BodyMd:   body,
	})
	if err != nil {
		return nil, fmt.Errorf("add comment: %w", err)
	}
	var deletedAt *time.Time
	if row.DeletedAt.Valid {
		t := row.DeletedAt.Time
		deletedAt = &t
	}
	return &Comment{
		ID:         row.ID,
		TicketID:   row.TicketID,
		AuthorID:   row.AuthorID,
		BodyMD:     row.BodyMd,
		ExternalID: row.ExternalID,
		CreatedAt:  row.CreatedAt,
		UpdatedAt:  row.UpdatedAt,
		DeletedAt:  deletedAt,
	}, nil
}

func (r *pgRepository) ListComments(ctx context.Context, ticketID uuid.UUID) ([]*Comment, error) {
	rows, err := r.q(ctx).ListComments(ctx, ticketID)
	if err != nil {
		return nil, fmt.Errorf("list comments: %w", err)
	}
	out := make([]*Comment, 0, len(rows))
	for _, row := range rows {
		var deletedAt *time.Time
		if row.DeletedAt.Valid {
			t := row.DeletedAt.Time
			deletedAt = &t
		}
		out = append(out, &Comment{
			ID:         row.ID,
			TicketID:   row.TicketID,
			AuthorID:   row.AuthorID,
			BodyMD:     row.BodyMd,
			ExternalID: row.ExternalID,
			CreatedAt:  row.CreatedAt,
			UpdatedAt:  row.UpdatedAt,
			DeletedAt:  deletedAt,
		})
	}
	return out, nil
}

func (r *pgRepository) StatusHistory(ctx context.Context, ticketID uuid.UUID) ([]*StatusChange, error) {
	rows, err := r.q(ctx).ListStatusHistory(ctx, ticketID)
	if err != nil {
		return nil, fmt.Errorf("status history: %w", err)
	}
	out := make([]*StatusChange, 0, len(rows))
	for _, row := range rows {
		out = append(out, &StatusChange{
			ID:         row.ID,
			TicketID:   row.TicketID,
			FromStatus: row.FromStatus,
			ToStatus:   row.ToStatus,
			ChangedBy:  row.ChangedBy,
			Note:       row.Note,
			ChangedAt:  row.ChangedAt,
		})
	}
	return out, nil
}

func (r *pgRepository) LinkExternal(ctx context.Context, orgID, id uuid.UUID, link ExternalLink) (*Ticket, error) {
	row, err := r.q(ctx).LinkTicketExternal(ctx, ticketdb.LinkTicketExternalParams{
		ID:               id,
		ExternalProvider: link.Provider,
		ExternalID:       link.ID,
		ExternalUrl:      link.URL,
	})
	if isExternalUniqueViolation(err) {
		return nil, ErrExternalAlreadyLinked
	}
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, err
	}
	return rowToTicket(linkTicketExternalRowAdapter(row)), nil
}

func (r *pgRepository) UnlinkExternal(ctx context.Context, orgID, id uuid.UUID) error {
	n, err := r.q(ctx).UnlinkTicketExternal(ctx, id)
	if err != nil {
		return fmt.Errorf("unlink external: %w", err)
	}
	if n == 0 {
		return ErrNotFound
	}
	return nil
}

// BulkLinkExternal aplica N mappings ticket→external en una sola
// transacción. REQ-58/REQ-59.
func (r *pgRepository) BulkLinkExternal(ctx context.Context, orgID, projectID uuid.UUID, provider string, mappings []BulkLinkMapping) (*BulkLinkResult, error) {
	out := &BulkLinkResult{}

	tx := txctx.TxFromContext(ctx)
	for i, m := range mappings {
		var tid uuid.UUID
		if m.TicketID != uuid.Nil {
			tid = m.TicketID
		} else if m.TicketKey != "" {
			found, err := r.q(ctx).GetTicketIDByKey(ctx, ticketdb.GetTicketIDByKeyParams{
				ProjectID: projectID,
				Key:       m.TicketKey,
			})
			if err != nil {
				out.NotFound = append(out.NotFound, m.TicketKey)
				continue
			}
			tid = found
		} else {
			out.Errors = append(out.Errors, "mapping sin TicketID ni TicketKey")
			continue
		}

		spName := fmt.Sprintf("bulk_link_%d", i)
		if tx != nil {
			if _, err := tx.Exec(ctx, "SAVEPOINT "+spName); err != nil {
				out.Errors = append(out.Errors, fmt.Sprintf("savepoint: %v", err))
				continue
			}
		}
		n, err := r.q(ctx).BulkLinkExternalByID(ctx, ticketdb.BulkLinkExternalByIDParams{
			ID:               tid,
			ExternalProvider: &provider,
			ExternalID:       m.ExternalID,
			ExternalUrl:      m.ExternalURL,
		})
		if err != nil {
			if tx != nil {
				_, _ = tx.Exec(ctx, "ROLLBACK TO SAVEPOINT "+spName)
			}
			if isExternalUniqueViolation(err) {
				out.Errors = append(out.Errors,
					fmt.Sprintf("ticket %s: external_id %q ya está vinculado a otro ticket en esta org", tid, m.ExternalID))
			} else {
				out.Errors = append(out.Errors, fmt.Sprintf("%s: %v", tid, err))
			}
			continue
		}
		if n == 0 {
			if tx != nil {
				_, _ = tx.Exec(ctx, "ROLLBACK TO SAVEPOINT "+spName)
			}
			out.NotFound = append(out.NotFound, tid.String())
			continue
		}
		if tx != nil {
			_, _ = tx.Exec(ctx, "RELEASE SAVEPOINT "+spName)
		}
		out.Linked++
	}
	return out, nil
}

// FindByExternal busca un ticket por (provider, external_id). REQ-58.
func (r *pgRepository) FindByExternal(ctx context.Context, orgID uuid.UUID, provider, externalID string) (*Ticket, error) {
	row, err := r.q(ctx).FindTicketByExternal(ctx, ticketdb.FindTicketByExternalParams{
		ExternalProvider: &provider,
		ExternalID:       &externalID,
	})
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, err
	}
	return rowToTicket(findTicketByExternalRowAdapter(row)), nil
}

// Claim adquiere un soft lock. REQ-63.
func (r *pgRepository) Claim(ctx context.Context, orgID, ticketID, userID uuid.UUID, ttlMinutes int) (*Ticket, error) {
	if ttlMinutes <= 0 || ttlMinutes > 240 {
		ttlMinutes = 30
	}

	curr, err := r.Get(ctx, orgID, ticketID)
	if err != nil {
		return nil, err
	}
	if curr.LockedBy != nil && *curr.LockedBy != userID &&
		curr.LockedUntil != nil && curr.LockedUntil.After(timeNow()) {
		return nil, ErrLockedByOther
	}
	row, err := r.q(ctx).ClaimTicket(ctx, ticketdb.ClaimTicketParams{
		ID:         ticketID,
		LockedBy:   &userID,
		TtlMinutes: int32(ttlMinutes),
	})
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, err
	}
	return rowToTicket(claimTicketRowAdapter(row)), nil
}

// Release suelta el lock. REQ-63.
func (r *pgRepository) Release(ctx context.Context, orgID, ticketID, userID uuid.UUID) (*Ticket, error) {
	curr, err := r.Get(ctx, orgID, ticketID)
	if err != nil {
		return nil, err
	}
	if curr.LockedBy == nil {
		return curr, nil
	}
	if *curr.LockedBy != userID &&
		curr.LockedUntil != nil && curr.LockedUntil.After(timeNow()) {
		return nil, ErrLockedByOther
	}
	row, err := r.q(ctx).ReleaseTicket(ctx, ticketID)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, err
	}
	return rowToTicket(releaseTicketRowAdapter(row)), nil
}

// timeNow centraliza el reloj para que tests puedan mockearlo.
var timeNow = func() time.Time { return time.Now() }
