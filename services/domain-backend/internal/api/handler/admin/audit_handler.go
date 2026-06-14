package admin

import (
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
)

type AuditHandler struct {
	Pool   *pgxpool.Pool
	Now    func() time.Time
}

func NewAuditHandler(pool *pgxpool.Pool) *AuditHandler {
	return &AuditHandler{Pool: pool, Now: time.Now}
}

type AuditCursor struct {
	OccurredAt time.Time `json:"ts"`
	ID         int64     `json:"id"`
}

func EncodeCursor(c AuditCursor) string {
	b, _ := json.Marshal(c)
	return base64.RawURLEncoding.EncodeToString(b)
}

func DecodeCursor(s string) (*AuditCursor, error) {
	b, err := base64.RawURLEncoding.DecodeString(s)
	if err != nil {
		return nil, fmt.Errorf("invalid cursor: %w", err)
	}
	var c AuditCursor
	if err := json.Unmarshal(b, &c); err != nil {
		return nil, fmt.Errorf("invalid cursor: %w", err)
	}
	return &c, nil
}

type AuditListResponse struct {
	Events     []AuditEvent `json:"events"`
	NextCursor string       `json:"next_cursor,omitempty"`
	HasMore    bool         `json:"has_more"`
}

type AuditEvent struct {
	ID         int64           `json:"id"`
	ActorID    *uuid.UUID      `json:"actor_user_id,omitempty"`
	ActorEmail string          `json:"actor_email,omitempty"`
	Action     string          `json:"action"`
	Resource   string          `json:"resource"`
	Metadata   json.RawMessage `json:"metadata,omitempty"`
	OccurredAt time.Time       `json:"occurred_at"`
}

type AuditQueryParams struct {
	OrgID    string
	Since    string
	Until    string
	Action   string
	Resource string
	Cursor   string
	Limit    int
}

func (h *AuditHandler) ListGET(w http.ResponseWriter, r *http.Request) {
	params := ParseQueryParams(r)

	orgID, err := uuid.Parse(params.OrgID)
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid_org_id", "org_id must be a valid UUID")
		return
	}

	limit := params.Limit
	if limit <= 0 {
		limit = 50
	}
	if limit > 200 {
		limit = 200
	}

	where := []string{"origin_org_id = $1"}
	args := []any{orgID}
	argN := 1

	addCond := func(cond string, v any) {
		argN++
		where = append(where, cond)
		args = append(args, v)
	}

	if params.Since != "" {
		t, err := time.Parse(time.RFC3339, params.Since)
		if err == nil {
			addCond(fmt.Sprintf("occurred_at >= $%d", argN+1), t)
		}
	}

	if params.Until != "" {
		t, err := time.Parse(time.RFC3339, params.Until)
		if err == nil {
			addCond(fmt.Sprintf("occurred_at <= $%d", argN+1), t)
		}
	}

	if params.Action != "" {
		action := params.Action
		if strings.Contains(action, "*") {
			like := strings.ReplaceAll(action, "*", "%")
			addCond(fmt.Sprintf("action LIKE $%d", argN+1), like)
		} else {
			addCond(fmt.Sprintf("action = $%d", argN+1), action)
		}
	}

	if params.Resource != "" {
		addCond(fmt.Sprintf("(entity_type || '/' || entity_id::text) = $%d", argN+1), params.Resource)
	}

	if params.Cursor != "" {
		c, err := DecodeCursor(params.Cursor)
		if err == nil {
			addCond(fmt.Sprintf("(occurred_at, id) < ($%d::timestamptz, $%d::bigint)", argN+1, argN+2), c.OccurredAt)
			argN++
			args = append(args, c.ID)
		}
	}

	query := fmt.Sprintf(`
		SELECT id, actor_id, action,
		       COALESCE(entity_type || '/' || entity_id::text, ''),
		       old_values, occurred_at
		FROM audit_log
		WHERE %s
		ORDER BY occurred_at DESC, id DESC
		LIMIT $%d
	`, strings.Join(where, " AND "), argN+1)
	args = append(args, limit)

	rows, err := h.Pool.Query(r.Context(), query, args...)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "query_failed", err.Error())
		return
	}
	defer rows.Close()

	var events []AuditEvent
	for rows.Next() {
		var e AuditEvent
		var oldValues json.RawMessage
		if err := rows.Scan(&e.ID, &e.ActorID, &e.Action, &e.Resource, &oldValues, &e.OccurredAt); err != nil {
			writeError(w, http.StatusInternalServerError, "scan_failed", err.Error())
			return
		}
		if oldValues != nil {
			e.Metadata = oldValues
		}
		events = append(events, e)
	}

	if err := rows.Err(); err != nil {
		writeError(w, http.StatusInternalServerError, "rows_error", err.Error())
		return
	}

	resp := AuditListResponse{Events: events}
	if len(events) == limit {
		last := events[len(events)-1]
		resp.NextCursor = EncodeCursor(AuditCursor{OccurredAt: last.OccurredAt, ID: last.ID})
		resp.HasMore = true
	}

	writeJSON(w, http.StatusOK, resp)
}

func ParseQueryParams(r *http.Request) AuditQueryParams {
	q := r.URL.Query()
	limit := 50
	if l, err := ParseIntParam(q.Get("limit")); err == nil && l > 0 {
		limit = l
	}
	return AuditQueryParams{
		OrgID:    q.Get("org_id"),
		Since:    q.Get("since"),
		Until:    q.Get("until"),
		Action:   q.Get("action"),
		Resource: q.Get("resource"),
		Cursor:   q.Get("cursor"),
		Limit:    limit,
	}
}

func ParseIntParam(s string) (int, error) {
	if s == "" {
		return 0, errors.New("empty")
	}
	n := 0
	for _, c := range s {
		if c < '0' || c > '9' {
			return 0, fmt.Errorf("invalid number: %s", s)
		}
		n = n*10 + int(c-'0')
	}
	return n, nil
}

func writeJSON(w http.ResponseWriter, status int, body any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(body)
}

func writeError(w http.ResponseWriter, status int, code, msg string) {
	writeJSON(w, status, map[string]any{
		"error": map[string]any{"code": code, "message": msg},
	})
}
