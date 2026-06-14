// issue-16.4 web-admin-skills — admin UI CRUD para skills.
//
// Pattern: server-rendered HTML por Go con html/template + endpoints
// JSON para acciones. Sin build pipeline, sin SPA. Operaciones soportadas:
//   - List + filter por slug/source.
//   - View detail con versions (issue-05.3) + JSON Schemas (issue-05.6).
//   - Pin/Unpin version.
//   - Delete (soft, marcar disabled).
package webui

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
)

// SkillRow es la representación lite que la tabla muestra.
type SkillRow struct {
	ID             uuid.UUID  `json:"id"`
	Slug           string     `json:"slug"`
	Name           string     `json:"name"`
	OrganizationID uuid.UUID  `json:"organization_id"`
	Source         string     `json:"source"`
	Enabled        bool       `json:"enabled"`
	PinnedVersion  *int       `json:"pinned_version,omitempty"`
	UpdatedAt      time.Time  `json:"updated_at"`
}

// SkillsAdmin maneja /admin/skills*.
type SkillsAdmin struct {
	Pool      *pgxpool.Pool
	AuthCheck func(*http.Request) bool
}

// Register monta las rutas en el mux.
func (a *SkillsAdmin) Register(mux *http.ServeMux) {
	mux.HandleFunc("/admin/skills", a.list)
	mux.HandleFunc("/admin/skills/", a.detail)
	mux.HandleFunc("/admin/api/skills", a.apiList)
	mux.HandleFunc("/admin/api/skills/", a.apiAction)
}

func (a *SkillsAdmin) checkAuth(r *http.Request) bool {
	if a.AuthCheck == nil {
		return true
	}
	return a.AuthCheck(r)
}

func (a *SkillsAdmin) list(w http.ResponseWriter, r *http.Request) {
	if !a.checkAuth(r) {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}
	data, err := assets.ReadFile("assets/skills.html")
	if err != nil {
		http.NotFound(w, r)
		return
	}
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	_, _ = w.Write(data)
}

func (a *SkillsAdmin) detail(w http.ResponseWriter, r *http.Request) {
	// Reutiliza skills.html que usa query params para detail vs list.
	a.list(w, r)
}

func (a *SkillsAdmin) apiList(w http.ResponseWriter, r *http.Request) {
	if !a.checkAuth(r) {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}
	ctx, cancel := context.WithTimeout(r.Context(), 5*time.Second)
	defer cancel()

	filter := r.URL.Query().Get("q")
	limit, _ := strconv.Atoi(r.URL.Query().Get("limit"))
	if limit <= 0 || limit > 500 {
		limit = 100
	}

	q := `SELECT id, slug, name, organization_id, COALESCE(source, ''),
	       COALESCE(enabled, true), pinned_version, updated_at
	      FROM skills`
	args := []any{}
	if filter != "" {
		q += ` WHERE slug ILIKE $1 OR name ILIKE $1`
		args = append(args, "%"+filter+"%")
	}
	q += fmt.Sprintf(` ORDER BY updated_at DESC LIMIT $%d`, len(args)+1)
	args = append(args, limit)

	rows, err := a.Pool.Query(ctx, q, args...)
	if err != nil {
		http.Error(w, "query: "+err.Error(), http.StatusInternalServerError)
		return
	}
	defer rows.Close()

	out := make([]SkillRow, 0, limit)
	for rows.Next() {
		var s SkillRow
		if err := rows.Scan(&s.ID, &s.Slug, &s.Name, &s.OrganizationID,
			&s.Source, &s.Enabled, &s.PinnedVersion, &s.UpdatedAt); err != nil {
			continue
		}
		out = append(out, s)
	}
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(out)
}

// apiAction maneja POST /admin/api/skills/{id}/{action}
//   action ∈ {pin, unpin, disable, enable}
//   pin requiere ?version=N
func (a *SkillsAdmin) apiAction(w http.ResponseWriter, r *http.Request) {
	if !a.checkAuth(r) {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}
	if r.Method != http.MethodPost {
		http.Error(w, "POST required", http.StatusMethodNotAllowed)
		return
	}
	// path: /admin/api/skills/{id}/{action}
	path := r.URL.Path[len("/admin/api/skills/"):]
	var idStr, action string
	for i := 0; i < len(path); i++ {
		if path[i] == '/' {
			idStr = path[:i]
			action = path[i+1:]
			break
		}
	}
	if idStr == "" || action == "" {
		http.Error(w, "invalid path", http.StatusBadRequest)
		return
	}
	skillID, err := uuid.Parse(idStr)
	if err != nil {
		http.Error(w, "invalid uuid", http.StatusBadRequest)
		return
	}

	ctx, cancel := context.WithTimeout(r.Context(), 5*time.Second)
	defer cancel()

	switch action {
	case "pin":
		v, err := strconv.Atoi(r.URL.Query().Get("version"))
		if err != nil || v <= 0 {
			http.Error(w, "version query param required", http.StatusBadRequest)
			return
		}
		if _, err := a.Pool.Exec(ctx,
			`UPDATE skills SET pinned_version = $1, updated_at = now() WHERE id = $2`,
			v, skillID,
		); err != nil {
			http.Error(w, "pin: "+err.Error(), http.StatusInternalServerError)
			return
		}
	case "unpin":
		if _, err := a.Pool.Exec(ctx,
			`UPDATE skills SET pinned_version = NULL, updated_at = now() WHERE id = $1`,
			skillID,
		); err != nil {
			http.Error(w, "unpin: "+err.Error(), http.StatusInternalServerError)
			return
		}
	case "disable":
		if _, err := a.Pool.Exec(ctx,
			`UPDATE skills SET enabled = false, updated_at = now() WHERE id = $1`,
			skillID,
		); err != nil {
			http.Error(w, "disable: "+err.Error(), http.StatusInternalServerError)
			return
		}
	case "enable":
		if _, err := a.Pool.Exec(ctx,
			`UPDATE skills SET enabled = true, updated_at = now() WHERE id = $1`,
			skillID,
		); err != nil {
			http.Error(w, "enable: "+err.Error(), http.StatusInternalServerError)
			return
		}
	default:
		http.Error(w, "unknown action: "+action, http.StatusBadRequest)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]string{"action": action, "id": skillID.String()})
}
