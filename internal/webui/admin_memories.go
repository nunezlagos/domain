// issue-16.5 web-admin-memories — admin UI CRUD para knowledge_docs +
// observations (read-only listing + soft-delete admin action).
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

// MemoriesAdmin maneja /admin/memories*.
type MemoriesAdmin struct {
	Pool      *pgxpool.Pool
	AuthCheck func(*http.Request) bool
}

func (a *MemoriesAdmin) Register(mux *http.ServeMux) {
	mux.HandleFunc("/admin/memories", a.page)
	mux.HandleFunc("/admin/api/memories/observations", a.apiObservations)
	mux.HandleFunc("/admin/api/memories/knowledge", a.apiKnowledge)
	mux.HandleFunc("/admin/api/memories/", a.apiAction)
}

func (a *MemoriesAdmin) checkAuth(r *http.Request) bool {
	if a.AuthCheck == nil {
		return true
	}
	return a.AuthCheck(r)
}

func (a *MemoriesAdmin) page(w http.ResponseWriter, r *http.Request) {
	if !a.checkAuth(r) {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}
	data, err := assets.ReadFile("assets/memories.html")
	if err != nil {
		http.NotFound(w, r)
		return
	}
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	_, _ = w.Write(data)
}

type ObservationRow struct {
	ID              uuid.UUID `json:"id"`
	ProjectID       uuid.UUID `json:"project_id"`
	ObservationType string    `json:"observation_type"`
	ContentPreview  string    `json:"content_preview"`
	CreatedAt       time.Time `json:"created_at"`
}

func (a *MemoriesAdmin) apiObservations(w http.ResponseWriter, r *http.Request) {
	if !a.checkAuth(r) {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}
	limit, _ := strconv.Atoi(r.URL.Query().Get("limit"))
	if limit <= 0 || limit > 500 {
		limit = 100
	}
	q := r.URL.Query().Get("q")

	ctx, cancel := context.WithTimeout(r.Context(), 5*time.Second)
	defer cancel()

	sqlText := `SELECT id, project_id, observation_type, LEFT(content, 200), created_at
	            FROM observations WHERE deleted_at IS NULL`
	args := []any{}
	if q != "" {
		sqlText += ` AND content ILIKE $1`
		args = append(args, "%"+q+"%")
	}
	sqlText += fmt.Sprintf(` ORDER BY created_at DESC LIMIT $%d`, len(args)+1)
	args = append(args, limit)

	rows, err := a.Pool.Query(ctx, sqlText, args...)
	if err != nil {
		http.Error(w, "query: "+err.Error(), http.StatusInternalServerError)
		return
	}
	defer rows.Close()

	out := make([]ObservationRow, 0, limit)
	for rows.Next() {
		var o ObservationRow
		if err := rows.Scan(&o.ID, &o.ProjectID, &o.ObservationType,
			&o.ContentPreview, &o.CreatedAt); err != nil {
			continue
		}
		out = append(out, o)
	}
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(out)
}

type KnowledgeRow struct {
	ID         uuid.UUID `json:"id"`
	ProjectID  uuid.UUID `json:"project_id"`
	Title      string    `json:"title"`
	BodyPreview string   `json:"body_preview"`
	CreatedAt  time.Time `json:"created_at"`
}

func (a *MemoriesAdmin) apiKnowledge(w http.ResponseWriter, r *http.Request) {
	if !a.checkAuth(r) {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}
	limit, _ := strconv.Atoi(r.URL.Query().Get("limit"))
	if limit <= 0 || limit > 500 {
		limit = 100
	}
	q := r.URL.Query().Get("q")

	ctx, cancel := context.WithTimeout(r.Context(), 5*time.Second)
	defer cancel()

	sqlText := `SELECT id, project_id, title, LEFT(body, 200), created_at
	            FROM knowledge_docs WHERE deleted_at IS NULL`
	args := []any{}
	if q != "" {
		sqlText += ` AND (title ILIKE $1 OR body ILIKE $1)`
		args = append(args, "%"+q+"%")
	}
	sqlText += fmt.Sprintf(` ORDER BY created_at DESC LIMIT $%d`, len(args)+1)
	args = append(args, limit)

	rows, err := a.Pool.Query(ctx, sqlText, args...)
	if err != nil {
		http.Error(w, "query: "+err.Error(), http.StatusInternalServerError)
		return
	}
	defer rows.Close()

	out := make([]KnowledgeRow, 0, limit)
	for rows.Next() {
		var k KnowledgeRow
		if err := rows.Scan(&k.ID, &k.ProjectID, &k.Title, &k.BodyPreview, &k.CreatedAt); err != nil {
			continue
		}
		out = append(out, k)
	}
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(out)
}

// apiAction soporta soft-delete sobre observations|knowledge.
// POST /admin/api/memories/{kind}/{id}/delete
func (a *MemoriesAdmin) apiAction(w http.ResponseWriter, r *http.Request) {
	if !a.checkAuth(r) {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}
	if r.Method != http.MethodPost {
		http.Error(w, "POST required", http.StatusMethodNotAllowed)
		return
	}
	path := r.URL.Path[len("/admin/api/memories/"):]
	// {kind}/{id}/{action}
	parts := splitPath(path)
	if len(parts) < 3 {
		http.Error(w, "invalid path", http.StatusBadRequest)
		return
	}
	kind, idStr, action := parts[0], parts[1], parts[2]
	id, err := uuid.Parse(idStr)
	if err != nil {
		http.Error(w, "invalid uuid", http.StatusBadRequest)
		return
	}

	ctx, cancel := context.WithTimeout(r.Context(), 5*time.Second)
	defer cancel()

	if action != "delete" {
		http.Error(w, "unknown action: "+action, http.StatusBadRequest)
		return
	}
	var table string
	switch kind {
	case "observations":
		table = "observations"
	case "knowledge":
		table = "knowledge_docs"
	default:
		http.Error(w, "unknown kind: "+kind, http.StatusBadRequest)
		return
	}
	if _, err := a.Pool.Exec(ctx,
		fmt.Sprintf(`UPDATE %s SET deleted_at = now() WHERE id = $1`, table),
		id,
	); err != nil {
		http.Error(w, "delete: "+err.Error(), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]string{
		"kind": kind, "id": id.String(), "action": "soft_deleted",
	})
}

func splitPath(p string) []string {
	out := []string{}
	cur := ""
	for i := 0; i < len(p); i++ {
		if p[i] == '/' {
			if cur != "" {
				out = append(out, cur)
				cur = ""
			}
			continue
		}
		cur += string(p[i])
	}
	if cur != "" {
		out = append(out, cur)
	}
	return out
}
