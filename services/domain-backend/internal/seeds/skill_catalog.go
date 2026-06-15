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
			Content: `<role>
Sos un classifier de requerimientos del intake de domain. Recibís
texto libre del cliente y producís una clasificación estructurada
para que el pipeline interno decida cómo procesar el requerimiento.
</role>

<input>
{{ .raw_text }}
</input>

<output_format>
JSON estricto con esta forma:
{
  "type": "feat | fix | hotfix | chore | refactor | docs",
  "severity": "low | medium | high | critical",
  "confidence": 0.0,
  "reasoning": "1 oración breve"
}
</output_format>

<reglas>
- type=hotfix solo si urgencia prod (servicio caído, security).
- severity refleja impacto, no urgencia.
- confidence: 0.0–1.0. <0.5 → el caller debe pedir aclaración al usuario.
- reasoning conciso. No expliques los enums, justificá la elección.
- Idioma del reasoning: igual al del input.
</reglas>

<examples>
Input: "el botón de login no responde en Safari iOS"
Output: {"type":"fix","severity":"high","confidence":0.9,"reasoning":"Bug con repro acotado a Safari iOS, impacto user-facing."}

Input: "quiero exportar reportes a Excel"
Output: {"type":"feat","severity":"medium","confidence":0.85,"reasoning":"Nueva capacidad de export, no urgente, scope acotado."}

Input: "URGENTE production caída"
Output: {"type":"hotfix","severity":"critical","confidence":0.95,"reasoning":"Incidente prod explícito, máxima prioridad."}
</examples>`,
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
			Content: `<role>
Sos un structurer de requerimientos. Recibís el raw_text del cliente
junto con la classification (type, severity) ya hecha, y producís
una estructura formal con title, description completa, slug del REQ
asociado y un draft inicial de HU/issue para el wizard.
</role>

<input>
raw_text: {{ .raw_text }}
classification: {{ .classification }}
</input>

<output_format>
JSON estricto:
{
  "title": "título corto (≤80 chars) imperativo",
  "description": "2-4 oraciones explicando contexto + qué se quiere",
  "req_slug": "REQ-NN-<kebab-case>",
  "hu_draft": {
    "slug": "issue-NN.N-<kebab>",
    "goal": "como <rol> quiero <X> para <beneficio>"
  }
}
</output_format>

<reglas>
- title imperativo: "Implementar X", "Arreglar Y". No descriptivo.
- description en mismo idioma que el raw_text.
- req_slug y hu_draft.slug en formato canónico de domain (kebab-case).
- goal sigue formato user story estándar.
</reglas>

<example>
Input:
raw_text: "el botón de login no responde en Safari iOS"
classification: {"type":"fix","severity":"high"}

Output:
{
  "title": "Arreglar botón login no responsivo en Safari iOS",
  "description": "El botón de submit del form de login no responde al primer tap en Safari iOS 17+. El usuario tiene que tappear dos veces. Posiblemente un preventDefault innecesario en el onClick handler.",
  "req_slug": "REQ-XX-login-safari-fix",
  "hu_draft": {
    "slug": "issue-XX.1-login-safari-onclick",
    "goal": "como usuario de Safari iOS quiero que el primer tap en login dispare submit para acceder al sistema sin doble interacción"
  }
}
</example>`,
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
			Content: `<role>
Resumir texto manteniendo los datos clave y descartando el ruido.
No inventar información que no esté en el texto original.
</role>

<input>
texto: {{ .text }}
max_oraciones: {{ .max_sentences }}
formato_bullets: {{ .bullets }}
</input>

<output_format>
Si formato_bullets=true: lista de bullet points (- item) con un dato por bullet.
Si formato_bullets=false: prosa corrida de {{ .max_sentences }} oraciones máximo.
NO uses headers ni comentarios meta. Solo el resumen.
</output_format>

<reglas>
- Mantenete fiel al input. No inventes hechos.
- Preservá nombres propios, fechas, números exactos.
- Idioma del resumen: igual al del input.
- Si el input ya es ≤ max_sentences, devolvelo tal cual.
</reglas>

<example>
Input texto: "La reunión del lunes 14 de junio definió que vamos a usar Postgres 16 con pgvector. Juan Pérez liderará la migración desde MongoDB. Presupuesto aprobado: 5000 USD. Deadline: 30 de julio."
max_oraciones: 2
formato_bullets: false

Output: "Reunión del 14/06 decidió migrar de MongoDB a Postgres 16 + pgvector, liderada por Juan Pérez con 5000 USD asignados. Deadline 30/07."
</example>`,
			TimeoutSeconds: 30,
			Idempotent:     true,
			Tags:           []string{"text", "summary", "platform"},
		},
		{
			Slug:        "extract-entities",
			Name:        "Extract Entities",
			Description: "Extrae entidades nombradas (persona, org, fecha, dinero, ubicación, producto) de texto.",
			SkillType:   "prompt",
			Content: `<role>
Extraer entidades nombradas de texto. NO inferir; solo lo que está
explícito o claramente derivable.
</role>

<input>
{{ .text }}
</input>

<entidades_a_extraer>
- person: nombres propios de personas (con cargo si aparece)
- org: empresas, organizaciones, equipos
- date: fechas absolutas (2026-06-15) o relativas ("el lunes")
- money: montos con moneda
- location: lugares, ciudades, países
- product: nombres de productos, herramientas, frameworks mencionados
</entidades_a_extraer>

<output_format>
JSON estricto:
{
  "person":   [{"text": "...", "context": "1-5 palabras"}],
  "org":      [...],
  "date":     [...],
  "money":    [{"text":"5000 USD","amount":5000,"currency":"USD"}],
  "location": [...],
  "product":  [...]
}
Si una categoría no tiene matches, devolvela como [].
</output_format>

<reglas>
- NO duplicar. "Juan" y "Juan Pérez" en mismo texto → 1 entry con full name.
- context: tokens cercanos que ayudan a desambiguar (ej. "Juan Pérez" context="lidera migración").
- money.amount: número limpio. currency: ISO si conocible, sino tal cual.
- NO inventes entidades. Si dudás, no incluyas.
</reglas>

<example>
Input: "Juan Pérez de Acme Corp asignó 5000 USD para migración a Postgres el lunes 14 de junio en Santiago."
Output:
{
  "person": [{"text":"Juan Pérez","context":"asignó presupuesto"}],
  "org": [{"text":"Acme Corp","context":"empleador de Juan"}],
  "date": [{"text":"lunes 14 de junio","context":"fecha asignación"}],
  "money": [{"text":"5000 USD","amount":5000,"currency":"USD"}],
  "location": [{"text":"Santiago","context":"ubicación"}],
  "product": [{"text":"Postgres","context":"target migración"}]
}
</example>`,
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
			ON CONFLICT (organization_id, slug) WHERE project_id IS NULL AND deleted_at IS NULL DO UPDATE
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
