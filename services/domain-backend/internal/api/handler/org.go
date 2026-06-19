package handler

import (
	"net/http"
	"regexp"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	"nunezlagos/domain/internal/audit"
	"nunezlagos/domain/internal/auth/apikey"
	"nunezlagos/domain/internal/auth/rbac"
)

func principal(r *http.Request) (*apikey.Principal, bool) {
	return apikey.FromContext(r.Context())
}

func parseUUID(s string) (uuid.UUID, error) {
	return uuid.Parse(s)
}

// roleOwner / roleAdmin: single-org global roles. La entidad organization se
// removió (single-org); los roles viven en users.role y se validan acá contra
// el catálogo rbac.
var (
	roleOwner = string(rbac.RoleOwner)
	roleAdmin = string(rbac.RoleAdmin)
)

// memberEmailRegex validación de formato (sin DNS).
var memberEmailRegex = regexp.MustCompile(`^[a-zA-Z0-9._%+\-]+@[a-zA-Z0-9.\-]+\.[a-zA-Z]{2,}$`)

// allowedMemberRoles cierra el conjunto de roles aceptados al crear members.
var allowedMemberRoles = map[string]bool{
	"owner":      true,
	"admin":      true,
	"maintainer": true,
	"member":     true,
	"viewer":     true,
}

// member es el snapshot de lectura de un user (single-org global).
type member struct {
	UserID   uuid.UUID `json:"user_id"`
	Email    string    `json:"email"`
	Name     string    `json:"name"`
	Role     string    `json:"role"`
	JoinedAt time.Time `json:"joined_at"`
}

// addMemberWithKeyBody body de POST /api/v1/organizations/{id}/members (issue-36.1).
type addMemberWithKeyBody struct {
	Email string `json:"email"`
	Name  string `json:"name,omitempty"`
	Role  string `json:"role"`
}

// addMemberWithKey crea user + api_key en una sola tx, sin pasar por
// auth_invitations/OTP/email. Solo accesible para admin/owner.
// El plaintext de la key se devuelve UNA SOLA VEZ en la response.
//
// Single-org: ya no existe la entidad organization. El user y la api_key se
// crean globales (sin organization_id).
func (a *API) addMemberWithKey(w http.ResponseWriter, r *http.Request) {
	p, _ := principal(r)
	if p == nil {
		writeError(w, http.StatusUnauthorized, "unauthorized", "")
		return
	}
	if p.Role != roleOwner && p.Role != roleAdmin {
		writeError(w, http.StatusForbidden, "forbidden", "owners/admins only")
		return
	}
	actorID, err := uuid.Parse(p.UserID)
	if err != nil {
		writeError(w, http.StatusUnauthorized, "unauthorized", "invalid actor id")
		return
	}

	var b addMemberWithKeyBody
	if err := decodeJSON(r, &b); err != nil {
		writeError(w, http.StatusBadRequest, "invalid_body", err.Error())
		return
	}
	if b.Email == "" || b.Role == "" {
		writeError(w, http.StatusUnprocessableEntity, "validation_failed", "email and role are required")
		return
	}

	email := strings.ToLower(strings.TrimSpace(b.Email))
	if !memberEmailRegex.MatchString(email) {
		writeError(w, http.StatusUnprocessableEntity, "validation_failed", "email format invalid")
		return
	}
	if !allowedMemberRoles[b.Role] {
		writeError(w, http.StatusUnprocessableEntity, "invalid_role",
			"role must be one of: owner, admin, maintainer, member, viewer")
		return
	}

	plaintext, prefix, hash, err := apikey.Generate("live")
	if err != nil {
		writeError(w, http.StatusInternalServerError, "create_member", err.Error())
		return
	}

	ctx := r.Context()
	tx, err := a.Pool.BeginTx(ctx, pgx.TxOptions{})
	if err != nil {
		writeError(w, http.StatusInternalServerError, "create_member", err.Error())
		return
	}
	defer func() { _ = tx.Rollback(ctx) }()

	var m member
	err = tx.QueryRow(ctx,
		`INSERT INTO users (email, name, role)
		 VALUES ($1, NULLIF($2, ''), $3)
		 RETURNING id, email, COALESCE(name,''), role, created_at`,
		email, b.Name, b.Role,
	).Scan(&m.UserID, &m.Email, &m.Name, &m.Role, &m.JoinedAt)
	if err != nil {
		if isMemberEmailUniqueViolation(err) {
			writeError(w, http.StatusConflict, "email_taken", "email already in use")
			return
		}
		writeError(w, http.StatusInternalServerError, "create_member", err.Error())
		return
	}

	keyID := uuid.New()
	_, err = tx.Exec(ctx,
		`INSERT INTO auth_api_keys (id, user_id, key_hash, key_prefix,
		                        name, environment, expires_at)
		 VALUES ($1, $2, $3, $4, 'default', 'live', NULL)`,
		keyID, m.UserID, hash, prefix,
	)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "create_member", err.Error())
		return
	}

	if err := tx.Commit(ctx); err != nil {
		writeError(w, http.StatusInternalServerError, "create_member", err.Error())
		return
	}

	if a.Audit != nil {
		audit.RecordOrLog(ctx, a.Audit, audit.Event{
			ActorID:    &actorID,
			ActorType:  audit.ActorUser,
			Action:     "member.created_with_key",
			EntityType: "user",
			EntityID:   &m.UserID,
			NewValues: map[string]any{
				"email":      email,
				"role":       b.Role,
				"key_prefix": prefix,
			},
		})
	}

	w.Header().Set("Location", "/api/v1/users/"+m.UserID.String())
	writeData(w, http.StatusCreated, map[string]any{
		"user": map[string]any{
			"id":        m.UserID,
			"email":     m.Email,
			"name":      m.Name,
			"role":      m.Role,
			"joined_at": m.JoinedAt,
		},
		"api_key":    plaintext,
		"api_key_id": keyID,
		"key_prefix": prefix,
	})
}

// listMembers GET /api/v1/organizations/{id}/members — single-org: lista todos
// los users activos directamente desde la tabla users (sin organization_id).
func (a *API) listMembers(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	rows, err := a.Pool.Query(ctx,
		`SELECT id, email, COALESCE(name,''), role, created_at
		 FROM users
		 WHERE deleted_at IS NULL
		 ORDER BY created_at ASC`)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "list", err.Error())
		return
	}
	defer rows.Close()
	out := []member{}
	for rows.Next() {
		var m member
		if err := rows.Scan(&m.UserID, &m.Email, &m.Name, &m.Role, &m.JoinedAt); err != nil {
			writeError(w, http.StatusInternalServerError, "list", err.Error())
			return
		}
		out = append(out, m)
	}
	if err := rows.Err(); err != nil {
		writeError(w, http.StatusInternalServerError, "list", err.Error())
		return
	}
	writeData(w, http.StatusOK, out)
}

// isMemberEmailUniqueViolation detecta unique constraint sobre users.email.
func isMemberEmailUniqueViolation(err error) bool {
	if err == nil {
		return false
	}
	msg := err.Error()
	if !strings.Contains(msg, "duplicate key") {
		return false
	}
	return strings.Contains(msg, "email") ||
		(strings.Contains(msg, "users_") && strings.Contains(msg, "uniq"))
}
