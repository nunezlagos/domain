package seeds

import (
	"context"
	"encoding/json"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
)

// SkillCatalog define skills built-in que toda org nueva recibe.
// Pattern: skill_catalog NO es global (skills tienen org_id NOT NULL),
// se materializa per-org via SeedSkillsForOrg invocado al crear org
// (issue-21.1 org-management) o vía CLI `domain seed --org <id>`.
//
// Cada skill se marca seed_managed=true; updates futuros del catalog
// solo afectan filas con is_user_modified=false (no sobrescribe customs).
type SkillCatalogEntry struct {
	Slug           string
	Name           string
	Description    string
	SkillType      string // prompt | code | api | mcp_tool
	Content        string
	InputSchema    map[string]any
	OutputSchema   map[string]any
	TimeoutSeconds int
	Idempotent     bool
	HasSideEffects bool
	Tags           []string
}

// SkillCatalog es el catálogo built-in.
func SkillCatalog() []SkillCatalogEntry {
	return []SkillCatalogEntry{
		{
			Slug:        "intake-classify",
			Name:        "Intake Classify",
			Description: "Clasifica un raw text en {type, severity, confidence} para el intake pipeline (issue-04.8).",
			SkillType:   "prompt",
			Content: `Sos un classifier de requerimientos.
Input: texto libre del cliente.
Output JSON estricto:
{"type":"feat|fix|hotfix|chore|refactor|docs","severity":"low|medium|high|critical","confidence":0..1,"reasoning":"breve"}`,
			InputSchema: map[string]any{
				"type":     "object",
				"required": []string{"raw_text"},
				"properties": map[string]any{
					"raw_text": map[string]string{"type": "string"},
				},
			},
			OutputSchema: map[string]any{
				"type":     "object",
				"required": []string{"type", "severity", "confidence"},
				"properties": map[string]any{
					"type":       map[string]any{"type": "string", "enum": []string{"feat", "fix", "hotfix", "chore", "refactor", "docs"}},
					"severity":   map[string]any{"type": "string", "enum": []string{"low", "medium", "high", "critical"}},
					"confidence": map[string]any{"type": "number"},
					"reasoning":  map[string]string{"type": "string"},
				},
			},
			TimeoutSeconds: 30,
			Idempotent:     true,
			Tags:           []string{"intake", "classify", "platform"},
		},
		{
			Slug:        "intake-structure",
			Name:        "Intake Structure",
			Description: "Genera {title, description, req_slug, hu_draft} desde raw text + classification.",
			SkillType:   "prompt",
			Content: `Sos un structurer de requerimientos. Recibís raw text + classification y producís:
{"title":"...","description":"...","req_slug":"REQ-XX-…","hu_draft":{"slug":"...","goal":"..."}}`,
			TimeoutSeconds: 45,
			Idempotent:     true,
			Tags:           []string{"intake", "structure", "platform"},
		},
		{
			Slug:        "code-search",
			Name:        "Code Search",
			Description: "Búsqueda semántica + fts sobre repo del proyecto. Retorna files + line ranges.",
			SkillType:   "code",
			TimeoutSeconds: 15,
			Idempotent:     true,
			Tags:           []string{"code", "search", "platform"},
		},
		{
			Slug:        "file-read",
			Name:        "File Read",
			Description: "Lee contenido de un archivo del repo (sandboxed; readonly).",
			SkillType:   "code",
			TimeoutSeconds: 10,
			Idempotent:     true,
			Tags:           []string{"code", "file", "platform"},
		},
		{
			Slug:        "web-fetch",
			Name:        "Web Fetch",
			Description: "Descarga URL → markdown text. SSRF guarded.",
			SkillType:   "api",
			TimeoutSeconds: 30,
			Idempotent:     true,
			Tags:           []string{"web", "fetch", "platform"},
		},
		{
			Slug:        "summarize",
			Name:        "Summarize",
			Description: "Resume texto largo en N frases con bullets opcionales.",
			SkillType:   "prompt",
			Content: `Resumí el siguiente texto en {{ .max_sentences }} oraciones.
Si {{ .bullets }} = true, formato bullet points.
Texto:
{{ .text }}`,
			TimeoutSeconds: 30,
			Idempotent:     true,
			Tags:           []string{"text", "summary", "platform"},
		},
		{
			Slug:        "extract-entities",
			Name:        "Extract Entities",
			Description: "Extrae entidades nombradas (persona, org, fecha, dinero) de texto.",
			SkillType:   "prompt",
			TimeoutSeconds: 30,
			Idempotent:     true,
			Tags:           []string{"nlp", "ner", "platform"},
		},
	}
}

// SeedSkillsForOrg materializa el catálogo en una org específica.
// Idempotente: ON CONFLICT (org, slug) UPDATE solo si is_user_modified=false.
func SeedSkillsForOrg(ctx context.Context, pool *pgxpool.Pool, orgID uuid.UUID, version int) (Report, error) {
	var rep Report
	for _, e := range SkillCatalog() {
		input, _ := json.Marshal(e.InputSchema)
		output, _ := json.Marshal(e.OutputSchema)
		tags := e.Tags
		if tags == nil {
			tags = []string{}
		}
		var contentPtr *string
		if e.Content != "" {
			contentPtr = &e.Content
		}

		tag, err := pool.Exec(ctx, `
			INSERT INTO skills (organization_id, slug, name, description, skill_type,
			                    content, input_schema, output_schema, timeout_seconds,
			                    idempotent, has_side_effects, tags,
			                    seed_managed, seed_version)
			VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12, TRUE, $13)
			ON CONFLICT (organization_id, slug) DO UPDATE
			SET name             = EXCLUDED.name,
			    description      = EXCLUDED.description,
			    skill_type       = EXCLUDED.skill_type,
			    content          = EXCLUDED.content,
			    input_schema     = EXCLUDED.input_schema,
			    output_schema    = EXCLUDED.output_schema,
			    timeout_seconds  = EXCLUDED.timeout_seconds,
			    idempotent       = EXCLUDED.idempotent,
			    has_side_effects = EXCLUDED.has_side_effects,
			    tags             = EXCLUDED.tags,
			    seed_version     = EXCLUDED.seed_version
			WHERE skills.is_user_modified = FALSE`,
			orgID, e.Slug, e.Name, e.Description, e.SkillType,
			contentPtr, input, output, e.TimeoutSeconds,
			e.Idempotent, e.HasSideEffects, tags, version)
		if err != nil {
			return rep, err
		}
		if tag.RowsAffected() == 1 {
			rep.Created++
		} else if tag.RowsAffected() == 0 {
			rep.Preserved++ // user-modified, no se sobrescribe
		} else {
			rep.Updated++
		}
	}
	return rep, nil
}
