// Package skill — issue-05.1 + issue-05.2 skill registry y management.
//
// Skills son capacidades reutilizables (prompt | code | api | mcp_tool) que
// los agentes invocan. Cada skill tiene:
//   - slug único por org
//   - skill_type (CHECK constraint en BD)
//   - input_schema + output_schema (JSON Schema 2020-12, validado al Create)
//   - content (template / código / config de la skill)
//   - embedding generado de (name + description) para semantic search
//
// La ejecución (run) vive en issue-05.5, separada.
package skill

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"regexp"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgerrcode"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/santhosh-tekuri/jsonschema/v6"

	"nunezlagos/domain/internal/audit"
	"nunezlagos/domain/internal/llm"
)

const (
	TypePrompt  = "prompt"
	TypeCode    = "code"
	TypeAPI     = "api"
	TypeMCPTool = "mcp_tool"
)

var allowedTypes = map[string]bool{
	TypePrompt: true, TypeCode: true, TypeAPI: true, TypeMCPTool: true,
}

var (
	ErrSlugInvalid       = errors.New("slug must be lowercase ascii, digits, dashes (2-100 chars)")
	ErrSlugTaken         = errors.New("slug already taken in this organization")
	ErrInvalidType       = errors.New("invalid skill_type (allowed: prompt, code, api, mcp_tool)")
	ErrInvalidSchema     = errors.New("invalid JSON Schema")
	ErrContentRequired   = errors.New("content required")
	ErrNameRequired      = errors.New("name required")
	ErrNotFound          = errors.New("skill not found")
	ErrHasDependencies   = errors.New("skill has active dependencies and cannot be deleted")
)

var reSlug = regexp.MustCompile(`^[a-z][a-z0-9-]{0,98}[a-z0-9]$|^[a-z]$`)

type Skill struct {
	ID              uuid.UUID
	OrganizationID  uuid.UUID
	Slug            string
	Name            string
	Description     string
	SkillType       string
	Content         string
	InputSchema     map[string]any
	OutputSchema    map[string]any
	TimeoutSeconds  int
	Idempotent      bool
	HasSideEffects  bool
	DependsOn       []string
	Tags            []string
	SeedManaged     bool
	SeedVersion     *int
	IsUserModified  bool
	CreatedAt       time.Time
	UpdatedAt       time.Time
}

type CreateInput struct {
	OrganizationID  uuid.UUID
	Slug            string
	Name            string
	Description     string
	SkillType       string
	Content         string
	InputSchema     map[string]any
	OutputSchema    map[string]any
	TimeoutSeconds  int
	Idempotent      bool
	HasSideEffects  bool
	DependsOn       []string
	Tags            []string
	ActorID         uuid.UUID
}

type UpdateInput struct {
	Name           *string
	Description    *string
	Content        *string
	InputSchema    map[string]any
	OutputSchema   map[string]any
	TimeoutSeconds *int
	Idempotent     *bool
	HasSideEffects *bool
	DependsOn      []string
	Tags           []string
	ActorID        uuid.UUID
}

type SearchResult struct {
	Skill
	Score      float64
	BM25Rank   int
	VectorRank int
}

type Service struct {
	Pool     *pgxpool.Pool
	Audit    audit.Recorder
	Embedder llm.Embedder
}

// validateSchema usa santhosh-tekuri/jsonschema (cargado por mcp-go) para
// confirmar que el shape es un JSON Schema válido.
func validateSchema(schema map[string]any) error {
	if len(schema) == 0 {
		return nil
	}
	c := jsonschema.NewCompiler()
	if err := c.AddResource("inline://schema.json", schema); err != nil {
		return fmt.Errorf("%w: %v", ErrInvalidSchema, err)
	}
	if _, err := c.Compile("inline://schema.json"); err != nil {
		return fmt.Errorf("%w: %v", ErrInvalidSchema, err)
	}
	return nil
}

// Create persiste skill + auto-embed sobre (name + " " + description).
func (s *Service) Create(ctx context.Context, in CreateInput) (*Skill, error) {
	if !reSlug.MatchString(in.Slug) {
		return nil, ErrSlugInvalid
	}
	if strings.TrimSpace(in.Name) == "" {
		return nil, ErrNameRequired
	}
	if !allowedTypes[in.SkillType] {
		return nil, ErrInvalidType
	}
	if strings.TrimSpace(in.Content) == "" {
		return nil, ErrContentRequired
	}
	if in.InputSchema == nil {
		in.InputSchema = map[string]any{}
	}
	if in.OutputSchema == nil {
		in.OutputSchema = map[string]any{}
	}
	if err := validateSchema(in.InputSchema); err != nil {
		return nil, err
	}
	if err := validateSchema(in.OutputSchema); err != nil {
		return nil, err
	}
	if in.TimeoutSeconds == 0 {
		in.TimeoutSeconds = 30
	}
	if in.Tags == nil {
		in.Tags = []string{}
	}
	if in.DependsOn == nil {
		in.DependsOn = []string{}
	}

	// Embedding
	embedText := in.Name + " " + in.Description
	vec, err := s.Embedder.Embed(ctx, embedText)
	if err != nil {
		return nil, fmt.Errorf("embed: %w", err)
	}
	embedLit := vectorLiteral(vec)
	inJSON, _ := json.Marshal(in.InputSchema)
	outJSON, _ := json.Marshal(in.OutputSchema)

	var sk Skill
	err = s.Pool.QueryRow(ctx,
		`INSERT INTO skills
		   (organization_id, slug, name, description, skill_type, content,
		    input_schema, output_schema, timeout_seconds, idempotent,
		    has_side_effects, depends_on, tags, embedding)
		 VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14::vector)
		 RETURNING id, organization_id, slug, name, COALESCE(description,''),
		           skill_type, COALESCE(content,''), input_schema, output_schema,
		           timeout_seconds, idempotent, has_side_effects, depends_on, tags,
		           seed_managed, seed_version, is_user_modified, created_at, updated_at`,
		in.OrganizationID, in.Slug, in.Name, nullStr(in.Description), in.SkillType, in.Content,
		inJSON, outJSON, in.TimeoutSeconds, in.Idempotent,
		in.HasSideEffects, in.DependsOn, in.Tags, embedLit,
	).Scan(&sk.ID, &sk.OrganizationID, &sk.Slug, &sk.Name, &sk.Description,
		&sk.SkillType, &sk.Content, &sk.InputSchema, &sk.OutputSchema,
		&sk.TimeoutSeconds, &sk.Idempotent, &sk.HasSideEffects, &sk.DependsOn, &sk.Tags,
		&sk.SeedManaged, &sk.SeedVersion, &sk.IsUserModified, &sk.CreatedAt, &sk.UpdatedAt)
	if err != nil {
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) {
			if pgErr.Code == pgerrcode.UniqueViolation {
				return nil, ErrSlugTaken
			}
			if pgErr.Code == pgerrcode.CheckViolation && pgErr.ConstraintName == "skills_skill_type_check" {
				return nil, ErrInvalidType
			}
		}
		return nil, fmt.Errorf("insert skill: %w", err)
	}

	if s.Audit != nil {
		audit.RecordOrLog(ctx, s.Audit, audit.Event{
			OrganizationID: &in.OrganizationID,
			ActorID:        &in.ActorID,
			ActorType:      audit.ActorUser,
			Action:         "skill.created",
			EntityType:     "skill",
			EntityID:       &sk.ID,
			NewValues:      map[string]any{"slug": sk.Slug, "type": sk.SkillType},
		})
	}
	return &sk, nil
}

// Update modifica los campos provistos. Re-embed si Name o Description cambian.
// Marca is_user_modified=true si seed_managed (issue-01.7 seeders contract).
func (s *Service) Update(ctx context.Context, id uuid.UUID, in UpdateInput) (*Skill, error) {
	prev, err := s.GetByID(ctx, id)
	if err != nil {
		return nil, err
	}
	name := prev.Name
	if in.Name != nil {
		name = *in.Name
	}
	desc := prev.Description
	if in.Description != nil {
		desc = *in.Description
	}
	content := prev.Content
	if in.Content != nil {
		content = *in.Content
	}
	inSchema := prev.InputSchema
	if in.InputSchema != nil {
		if err := validateSchema(in.InputSchema); err != nil {
			return nil, err
		}
		inSchema = in.InputSchema
	}
	outSchema := prev.OutputSchema
	if in.OutputSchema != nil {
		if err := validateSchema(in.OutputSchema); err != nil {
			return nil, err
		}
		outSchema = in.OutputSchema
	}
	timeout := prev.TimeoutSeconds
	if in.TimeoutSeconds != nil {
		timeout = *in.TimeoutSeconds
	}
	idempotent := prev.Idempotent
	if in.Idempotent != nil {
		idempotent = *in.Idempotent
	}
	hasSE := prev.HasSideEffects
	if in.HasSideEffects != nil {
		hasSE = *in.HasSideEffects
	}
	dependsOn := prev.DependsOn
	if in.DependsOn != nil {
		dependsOn = in.DependsOn
	}
	tags := prev.Tags
	if in.Tags != nil {
		tags = in.Tags
	}
	userMod := prev.IsUserModified || prev.SeedManaged

	// Re-embed solo si name o description cambian
	embedLit := ""
	reembed := (in.Name != nil && *in.Name != prev.Name) ||
		(in.Description != nil && *in.Description != prev.Description)
	if reembed {
		vec, err := s.Embedder.Embed(ctx, name+" "+desc)
		if err != nil {
			return nil, fmt.Errorf("embed: %w", err)
		}
		embedLit = vectorLiteral(vec)
	}

	inJSON, _ := json.Marshal(inSchema)
	outJSON, _ := json.Marshal(outSchema)

	var sk Skill
	var query string
	args := []any{id, name, nullStr(desc), nullStr(content), inJSON, outJSON,
		timeout, idempotent, hasSE, dependsOn, tags, userMod}
	if reembed {
		query = `UPDATE skills
		         SET name = $2, description = $3, content = $4, input_schema = $5,
		             output_schema = $6, timeout_seconds = $7, idempotent = $8,
		             has_side_effects = $9, depends_on = $10, tags = $11,
		             is_user_modified = $12, embedding = $13::vector
		         WHERE id = $1 AND deleted_at IS NULL
		         RETURNING id, organization_id, slug, name, COALESCE(description,''),
		                   skill_type, COALESCE(content,''), input_schema, output_schema,
		                   timeout_seconds, idempotent, has_side_effects, depends_on, tags,
		                   seed_managed, seed_version, is_user_modified, created_at, updated_at`
		args = append(args, embedLit)
	} else {
		query = `UPDATE skills
		         SET name = $2, description = $3, content = $4, input_schema = $5,
		             output_schema = $6, timeout_seconds = $7, idempotent = $8,
		             has_side_effects = $9, depends_on = $10, tags = $11,
		             is_user_modified = $12
		         WHERE id = $1 AND deleted_at IS NULL
		         RETURNING id, organization_id, slug, name, COALESCE(description,''),
		                   skill_type, COALESCE(content,''), input_schema, output_schema,
		                   timeout_seconds, idempotent, has_side_effects, depends_on, tags,
		                   seed_managed, seed_version, is_user_modified, created_at, updated_at`
	}
	err = s.Pool.QueryRow(ctx, query, args...).Scan(
		&sk.ID, &sk.OrganizationID, &sk.Slug, &sk.Name, &sk.Description,
		&sk.SkillType, &sk.Content, &sk.InputSchema, &sk.OutputSchema,
		&sk.TimeoutSeconds, &sk.Idempotent, &sk.HasSideEffects, &sk.DependsOn, &sk.Tags,
		&sk.SeedManaged, &sk.SeedVersion, &sk.IsUserModified, &sk.CreatedAt, &sk.UpdatedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("update skill: %w", err)
	}

	if s.Audit != nil {
		audit.RecordOrLog(ctx, s.Audit, audit.Event{
			OrganizationID: &sk.OrganizationID,
			ActorID:        &in.ActorID,
			ActorType:      audit.ActorUser,
			Action:         "skill.updated",
			EntityType:     "skill",
			EntityID:       &sk.ID,
		})
	}
	return &sk, nil
}

func (s *Service) GetByID(ctx context.Context, id uuid.UUID) (*Skill, error) {
	return s.queryOne(ctx, `WHERE id = $1 AND deleted_at IS NULL`, id)
}

func (s *Service) GetBySlug(ctx context.Context, orgID uuid.UUID, slug string) (*Skill, error) {
	return s.queryOne(ctx,
		`WHERE organization_id = $1 AND slug = $2 AND deleted_at IS NULL AND proposed = false`, orgID, slug)
}

type ListFilter struct {
	SkillType string
	Tag       string
	Limit     int
}

func (s *Service) List(ctx context.Context, orgID uuid.UUID, f ListFilter) ([]Skill, error) {
	if f.Limit <= 0 || f.Limit > 200 {
		f.Limit = 50
	}
	q := `SELECT id, organization_id, slug, name, COALESCE(description,''),
	        skill_type, COALESCE(content,''), input_schema, output_schema,
	        timeout_seconds, idempotent, has_side_effects, depends_on, tags,
	        seed_managed, seed_version, is_user_modified, created_at, updated_at
	      FROM skills WHERE organization_id = $1 AND deleted_at IS NULL AND proposed = false`
	args := []any{orgID}
	if f.SkillType != "" {
		q += fmt.Sprintf(" AND skill_type = $%d", len(args)+1)
		args = append(args, f.SkillType)
	}
	if f.Tag != "" {
		q += fmt.Sprintf(" AND $%d = ANY(tags)", len(args)+1)
		args = append(args, f.Tag)
	}
	q += fmt.Sprintf(" ORDER BY created_at DESC LIMIT $%d", len(args)+1)
	args = append(args, f.Limit)

	rows, err := s.Pool.Query(ctx, q, args...)
	if err != nil {
		return nil, fmt.Errorf("list: %w", err)
	}
	defer rows.Close()
	var out []Skill
	for rows.Next() {
		var sk Skill
		if err := rows.Scan(&sk.ID, &sk.OrganizationID, &sk.Slug, &sk.Name, &sk.Description,
			&sk.SkillType, &sk.Content, &sk.InputSchema, &sk.OutputSchema,
			&sk.TimeoutSeconds, &sk.Idempotent, &sk.HasSideEffects, &sk.DependsOn, &sk.Tags,
			&sk.SeedManaged, &sk.SeedVersion, &sk.IsUserModified, &sk.CreatedAt, &sk.UpdatedAt); err != nil {
			return nil, err
		}
		out = append(out, sk)
	}
	return out, rows.Err()
}

// SearchHybrid sobre description_tsv + embedding con RRF fusion.
func (s *Service) SearchHybrid(ctx context.Context, orgID uuid.UUID, query string, limit int) ([]SearchResult, error) {
	if strings.TrimSpace(query) == "" {
		return nil, nil
	}
	if limit <= 0 || limit > 100 {
		limit = 20
	}
	vec, err := s.Embedder.Embed(ctx, query)
	if err != nil {
		return nil, fmt.Errorf("embed query: %w", err)
	}
	useVector := !llm.IsZero(vec)

	const rrfK = 60
	const candidates = 100

	var rows pgx.Rows
	if useVector {
		rows, err = s.Pool.Query(ctx, `
WITH bm25 AS (
  SELECT id, ROW_NUMBER() OVER (ORDER BY ts_rank(description_tsv, q) DESC) AS r
  FROM skills, plainto_tsquery('spanish', $2) AS q
  WHERE organization_id = $1 AND deleted_at IS NULL AND proposed = false AND description_tsv @@ q
  LIMIT $4
),
vec AS (
  SELECT id, ROW_NUMBER() OVER (ORDER BY embedding <=> $3::vector ASC) AS r
  FROM skills
  WHERE organization_id = $1 AND deleted_at IS NULL AND proposed = false AND embedding IS NOT NULL
  LIMIT $4
),
fused AS (
  SELECT id,
         COALESCE(1.0 / ($5 + bm25.r), 0) + COALESCE(1.0 / ($5 + vec.r), 0) AS score,
         COALESCE(bm25.r, 0) AS bm25_rank,
         COALESCE(vec.r, 0) AS vec_rank
  FROM bm25 FULL OUTER JOIN vec USING (id)
)
SELECT s.id, s.organization_id, s.slug, s.name, COALESCE(s.description,''),
       s.skill_type, COALESCE(s.content,''), s.input_schema, s.output_schema,
       s.timeout_seconds, s.idempotent, s.has_side_effects, s.depends_on, s.tags,
       s.seed_managed, s.seed_version, s.is_user_modified, s.created_at, s.updated_at,
       f.score, f.bm25_rank, f.vec_rank
FROM fused f
JOIN skills s ON s.id = f.id
ORDER BY f.score DESC
LIMIT $6
`, orgID, query, vectorLiteral(vec), candidates, rrfK, limit)
	} else {
		rows, err = s.Pool.Query(ctx, `
SELECT s.id, s.organization_id, s.slug, s.name, COALESCE(s.description,''),
       s.skill_type, COALESCE(s.content,''), s.input_schema, s.output_schema,
       s.timeout_seconds, s.idempotent, s.has_side_effects, s.depends_on, s.tags,
       s.seed_managed, s.seed_version, s.is_user_modified, s.created_at, s.updated_at,
       ts_rank(s.description_tsv, q)::float8 AS score, 0::bigint AS bm25_rank, 0::bigint AS vec_rank
FROM skills s, plainto_tsquery('spanish', $2) AS q
WHERE s.organization_id = $1 AND s.deleted_at IS NULL AND s.proposed = false AND s.description_tsv @@ q
ORDER BY score DESC LIMIT $3
`, orgID, query, limit)
	}
	if err != nil {
		return nil, fmt.Errorf("search: %w", err)
	}
	defer rows.Close()
	var out []SearchResult
	for rows.Next() {
		var r SearchResult
		var bm25Rank, vecRank int64
		if err := rows.Scan(&r.ID, &r.OrganizationID, &r.Slug, &r.Name, &r.Description,
			&r.SkillType, &r.Content, &r.InputSchema, &r.OutputSchema,
			&r.TimeoutSeconds, &r.Idempotent, &r.HasSideEffects, &r.DependsOn, &r.Tags,
			&r.SeedManaged, &r.SeedVersion, &r.IsUserModified, &r.CreatedAt, &r.UpdatedAt,
			&r.Score, &bm25Rank, &vecRank); err != nil {
			return nil, err
		}
		r.BM25Rank = int(bm25Rank)
		r.VectorRank = int(vecRank)
		out = append(out, r)
	}
	return out, rows.Err()
}

// SoftDelete marca deleted_at si no hay dependencias activas.
// Detección de dependencias por depends_on de OTROS skills (no flows aún —
// REQ-09 flow_steps no implementado todavía; verificación pending).
func (s *Service) SoftDelete(ctx context.Context, id, actorID uuid.UUID) error {
	prev, err := s.GetByID(ctx, id)
	if err != nil {
		return err
	}

	// Si otro skill nos referencia en depends_on → conflict
	var depCount int
	err = s.Pool.QueryRow(ctx,
		`SELECT COUNT(*) FROM skills
		 WHERE organization_id = $1 AND deleted_at IS NULL AND $2 = ANY(depends_on)`,
		prev.OrganizationID, prev.Slug,
	).Scan(&depCount)
	if err != nil {
		return fmt.Errorf("check deps: %w", err)
	}
	if depCount > 0 {
		return ErrHasDependencies
	}

	_, err = s.Pool.Exec(ctx,
		`UPDATE skills SET deleted_at = NOW() WHERE id = $1 AND deleted_at IS NULL`, id)
	if err != nil {
		return fmt.Errorf("soft delete: %w", err)
	}
	if s.Audit != nil {
		audit.RecordOrLog(ctx, s.Audit, audit.Event{
			OrganizationID: &prev.OrganizationID,
			ActorID:        &actorID,
			ActorType:      audit.ActorUser,
			Action:         "skill.deleted",
			EntityType:     "skill",
			EntityID:       &id,
		})
	}
	return nil
}

// ValidateInput valida un input contra el InputSchema de la skill.
// Útil antes de llamar Run() en issue-05.5.
func (s *Service) ValidateInput(ctx context.Context, skillID uuid.UUID, input map[string]any) error {
	sk, err := s.GetByID(ctx, skillID)
	if err != nil {
		return err
	}
	if len(sk.InputSchema) == 0 {
		return nil
	}
	c := jsonschema.NewCompiler()
	if err := c.AddResource("inline://skill.json", sk.InputSchema); err != nil {
		return fmt.Errorf("compile schema: %w", err)
	}
	sch, err := c.Compile("inline://skill.json")
	if err != nil {
		return fmt.Errorf("compile: %w", err)
	}
	if err := sch.Validate(input); err != nil {
		return fmt.Errorf("input does not match input_schema: %w", err)
	}
	return nil
}

func (s *Service) queryOne(ctx context.Context, where string, args ...any) (*Skill, error) {
	var sk Skill
	q := `SELECT id, organization_id, slug, name, COALESCE(description,''),
	        skill_type, COALESCE(content,''), input_schema, output_schema,
	        timeout_seconds, idempotent, has_side_effects, depends_on, tags,
	        seed_managed, seed_version, is_user_modified, created_at, updated_at
	      FROM skills ` + where
	err := s.Pool.QueryRow(ctx, q, args...).Scan(
		&sk.ID, &sk.OrganizationID, &sk.Slug, &sk.Name, &sk.Description,
		&sk.SkillType, &sk.Content, &sk.InputSchema, &sk.OutputSchema,
		&sk.TimeoutSeconds, &sk.Idempotent, &sk.HasSideEffects, &sk.DependsOn, &sk.Tags,
		&sk.SeedManaged, &sk.SeedVersion, &sk.IsUserModified, &sk.CreatedAt, &sk.UpdatedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("query: %w", err)
	}
	return &sk, nil
}

func vectorLiteral(v []float32) string {
	if len(v) == 0 {
		return "[]"
	}
	var sb strings.Builder
	sb.WriteByte('[')
	for i, f := range v {
		if i > 0 {
			sb.WriteByte(',')
		}
		fmt.Fprintf(&sb, "%g", f)
	}
	sb.WriteByte(']')
	return sb.String()
}

func nullStr(s string) any {
	if s == "" {
		return nil
	}
	return s
}
