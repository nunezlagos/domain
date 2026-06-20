package domain

import "time"

// Tipos del API Domain. JSON tags reflejan snake_case del wire format.
// UUIDs van como string para que el módulo no requiera dependencias externas.
// issue-21.6: Organization + organization_id removidos (single-org).

type Member struct {
	UserID    string    `json:"user_id"`
	Email     string    `json:"email"`
	Role      string    `json:"role"`
	JoinedAt  time.Time `json:"joined_at"`
}

type Project struct {
	ID          string    `json:"id"`
	Name        string    `json:"name"`
	Slug        string    `json:"slug"`
	Description string    `json:"description,omitempty"`
	CreatedAt   time.Time `json:"created_at"`
}

type Observation struct {
	ID              string         `json:"id"`
	ProjectID       string         `json:"project_id"`
	Content         string         `json:"content"`
	ObservationType string         `json:"observation_type,omitempty"`
	Tags            []string       `json:"tags,omitempty"`
	Metadata        map[string]any `json:"metadata,omitempty"`
	CreatedAt       time.Time      `json:"created_at"`
}

type Session struct {
	ID        string     `json:"id"`
	Title     string     `json:"title"`
	Summary   string     `json:"summary,omitempty"`
	StartedAt time.Time  `json:"started_at"`
	EndedAt   *time.Time `json:"ended_at,omitempty"`
}

type SearchResult struct {
	EntityType string    `json:"entity_type"`
	ID         string    `json:"id"`
	Title      string    `json:"title,omitempty"`
	Snippet    string    `json:"snippet,omitempty"`
	Score      float64   `json:"score,omitempty"`
	ProjectID  *string   `json:"project_id,omitempty"`
	Tags       []string  `json:"tags,omitempty"`
	CreatedAt  time.Time `json:"created_at"`
}

type Skill struct {
	ID          string         `json:"id"`
	Slug        string         `json:"slug"`
	Name        string         `json:"name"`
	Type        string         `json:"type,omitempty"`
	Description string         `json:"description,omitempty"`
	Tags        []string       `json:"tags,omitempty"`
	Spec        map[string]any `json:"spec,omitempty"`
	CreatedAt   time.Time      `json:"created_at"`
}

type Agent struct {
	ID          string         `json:"id"`
	Slug        string         `json:"slug"`
	Name        string         `json:"name"`
	Description string         `json:"description,omitempty"`
	Config      map[string]any `json:"config,omitempty"`
	CreatedAt   time.Time      `json:"created_at"`
}

type Flow struct {
	ID          string         `json:"id"`
	Slug        string         `json:"slug"`
	Name        string         `json:"name"`
	Description string         `json:"description,omitempty"`
	Spec        map[string]any `json:"spec,omitempty"`
	CreatedAt   time.Time      `json:"created_at"`
}

type Knowledge struct {
	ID         string    `json:"id"`
	ProjectID  string    `json:"project_id"`
	Title      string    `json:"title"`
	Body       string    `json:"body,omitempty"`
	Source     string    `json:"source,omitempty"`
	Tags       []string  `json:"tags,omitempty"`
	CreatedAt  time.Time `json:"created_at"`
}

type KnowledgeSaveResult struct {
	Document     Knowledge `json:"document"`
	ChunksCount  int       `json:"chunks_count"`
}

type RunResult struct {
	RunID        string     `json:"run_id"`
	Status       string     `json:"status"`
	Output       string     `json:"output,omitempty"`
	Error        string     `json:"error,omitempty"`
	TokensInput  int        `json:"tokens_input,omitempty"`
	TokensOutput int        `json:"tokens_output,omitempty"`
	Iterations   int        `json:"iterations,omitempty"`
	StartedAt    *time.Time `json:"started_at,omitempty"`
	FinishedAt   *time.Time `json:"finished_at,omitempty"`
}

// Pagination es el wrapper estándar de listas paginadas.
type Pagination struct {
	NextCursor string `json:"next_cursor,omitempty"`
	HasMore    bool   `json:"has_more"`
	Limit      int    `json:"limit,omitempty"`
}
