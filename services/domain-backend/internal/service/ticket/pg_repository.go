package ticket

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"

	"nunezlagos/domain/internal/store/txctx"
)

type pgRepository struct {
	pool *pgxpool.Pool
}

func NewPgRepository(pool *pgxpool.Pool) Repository {
	return &pgRepository{pool: pool}
}

type querier interface {
	QueryRow(ctx context.Context, sql string, args ...any) pgx.Row
	Query(ctx context.Context, sql string, args ...any) (pgx.Rows, error)
	Exec(ctx context.Context, sql string, args ...any) (pgconn.CommandTag, error)
}

func (r *pgRepository) q(ctx context.Context) querier {
	if tx := txctx.TxFromContext(ctx); tx != nil {
		return tx
	}
	return r.pool
}

const selectCols = `id, organization_id, project_id, client_id, key, number,
		title, COALESCE(description_md,''), issue_type, status, priority,
		assignee_id, reporter_id, labels,
		COALESCE(external_provider,''), COALESCE(external_id,''),
		COALESCE(external_url,''), external_synced_at,
		parent_id, linked_issue_id, estimated_hours, actual_hours,
		due_date, started_at, completed_at,
		created_at, updated_at, deleted_at`

func scanTicket(row pgx.Row) (*Ticket, error) {
	var t Ticket
	if err := row.Scan(
		&t.ID, &t.OrganizationID, &t.ProjectID, &t.ClientID, &t.Key, &t.Number,
		&t.Title, &t.DescriptionMD, &t.IssueType, &t.Status, &t.Priority,
		&t.AssigneeID, &t.ReporterID, &t.Labels,
		&t.ExternalProvider, &t.ExternalID, &t.ExternalURL, &t.ExternalSyncedAt,
		&t.ParentID, &t.LinkedIssueID, &t.EstimatedHours, &t.ActualHours,
		&t.DueDate, &t.StartedAt, &t.CompletedAt,
		&t.CreatedAt, &t.UpdatedAt, &t.DeletedAt,
	); err != nil {
		return nil, err
	}
	// REQ-58: display_key = external_id si está, sino key interno.
	t.DisplayKey = t.Key
	if t.ExternalID != "" {
		t.DisplayKey = t.ExternalID
	}
	return &t, nil
}

// LinkIssue setea o limpia linked_issue_id. issueID=nil → desvinculación.
func (r *pgRepository) LinkIssue(ctx context.Context, orgID, ticketID uuid.UUID, issueID *uuid.UUID) (*Ticket, error) {
	row := r.q(ctx).QueryRow(ctx,
		`UPDATE project_tickets SET linked_issue_id = $3
		   WHERE organization_id = $1 AND id = $2 AND deleted_at IS NULL
		   RETURNING `+selectCols,
		orgID, ticketID, issueID,
	)
	t, err := scanTicket(row)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrNotFound
	}
	return t, err
}

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

func (r *pgRepository) nextNumber(ctx context.Context, q querier, orgID, projectID uuid.UUID) (int, error) {
	var n int
	err := q.QueryRow(ctx,
		`SELECT COALESCE(MAX(number), 0) + 1
		   FROM project_tickets
		   WHERE organization_id = $1 AND project_id = $2`,
		orgID, projectID,
	).Scan(&n)
	if err != nil {
		return 0, fmt.Errorf("next number: %w", err)
	}
	return n, nil
}

func (r *pgRepository) Insert(ctx context.Context, in CreateInput) (*Ticket, error) {
	prefix := keyPrefix(in.ProjectSlug)
	for attempt := 0; attempt < 2; attempt++ {
		num, err := r.nextNumber(ctx, r.q(ctx), in.OrganizationID, in.ProjectID)
		if err != nil {
			return nil, err
		}
		key := fmt.Sprintf("%s-%d", prefix, num)
		// REQ-58: external_* opcional al crear. Si vienen, link en el
		// mismo INSERT + external_synced_at=NOW(); sino, NULLs.
		var extSyncedAt any
		if in.ExternalProvider != "" {
			extSyncedAt = "now"
		}
		row := r.q(ctx).QueryRow(ctx,
			`INSERT INTO project_tickets
			   (organization_id, project_id, client_id, key, number,
			    title, description_md, issue_type, priority,
			    assignee_id, reporter_id, labels, parent_id,
			    estimated_hours, due_date,
			    external_provider, external_id, external_url, external_synced_at)
			 VALUES ($1,$2,$3,$4,$5,$6,NULLIF($7,''),$8,$9,$10,$11,$12,$13,$14,$15,
			         NULLIF($16,''), NULLIF($17,''), NULLIF($18,''),
			         CASE WHEN $19::text = 'now' THEN NOW() ELSE NULL END)
			 RETURNING `+selectCols,
			in.OrganizationID, in.ProjectID, in.ClientID, key, num,
			in.Title, in.DescriptionMD, in.IssueType, in.Priority,
			in.AssigneeID, in.ReporterID, in.Labels, in.ParentID,
			in.EstimatedHours, in.DueDate,
			in.ExternalProvider, in.ExternalID, in.ExternalURL, extSyncedAt,
		)
		t, err := scanTicket(row)
		if isUniqueViolation(err) {
			continue
		}
		if err != nil {
			return nil, fmt.Errorf("insert ticket: %w", err)
		}
		_, _ = r.q(ctx).Exec(ctx,
			`INSERT INTO project_ticket_status_history
			   (ticket_id, from_status, to_status, changed_by, note)
			 VALUES ($1, NULL, $2, $3, 'created')`,
			t.ID, t.Status, in.ReporterID,
		)
		return t, nil
	}
	return nil, fmt.Errorf("insert ticket: tras 2 reintentos sigue habiendo race condition")
}

func (r *pgRepository) Get(ctx context.Context, orgID, id uuid.UUID) (*Ticket, error) {
	row := r.q(ctx).QueryRow(ctx,
		`SELECT `+selectCols+`
		   FROM project_tickets
		   WHERE organization_id = $1 AND id = $2 AND deleted_at IS NULL`,
		orgID, id,
	)
	t, err := scanTicket(row)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrNotFound
	}
	return t, err
}

func (r *pgRepository) GetByKey(ctx context.Context, orgID, projectID uuid.UUID, key string) (*Ticket, error) {
	row := r.q(ctx).QueryRow(ctx,
		`SELECT `+selectCols+`
		   FROM project_tickets
		   WHERE organization_id = $1 AND project_id = $2 AND key = $3 AND deleted_at IS NULL`,
		orgID, projectID, strings.ToUpper(key),
	)
	t, err := scanTicket(row)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrNotFound
	}
	return t, err
}

func (r *pgRepository) List(ctx context.Context, orgID uuid.UUID, filter ListFilter) ([]*Ticket, int64, error) {
	conds := []string{"organization_id = $1", "deleted_at IS NULL"}
	args := []any{orgID}
	idx := 2
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
	if err := r.q(ctx).QueryRow(ctx,
		`SELECT COUNT(*) FROM project_tickets `+where, args...,
	).Scan(&total); err != nil {
		return nil, 0, fmt.Errorf("count tickets: %w", err)
	}

	limit := filter.Limit
	if limit <= 0 || limit > 200 {
		limit = 50
	}
	args = append(args, limit, filter.Offset)
	rows, err := r.q(ctx).Query(ctx,
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

func (r *pgRepository) Update(ctx context.Context, orgID, id uuid.UUID, in UpdateInput) (*Ticket, error) {
	sets := []string{}
	args := []any{orgID, id}
	idx := 3
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
		` WHERE organization_id = $1 AND id = $2 AND deleted_at IS NULL
		  RETURNING ` + selectCols
	row := r.q(ctx).QueryRow(ctx, q, args...)
	t, err := scanTicket(row)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrNotFound
	}
	return t, err
}

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
	row := r.q(ctx).QueryRow(ctx,
		`UPDATE project_tickets SET status = $3`+startSet+completeSet+`
		   WHERE organization_id = $1 AND id = $2 AND deleted_at IS NULL
		   RETURNING `+selectCols,
		orgID, id, toStatus,
	)
	t, err := scanTicket(row)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("change status: %w", err)
	}
	_, _ = r.q(ctx).Exec(ctx,
		`INSERT INTO project_ticket_status_history
		   (ticket_id, from_status, to_status, changed_by, note)
		 VALUES ($1, $2, $3, $4, NULLIF($5,''))`,
		id, curr.Status, toStatus, changedBy, note,
	)
	return t, nil
}

func (r *pgRepository) SoftDelete(ctx context.Context, orgID, id uuid.UUID) error {
	tag, err := r.q(ctx).Exec(ctx,
		`UPDATE project_tickets SET deleted_at = NOW()
		   WHERE organization_id = $1 AND id = $2 AND deleted_at IS NULL`,
		orgID, id,
	)
	if err != nil {
		return fmt.Errorf("soft-delete ticket: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return ErrNotFound
	}
	return nil
}

func (r *pgRepository) AddComment(ctx context.Context, ticketID, authorID uuid.UUID, body string) (*Comment, error) {
	var c Comment
	err := r.q(ctx).QueryRow(ctx,
		`INSERT INTO project_ticket_comments (ticket_id, author_id, body_md)
		 VALUES ($1, $2, $3)
		 RETURNING id, ticket_id, author_id, body_md, COALESCE(external_id,''),
		           created_at, updated_at, deleted_at`,
		ticketID, authorID, body,
	).Scan(&c.ID, &c.TicketID, &c.AuthorID, &c.BodyMD, &c.ExternalID,
		&c.CreatedAt, &c.UpdatedAt, &c.DeletedAt)
	if err != nil {
		return nil, fmt.Errorf("add comment: %w", err)
	}
	return &c, nil
}

func (r *pgRepository) ListComments(ctx context.Context, ticketID uuid.UUID) ([]*Comment, error) {
	rows, err := r.q(ctx).Query(ctx,
		`SELECT id, ticket_id, author_id, body_md, COALESCE(external_id,''),
		        created_at, updated_at, deleted_at
		   FROM project_ticket_comments
		   WHERE ticket_id = $1 AND deleted_at IS NULL
		   ORDER BY created_at ASC`,
		ticketID,
	)
	if err != nil {
		return nil, fmt.Errorf("list comments: %w", err)
	}
	defer rows.Close()
	out := []*Comment{}
	for rows.Next() {
		var c Comment
		if err := rows.Scan(&c.ID, &c.TicketID, &c.AuthorID, &c.BodyMD,
			&c.ExternalID, &c.CreatedAt, &c.UpdatedAt, &c.DeletedAt); err != nil {
			return nil, err
		}
		out = append(out, &c)
	}
	return out, rows.Err()
}

func (r *pgRepository) StatusHistory(ctx context.Context, ticketID uuid.UUID) ([]*StatusChange, error) {
	rows, err := r.q(ctx).Query(ctx,
		`SELECT id, ticket_id, COALESCE(from_status,''), to_status,
		        changed_by, COALESCE(note,''), changed_at
		   FROM project_ticket_status_history
		   WHERE ticket_id = $1 ORDER BY changed_at ASC`,
		ticketID,
	)
	if err != nil {
		return nil, fmt.Errorf("status history: %w", err)
	}
	defer rows.Close()
	out := []*StatusChange{}
	for rows.Next() {
		var s StatusChange
		if err := rows.Scan(&s.ID, &s.TicketID, &s.FromStatus, &s.ToStatus,
			&s.ChangedBy, &s.Note, &s.ChangedAt); err != nil {
			return nil, err
		}
		out = append(out, &s)
	}
	return out, rows.Err()
}

func (r *pgRepository) LinkExternal(ctx context.Context, orgID, id uuid.UUID, link ExternalLink) (*Ticket, error) {
	row := r.q(ctx).QueryRow(ctx,
		`UPDATE project_tickets
		   SET external_provider = NULLIF($3,''),
		       external_id       = NULLIF($4,''),
		       external_url      = NULLIF($5,''),
		       external_synced_at = NOW()
		   WHERE organization_id = $1 AND id = $2 AND deleted_at IS NULL
		   RETURNING `+selectCols,
		orgID, id, link.Provider, link.ID, link.URL,
	)
	t, err := scanTicket(row)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrNotFound
	}
	return t, err
}

func (r *pgRepository) UnlinkExternal(ctx context.Context, orgID, id uuid.UUID) error {
	tag, err := r.q(ctx).Exec(ctx,
		`UPDATE project_tickets
		   SET external_provider = NULL, external_id = NULL,
		       external_url = NULL, external_synced_at = NULL
		   WHERE organization_id = $1 AND id = $2 AND deleted_at IS NULL`,
		orgID, id,
	)
	if err != nil {
		return fmt.Errorf("unlink external: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return ErrNotFound
	}
	return nil
}

func nullIfEmpty(s string) any {
	if s == "" {
		return nil
	}
	return s
}

// BulkLinkExternal aplica N mappings ticket→external en una sola
// transacción. Lookup por TicketID si está, sino por TicketKey.
// Errors per-item se acumulan en el Report; no aborta el batch.
// REQ-58 — preparación pre-Jira.
func (r *pgRepository) BulkLinkExternal(ctx context.Context, orgID, projectID uuid.UUID, provider string, mappings []BulkLinkMapping) (*BulkLinkResult, error) {
	out := &BulkLinkResult{}
	for _, m := range mappings {
		// Resolver ticket_id por id o por key
		var tid uuid.UUID
		if m.TicketID != uuid.Nil {
			tid = m.TicketID
		} else if m.TicketKey != "" {
			var found uuid.UUID
			if err := r.q(ctx).QueryRow(ctx,
				`SELECT id FROM project_tickets
				   WHERE organization_id=$1 AND project_id=$2 AND key=$3 AND deleted_at IS NULL`,
				orgID, projectID, m.TicketKey,
			).Scan(&found); err != nil {
				out.NotFound = append(out.NotFound, m.TicketKey)
				continue
			}
			tid = found
		} else {
			out.Errors = append(out.Errors, "mapping sin TicketID ni TicketKey")
			continue
		}
		tag, err := r.q(ctx).Exec(ctx,
			`UPDATE project_tickets
			   SET external_provider = $3,
			       external_id       = NULLIF($4,''),
			       external_url      = NULLIF($5,''),
			       external_synced_at = NOW()
			   WHERE organization_id = $1 AND id = $2 AND deleted_at IS NULL`,
			orgID, tid, provider, m.ExternalID, m.ExternalURL,
		)
		if err != nil {
			out.Errors = append(out.Errors, fmt.Sprintf("%s: %v", tid, err))
			continue
		}
		if tag.RowsAffected() == 0 {
			out.NotFound = append(out.NotFound, tid.String())
			continue
		}
		out.Linked++
	}
	return out, nil
}

// FindByExternal busca un ticket por (provider, external_id). Útil para
// el webhook receiver de Jira: encuentra el ticket local que mapea al
// issue externo. REQ-58.
func (r *pgRepository) FindByExternal(ctx context.Context, orgID uuid.UUID, provider, externalID string) (*Ticket, error) {
	row := r.q(ctx).QueryRow(ctx,
		`SELECT `+selectCols+`
		   FROM project_tickets
		   WHERE organization_id = $1 AND external_provider = $2
		     AND external_id = $3 AND deleted_at IS NULL
		   LIMIT 1`,
		orgID, provider, externalID,
	)
	t, err := scanTicket(row)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrNotFound
	}
	return t, err
}
