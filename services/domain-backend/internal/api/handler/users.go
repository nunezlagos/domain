package handler

import (
	"context"
	"net/http"
	"strconv"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	"nunezlagos/domain/internal/store/txctx"
)

// REQ-75: listar users de la org actual.
// Lo consume el dashboard para llenar selects (assignee, reporter, etc).
//
// GET /api/v1/users
//   ?role=admin|developer|pm|qa|viewer   filtro por users.role (legacy)
//   ?limit=N   default 100, max 500
//   ?offset=N  default 0
//
// El listado respeta RLS via la tx del middleware — solo retorna users
// de la misma organización que el principal.

type userListItem struct {
	ID    uuid.UUID `json:"id"`
	Email string    `json:"email"`
	Name  string    `json:"name"`
	Role  string    `json:"role"`
}

// rowsQuerier abstrae pgx.Tx y pgxpool.Pool (ambos implementan Query).
type rowsQuerier interface {
	Query(ctx context.Context, sql string, args ...any) (pgx.Rows, error)
}

func (a *API) listUsers(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	orgID := a.orgID(ctx)
	if orgID == uuid.Nil {
		writeError(w, http.StatusUnauthorized, "unauthorized", "")
		return
	}

	limit := atoiDefault(r.URL.Query().Get("limit"), 100)
	if limit <= 0 || limit > 500 {
		limit = 100
	}
	offset := atoiDefault(r.URL.Query().Get("offset"), 0)
	if offset < 0 {
		offset = 0
	}
	roleFilter := r.URL.Query().Get("role")

	var q rowsQuerier
	if tx := txctx.TxFromContext(ctx); tx != nil {
		q = tx
	} else if a.Pool != nil {
		q = a.Pool
	} else {
		writeError(w, http.StatusServiceUnavailable, "no_db", "")
		return
	}

	sql := `SELECT id, email, name, role
	          FROM users
	         WHERE organization_id = $1 AND deleted_at IS NULL`
	args := []any{orgID}
	if roleFilter != "" {
		sql += ` AND role = $2`
		args = append(args, roleFilter)
	}
	sql += ` ORDER BY name NULLS LAST, email LIMIT ` + strconv.Itoa(limit) +
		` OFFSET ` + strconv.Itoa(offset)

	rows, err := q.Query(ctx, sql, args...)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "list_users", err.Error())
		return
	}
	defer rows.Close()

	items := []userListItem{}
	for rows.Next() {
		var u userListItem
		if err := rows.Scan(&u.ID, &u.Email, &u.Name, &u.Role); err != nil {
			writeError(w, http.StatusInternalServerError, "scan_users", err.Error())
			return
		}
		items = append(items, u)
	}
	if err := rows.Err(); err != nil {
		writeError(w, http.StatusInternalServerError, "iter_users", err.Error())
		return
	}
	writeData(w, http.StatusOK, map[string]any{"items": items})
}

func atoiDefault(s string, def int) int {
	if s == "" {
		return def
	}
	if n, err := strconv.Atoi(s); err == nil {
		return n
	}
	return def
}
