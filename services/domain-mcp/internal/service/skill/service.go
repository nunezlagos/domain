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
	"github.com/pgvector/pgvector-go"
	"github.com/santhosh-tekuri/jsonschema/v6"

	"nunezlagos/domain/internal/audit"
	"nunezlagos/domain/internal/llm"
	"nunezlagos/domain/internal/service/skill/skilldb"
	"nunezlagos/domain/internal/store/txctx"
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

func (s *Service) q(ctx context.Context) *skilldb.Queries {
	if tx := txctx.TxFromContext(ctx); tx != nil {
		return skilldb.New(tx)
	}
	return skilldb.New(s.Pool)
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

func toSkill(id uuid.UUID, slug string, name string, description string, skillType string, content string, inputSchema, outputSchema []byte, timeoutSeconds int32, idempotent, hasSideEffects bool, dependsOn, tags []string, seedManaged bool, seedVersion *int32, isUserModified bool, createdAt, updatedAt time.Time) Skill {
	var inMap, outMap map[string]any
	if len(inputSchema) > 0 {
		_ = json.Unmarshal(inputSchema, &inMap)
	}
	if len(outputSchema) > 0 {
		_ = json.Unmarshal(outputSchema, &outMap)
	}
	var sv *int
	if seedVersion != nil {
		v := int(*seedVersion)
		sv = &v
	}
	return Skill{
		ID: id, Slug: slug, Name: name, Description: description,
		SkillType: skillType, Content: content,
		InputSchema: inMap, OutputSchema: outMap,
		TimeoutSeconds: int(timeoutSeconds),
		Idempotent: idempotent, HasSideEffects: hasSideEffects,
		DependsOn: dependsOn, Tags: tags,
		SeedManaged: seedManaged, SeedVersion: sv,
		IsUserModified: isUserModified,
		CreatedAt: createdAt, UpdatedAt: updatedAt,
	}
}

func toSkillFromCreateRow(r skilldb.SkillCreateRow) Skill {
	return toSkill(r.ID, r.Slug, r.Name, r.Description, r.SkillType, r.Content,
		r.InputSchema, r.OutputSchema, r.TimeoutSeconds, r.Idempotent, r.HasSideEffects,
		r.DependsOn, r.Tags, r.SeedManaged, r.SeedVersion, r.IsUserModified, r.CreatedAt, r.UpdatedAt)
}

func toSearchResultFromWithVector(r skilldb.SkillSearchHybridWithVectorRow) SearchResult {
	return SearchResult{
		Skill: toSkill(r.ID, r.Slug, r.Name, r.Description, r.SkillType, r.Content,
			r.InputSchema, r.OutputSchema, r.TimeoutSeconds, r.Idempotent, r.HasSideEffects,
			r.DependsOn, r.Tags, r.SeedManaged, r.SeedVersion, r.IsUserModified, r.CreatedAt, r.UpdatedAt),
		Score: r.Score, BM25Rank: int(r.Bm25Rank), VectorRank: int(r.VecRank),
	}
}

func toSkillFromGetByIDRow(r skilldb.SkillGetByIDRow) Skill {
	return toSkill(r.ID, r.Slug, r.Name, r.Description, r.SkillType, r.Content,
		r.InputSchema, r.OutputSchema, r.TimeoutSeconds, r.Idempotent, r.HasSideEffects,
		r.DependsOn, r.Tags, r.SeedManaged, r.SeedVersion, r.IsUserModified, r.CreatedAt, r.UpdatedAt)
}

func toSkillFromGetBySlugRow(r skilldb.SkillGetBySlugRow) Skill {
	return toSkill(r.ID, r.Slug, r.Name, r.Description, r.SkillType, r.Content,
		r.InputSchema, r.OutputSchema, r.TimeoutSeconds, r.Idempotent, r.HasSideEffects,
		r.DependsOn, r.Tags, r.SeedManaged, r.SeedVersion, r.IsUserModified, r.CreatedAt, r.UpdatedAt)
}

func toSkillFromListRow(r skilldb.SkillListRow) Skill {
	return toSkill(r.ID, r.Slug, r.Name, r.Description, r.SkillType, r.Content,
		r.InputSchema, r.OutputSchema, r.TimeoutSeconds, r.Idempotent, r.HasSideEffects,
		r.DependsOn, r.Tags, r.SeedManaged, r.SeedVersion, r.IsUserModified, r.CreatedAt, r.UpdatedAt)
}

func toSkillFromUpdateRow(r skilldb.SkillUpdateRow) Skill {
	return toSkill(r.ID, r.Slug, r.Name, r.Description, r.SkillType, r.Content,
		r.InputSchema, r.OutputSchema, r.TimeoutSeconds, r.Idempotent, r.HasSideEffects,
		r.DependsOn, r.Tags, r.SeedManaged, r.SeedVersion, r.IsUserModified, r.CreatedAt, r.UpdatedAt)
}

func toSkillFromUpdateWithEmbeddingRow(r skilldb.SkillUpdateWithEmbeddingRow) Skill {
	return toSkill(r.ID, r.Slug, r.Name, r.Description, r.SkillType, r.Content,
		r.InputSchema, r.OutputSchema, r.TimeoutSeconds, r.Idempotent, r.HasSideEffects,
		r.DependsOn, r.Tags, r.SeedManaged, r.SeedVersion, r.IsUserModified, r.CreatedAt, r.UpdatedAt)
}

func toSearchResultFromBM25(r skilldb.SkillSearchHybridBM25OnlyRow) SearchResult {
	return SearchResult{
		Skill: toSkill(r.ID, r.Slug, r.Name, r.Description, r.SkillType, r.Content,
			r.InputSchema, r.OutputSchema, r.TimeoutSeconds, r.Idempotent, r.HasSideEffects,
			r.DependsOn, r.Tags, r.SeedManaged, r.SeedVersion, r.IsUserModified, r.CreatedAt, r.UpdatedAt),
		Score: r.Score, BM25Rank: int(r.Bm25Rank), VectorRank: int(r.VecRank),
	}
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


	embedText := in.Name + " " + in.Description
	vec, err := s.Embedder.Embed(ctx, embedText)
	if err != nil {
		return nil, fmt.Errorf("embed: %w", err)
	}
	inJSON, _ := json.Marshal(in.InputSchema)
	outJSON, _ := json.Marshal(in.OutputSchema)

	emb := pgvector.NewVector(vec)
	row, err := s.q(ctx).SkillCreate(ctx, skilldb.SkillCreateParams{
		Slug:           in.Slug,
		Name:           in.Name,
		Description:    nullStr(in.Description),
		SkillType:      in.SkillType,
		Content:        nullStr(in.Content),
		InputSchema:    inJSON,
		OutputSchema:   outJSON,
		TimeoutSeconds: int32(in.TimeoutSeconds),
		Idempotent:     in.Idempotent,
		HasSideEffects: in.HasSideEffects,
		DependsOn:      in.DependsOn,
		Tags:           in.Tags,
		Embedding:      &emb,
	})
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
	sk := toSkillFromCreateRow(row)

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


	reembed := (in.Name != nil && *in.Name != prev.Name) ||
		(in.Description != nil && *in.Description != prev.Description)
	var embedVec *pgvector.Vector
	if reembed {
		vec, err := s.Embedder.Embed(ctx, name+" "+desc)
		if err != nil {
			return nil, fmt.Errorf("embed: %w", err)
		}
		v := pgvector.NewVector(vec)
		embedVec = &v
	}

	inJSON, _ := json.Marshal(inSchema)
	outJSON, _ := json.Marshal(outSchema)

	var sk Skill
	if reembed {
		row, err := s.q(ctx).SkillUpdateWithEmbedding(ctx, skilldb.SkillUpdateWithEmbeddingParams{
			Name:           name,
			Description:    nullStr(desc),
			Content:        nullStr(content),
			InputSchema:    inJSON,
			OutputSchema:   outJSON,
			TimeoutSeconds: int32(timeout),
			Idempotent:     idempotent,
			HasSideEffects: hasSE,
			DependsOn:      dependsOn,
			Tags:           tags,
			IsUserModified: userMod,
			Embedding:      embedVec,
			ID:             id,
		})
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrNotFound
		}
		if err != nil {
			return nil, fmt.Errorf("update skill: %w", err)
		}
		sk = toSkillFromUpdateWithEmbeddingRow(row)
	} else {
		row, err := s.q(ctx).SkillUpdate(ctx, skilldb.SkillUpdateParams{
			Name:           name,
			Description:    nullStr(desc),
			Content:        nullStr(content),
			InputSchema:    inJSON,
			OutputSchema:   outJSON,
			TimeoutSeconds: int32(timeout),
			Idempotent:     idempotent,
			HasSideEffects: hasSE,
			DependsOn:      dependsOn,
			Tags:           tags,
			IsUserModified: userMod,
			ID:             id,
		})
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrNotFound
		}
		if err != nil {
			return nil, fmt.Errorf("update skill: %w", err)
		}
		sk = toSkillFromUpdateRow(row)
	}

	if s.Audit != nil {
		audit.RecordOrLog(ctx, s.Audit, audit.Event{
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
	row, err := s.q(ctx).SkillGetByID(ctx, id)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("query: %w", err)
	}
	sk := toSkillFromGetByIDRow(row)
	return &sk, nil
}

func (s *Service) GetBySlug(ctx context.Context, orgID uuid.UUID, slug string) (*Skill, error) {
	row, err := s.q(ctx).SkillGetBySlug(ctx, slug)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("query: %w", err)
	}
	sk := toSkillFromGetBySlugRow(row)
	return &sk, nil
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
	var skillType, tag *string
	if f.SkillType != "" {
		skillType = &f.SkillType
	}
	if f.Tag != "" {
		tag = &f.Tag
	}
	rows, err := s.q(ctx).SkillList(ctx, skilldb.SkillListParams{
		SkillType:   skillType,
		Tag:         tag,
		ResultLimit: int32(f.Limit),
	})
	if err != nil {
		return nil, fmt.Errorf("list: %w", err)
	}
	out := make([]Skill, len(rows))
	for i, r := range rows {
		out[i] = toSkillFromListRow(r)
	}
	return out, nil
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

	if useVector {
		qvec := pgvector.NewVector(vec)
		rows, err := s.q(ctx).SkillSearchHybridWithVector(ctx, skilldb.SkillSearchHybridWithVectorParams{
			ResultLimit: int32(limit),
			QueryText:   query,
			Candidates:  int32(candidates),
			QueryVec:    &qvec,
			RrfK:        int32(rrfK),
		})
		if err != nil {
			return nil, fmt.Errorf("search: %w", err)
		}
		out := make([]SearchResult, len(rows))
		for i, r := range rows {
			out[i] = toSearchResultFromWithVector(r)
		}
		return out, nil
	}
	rows, err := s.q(ctx).SkillSearchHybridBM25Only(ctx, skilldb.SkillSearchHybridBM25OnlyParams{
		QueryText:   query,
		ResultLimit: int32(limit),
	})
	if err != nil {
		return nil, fmt.Errorf("search: %w", err)
	}
	out := make([]SearchResult, len(rows))
	for i, r := range rows {
		out[i] = toSearchResultFromBM25(r)
	}
	return out, nil
}

// ApplicableSkillIDs devuelve el set de skill IDs que APLICAN a un proyecto
// (modelo hibrido: auto + excluibles). Las globales (project_id IS NULL) aplican
// automaticamente a TODOS los proyectos; las del proyecto (project_id = P)
// tambien. project_skills se usa SOLO para EXCLUIR (fila con is_enabled = FALSE).
// El caller filtra los resultados de SearchHybrid contra este set. Si projectID
// es Nil devuelve nil (sin scope → el caller no filtra).
func (s *Service) ApplicableSkillIDs(ctx context.Context, projectID uuid.UUID) (map[uuid.UUID]bool, error) {
	if projectID == uuid.Nil {
		return nil, nil
	}
	rows, err := s.q(ctx).SkillApplicableIDs(ctx, &projectID)
	if err != nil {
		return nil, fmt.Errorf("applicable skills: %w", err)
	}
	out := make(map[uuid.UUID]bool, len(rows))
	for _, id := range rows {
		out[id] = true
	}
	return out, nil
}

// SoftDelete marca deleted_at si no hay dependencias activas.
// Detección de dependencias por depends_on de OTROS skills (no flows aún —
// REQ-09 flow_steps no implementado todavía; verificación pending).
func (s *Service) SoftDelete(ctx context.Context, id, actorID uuid.UUID) error {
	prev, err := s.GetByID(ctx, id)
	if err != nil {
		return err
	}

	depCount, err := s.q(ctx).SkillSoftDeleteCountDeps(ctx, prev.Slug)
	if err != nil {
		return fmt.Errorf("check deps: %w", err)
	}
	if depCount > 0 {
		return ErrHasDependencies
	}

	if err := s.q(ctx).SkillSoftDelete(ctx, id); err != nil {
		return fmt.Errorf("soft delete: %w", err)
	}
	if s.Audit != nil {
		audit.RecordOrLog(ctx, s.Audit, audit.Event{
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

func nullStr(s string) *string {
	if s == "" {
		return nil
	}
	return &s
}
