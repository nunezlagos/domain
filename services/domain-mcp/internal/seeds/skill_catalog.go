package seeds

import (
	"context"
	"encoding/json"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
)

// execer es la interfaz mínima compartida por *pgxpool.Pool y pgx.Tx.
// Permite que el cuerpo de los seeders de catálogo (UPSERT) corra tanto
// con un pool (wrappers SeedXForOrg, helpers de tests) como con una tx
// dentro del Registry (Seeder.Run, usado por `domain seed all`).
// Incluye QueryRow además de Exec porque los seeders de agent_templates y
// flows usan RETURNING (xmax=0) para distinguir Created de Updated.
type execer interface {
	Exec(ctx context.Context, sql string, args ...any) (pgconn.CommandTag, error)
	QueryRow(ctx context.Context, sql string, args ...any) pgx.Row
}

// skillsSeedVersion es la versión actual del catálogo de skills. Se usa
// tanto en el Seeder (SkillsCatalogSeeder.Version) como en el wrapper
// pool-based SeedSkillsForOrg.
const skillsSeedVersion = 8

// SkillsCatalogSeeder implementa el interface Seeder para el catálogo
// global de skills. Order > platform_policies/project_templates/mcp_providers.
type SkillsCatalogSeeder struct{}

func (s *SkillsCatalogSeeder) Name() string    { return "skills" }
func (s *SkillsCatalogSeeder) Version() int    { return skillsSeedVersion }
func (s *SkillsCatalogSeeder) Order() int      { return 50 }
func (s *SkillsCatalogSeeder) IsDevOnly() bool { return false }

func (s *SkillsCatalogSeeder) Run(ctx context.Context, tx pgx.Tx, _ Env) (Report, error) {
	return seedSkills(ctx, tx, skillsSeedVersion)
}

// SkillCatalog define las skills built-in globales del catálogo.
// Se materializan via SkillsCatalogSeeder dentro de `domain seed all`.
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
Eres un classifier de requerimientos del intake de domain. Recibes
texto libre del cliente y produces una clasificación estructurada
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
- reasoning conciso. No expliques los enums, justifica la elección.
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
Eres un structurer de requerimientos. Recibes el raw_text del cliente
junto con la classification (type, severity) ya hecha, y produces
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
		// code-search / file-read / web-fetch eliminadas (v6): eran stubs no
		// ejecutables (code/api sin implementación, issue-11.1/12.4). En la
		// arquitectura client-side el cliente (Claude Code/OpenCode) ya provee
		// esas capacidades nativas (Read, Grep/Glob, WebFetch). domain no las
		// reimplementa.
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
- Preserva nombres propios, fechas, números exactos.
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
- NO inventes entidades. Si dudas, no incluyas.
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

		{
			Slug:        "diff-summarize",
			Name:        "Diff Summarize",
			Description: "Convierte un git diff largo en un resumen estructurado (files_changed + lines + 3-5 bullets). Reduce el contexto del LLM al persistir/comparar diffs en lugar del diff completo.",
			SkillType:   "prompt",
			Content: `<role>
Resumir un git diff. NO inventar cambios que no están. NO opinar sobre
la calidad — solo reportar QUÉ cambió de forma estructurada.
</role>

<input>
{{ .diff }}
</input>

<output_format>
JSON estricto:
{
  "files_changed": N,
  "lines_added": N,
  "lines_removed": N,
  "files": [{"path":"...", "added":N, "removed":N, "category":"code|test|config|docs|migration"}],
  "key_changes": ["bullet 1", "bullet 2", "..."],
  "breaking_potential": "low | medium | high",
  "test_coverage_change": "added | removed | unchanged"
}
key_changes: 3-5 bullets, cada uno 1 oración describiendo el cambio
funcional (no "edité X líneas" sino "agregué endpoint POST /foo").
</output_format>

<reglas>
- breaking_potential=high si: cambio de signatura pública, drop column,
  rename de endpoint, cambio de behavior de API existente.
- NO inventes paths ni nombres de funciones que no estén en el diff.
- Si el diff es trivial (typo, comment), key_changes puede ser 1 bullet.
</reglas>

<example>
Input: diff con +50 -10 en services/foo/handler.go añadiendo GET /api/v1/foo
Output:
{
  "files_changed": 1,
  "lines_added": 50, "lines_removed": 10,
  "files": [{"path":"services/foo/handler.go","added":50,"removed":10,"category":"code"}],
  "key_changes": ["Agregado endpoint GET /api/v1/foo con paginación", "Removida validación duplicada de auth"],
  "breaking_potential": "low",
  "test_coverage_change": "unchanged"
}
</example>`,
			InputSchema: map[string]any{
				"type":     "object",
				"required": []string{"diff"},
				"properties": map[string]any{
					"diff": map[string]string{"type": "string"},
				},
			},
			TimeoutSeconds: 30,
			Idempotent:     true,
			Tags:           []string{"git", "diff", "reduce-context", "platform"},
		},
		{
			Slug:        "commit-message",
			Name:        "Commit Message Generator",
			Description: "Genera un Conventional Commit en español a partir de un diff. Respeta la policy no-co-authored-ia (sin Co-Authored-By).",
			SkillType:   "prompt",
			Content: `<role>
Generar mensaje de commit en formato Conventional Commits (español)
a partir de un diff. NO incluyas Co-Authored-By ni atribución a IA.
</role>

<input>
diff: {{ .diff }}
scope_hint: {{ .scope_hint }}
</input>

<types_validos>
feat | fix | refactor | docs | test | chore | perf | style | build | ci | revert
</types_validos>

<output_format>
JSON estricto:
{
  "subject": "feat(scope): hace X",
  "body": "Explicación 2-4 oraciones sobre el WHY del cambio.\n\nDetalles técnicos relevantes.",
  "type": "feat | fix | ...",
  "scope": "scope corto sin espacios",
  "breaking": boolean
}
subject: máximo 72 caracteres. Imperativo presente ("agrega", no "agregado").
body: separado de subject por línea en blanco. Explica POR QUÉ, no qué (el diff ya dice qué).
</output_format>

<reglas>
- Subject en español, imperativo: "agrega", "arregla", "refactoriza".
- NUNCA incluyas "Co-Authored-By: Claude" ni similar — está prohibido por policy.
- breaking=true → agrega "!" después del tipo: "feat(api)!: ..."
- scope: 1-2 palabras kebab-case del área tocada. Si es ambiguo, omitilo.
- Si hay BREAKING CHANGE en body, agrega línea "BREAKING CHANGE: ..." al final.
</reglas>

<example>
Input diff: agrega columna due_date a project_tickets + index
Output:
{
  "subject": "feat(tickets): agrega due_date + index para vencimientos",
  "body": "Habilita queries de tickets próximos a vencer sin seq scan.\n\nIndex parcial sobre (org, due_date) con WHERE status NOT IN ('done','cancelled').",
  "type": "feat",
  "scope": "tickets",
  "breaking": false
}
</example>`,
			InputSchema: map[string]any{
				"type":     "object",
				"required": []string{"diff"},
				"properties": map[string]any{
					"diff":       map[string]string{"type": "string"},
					"scope_hint": map[string]string{"type": "string"},
				},
			},
			TimeoutSeconds: 30,
			Idempotent:     true,
			Tags:           []string{"git", "commit", "platform"},
		},
		{
			Slug:        "error-classify",
			Name:        "Error / Stack Trace Classifier",
			Description: "Clasifica un stack trace o mensaje de error en {kind, severity, file_line, root_cause_hint, suggested_skill}. Reduce ruido para que el LLM sepa por dónde empezar.",
			SkillType:   "prompt",
			Content: `<role>
Analizar un stack trace / error log y devolver una clasificación
estructurada con la primera pista. NO resolver el bug; solo orientar.
</role>

<input>
{{ .error_text }}
</input>

<output_format>
JSON estricto:
{
  "kind": "panic | exception | compile_error | network | timeout | db_error | auth_error | validation | unknown",
  "severity": "low | medium | high | critical",
  "file_line": "path/to/file.go:42",
  "function": "Package.Function",
  "root_cause_hint": "1 oración con la pista más probable",
  "suggested_skill": "sql-explain-impact | null",
  "confidence": 0.0
}
</output_format>

<reglas>
- file_line: extraer el frame MÁS PROFUNDO del código del usuario (no del runtime).
- root_cause_hint: 1 oración. No inventes — si dudas, pista genérica + confidence<0.5.
- suggested_skill: cuál skill de domain ayudaría a investigar. null si ninguno
  aplica (para leer archivos o buscar en el código, usa tus tools nativos del
  cliente —Read, Grep— no una skill de domain).
- critical solo si: data loss, exposed secrets, prod down, security breach.
</reglas>

<example>
Input: "panic: runtime error: invalid memory address or nil pointer dereference\n  /app/internal/foo/svc.go:42 +0x2a"
Output:
{
  "kind": "panic",
  "severity": "high",
  "file_line": "/app/internal/foo/svc.go:42",
  "function": "",
  "root_cause_hint": "Nil pointer dereference en internal/foo/svc.go:42. Probablemente falta nil-check antes de acceder a un puntero.",
  "suggested_skill": null,
  "confidence": 0.85
}
</example>`,
			InputSchema: map[string]any{
				"type":     "object",
				"required": []string{"error_text"},
				"properties": map[string]any{
					"error_text": map[string]string{"type": "string"},
				},
			},
			TimeoutSeconds: 30,
			Idempotent:     true,
			Tags:           []string{"debug", "errors", "reduce-context", "platform"},
		},
		{
			Slug:        "gherkin-from-bug",
			Name:        "Gherkin Scenarios from Bug Report",
			Description: "Convierte un reporte de bug en lenguaje natural a scenarios Gherkin estructurados (given/when/then). Para sdd-spec phase.",
			SkillType:   "prompt",
			Content: `<role>
Convertir un reporte de bug en lenguaje natural a 1+ scenarios Gherkin
estructurados (BDD). Cada scenario captura UNA invariante concreta y
testeable. NO inventes contexto que no esté en el reporte.
</role>

<input>
bug_report: {{ .bug_report }}
context: {{ .context }}
</input>

<output_format>
JSON estricto:
{
  "feature": "nombre del feature/módulo afectado",
  "scenarios": [
    {
      "title": "Subject_action_outcome",
      "given": ["precondición 1", "precondición 2"],
      "when": "acción concreta del usuario",
      "then": ["resultado esperado 1", "resultado esperado 2"]
    }
  ],
  "missing_info": ["dato faltante 1 que el QA debería confirmar"]
}
</output_format>

<reglas>
- given: ARRAY de strings. Cada item es una precondición independiente.
- when: 1 acción concreta. Si el reporte tiene N acciones, son N scenarios.
- then: ARRAY de strings. Cada item es una aserción verificable.
- Si el reporte no dice repro steps claros, missing_info lo lista.
- NO inventes valores específicos (URLs, usuarios) — usa placeholders <user> <url>.
- title formato: "LoginForm_Submit_FailsInSafariIOS".
</reglas>

<example>
Input bug_report: "el botón de login no responde en Safari iOS al primer tap"
Output:
{
  "feature": "LoginForm",
  "scenarios": [
    {
      "title": "LoginForm_FirstTapInSafariIOS_DoesNotSubmit",
      "given": ["el usuario navega en Safari iOS 17+", "el form de login tiene credenciales válidas"],
      "when": "el usuario tappea el botón submit una vez",
      "then": ["el form NO se envía", "el usuario no ve cambio visual ni error", "el comportamiento esperado es que sí se envíe"]
    }
  ],
  "missing_info": ["versión exacta de Safari iOS confirmada", "si pasa también en Safari macOS"]
}
</example>`,
			InputSchema: map[string]any{
				"type":     "object",
				"required": []string{"bug_report"},
				"properties": map[string]any{
					"bug_report": map[string]string{"type": "string"},
					"context":    map[string]string{"type": "string"},
				},
			},
			TimeoutSeconds: 45,
			Idempotent:     true,
			Tags:           []string{"sdd", "bdd", "gherkin", "spec", "platform"},
		},
		{
			Slug:        "sql-explain-impact",
			Name:        "SQL Query Impact Analyzer",
			Description: "Analiza una query SQL y devuelve tablas tocadas, joins, riesgos (seq scan, n+1, locks). Para revisar queries antes de mergearlas.",
			SkillType:   "prompt",
			Content: `<role>
Analizar una query SQL y reportar su shape + riesgos de performance.
NO ejecutar la query. Solo análisis estático.
</role>

<input>
sql: {{ .sql }}
table_context: {{ .table_context }}
</input>

<output_format>
JSON estricto:
{
  "operation": "SELECT | INSERT | UPDATE | DELETE | DDL",
  "tables": ["tabla1", "tabla2"],
  "joins_count": N,
  "where_filters": ["col1", "col2"],
  "uses_index_hint": "yes | no | unknown",
  "risks": [
    {"kind": "seq_scan | n_plus_1 | lock_escalation | missing_where | full_table_lock", "severity":"low|medium|high", "explanation":"..."}
  ],
  "recommendations": ["..."],
  "estimated_complexity": "O(1) | O(log n) | O(n) | O(n log n) | O(n^2)"
}
</output_format>

<reglas>
- risks=[] si la query es trivial y segura.
- n_plus_1: SELECT dentro de loop conceptual (no detectable solo del SQL, marcalo solo si hay subselects que probablemente sean por-row).
- seq_scan: WHERE sobre columnas SIN index sugerido por table_context.
- missing_where: UPDATE o DELETE SIN WHERE. severity=critical.
- Recomendaciones concretas: "agregar index parcial WHERE deleted_at IS NULL".
</reglas>

<example>
Input sql: "SELECT * FROM project_tickets WHERE labels @> ARRAY['urgente']"
Output:
{
  "operation": "SELECT",
  "tables": ["project_tickets"],
  "joins_count": 0,
  "where_filters": ["labels"],
  "uses_index_hint": "yes",
  "risks": [],
  "recommendations": ["project_tickets tiene gin(labels) — query usa index. OK."],
  "estimated_complexity": "O(log n)"
}
</example>`,
			InputSchema: map[string]any{
				"type":     "object",
				"required": []string{"sql"},
				"properties": map[string]any{
					"sql":           map[string]string{"type": "string"},
					"table_context": map[string]string{"type": "string"},
				},
			},
			TimeoutSeconds: 30,
			Idempotent:     true,
			Tags:           []string{"sql", "performance", "review", "platform"},
		},
		{
			Slug:        "text-redact-secrets",
			Name:        "Text Secret Redactor",
			Description: "Reemplaza secrets (API keys, tokens, passwords, emails de prod) en texto por <redacted>. Útil antes de persistir logs en mem_save sin filtrar credenciales.",
			SkillType:   "prompt",
			Content: `<role>
Detectar y redactar credenciales / PII en texto. Devolver el texto
con los matches reemplazados por <redacted:KIND> (KIND es el tipo
detectado). Conservar la estructura y longitud aproximada.
</role>

<input>
{{ .text }}
</input>

<patrones_a_redactar>
- api_key: domk_*, sk-*, AKIA*, ghp_*, glpat-*, xoxb-*
- bearer_token: Bearer <token>, Authorization: <token>
- jwt: eyJ... con 3 segmentos base64
- password: campos JSON 'password', 'passwd', 'pwd' (valor)
- email: cualquier email completo (a@b.c)
- ssh_key: -----BEGIN ... PRIVATE KEY----- ... -----END
- aws_secret: 40-char base64 con prefix conocido
- rut_chileno: NN.NNN.NNN-N (sensible en Chile)
- credit_card: 13-19 dígitos con/sin espacios
- db_connection: postgres://user:pwd@host
</patrones_a_redactar>

<output_format>
JSON estricto:
{
  "redacted_text": "...texto con <redacted:api_key> en lugares apropiados...",
  "redactions": [
    {"kind":"api_key", "count": 2},
    {"kind":"email", "count": 5}
  ],
  "had_secrets": boolean
}
</output_format>

<reglas>
- NO devolver el secret original en redactions ni en ningún log.
- Si NO encuentras secrets, redacted_text=texto original sin cambios + had_secrets=false.
- Preservar formato (JSON sigue siendo JSON parseable post-redact).
- emails internos de devs (@saargo.com) también se redactan.
</reglas>

<example>
Input: 'curl -H "Authorization: Bearer domk_live_xyz123" https://api/users'
Output:
{
  "redacted_text": "curl -H \"Authorization: Bearer <redacted:api_key>\" https://api/users",
  "redactions": [{"kind":"api_key","count":1}],
  "had_secrets": true
}
</example>`,
			InputSchema: map[string]any{
				"type":     "object",
				"required": []string{"text"},
				"properties": map[string]any{
					"text": map[string]string{"type": "string"},
				},
			},
			TimeoutSeconds: 30,
			Idempotent:     true,
			Tags:           []string{"security", "privacy", "redaction", "platform"},
		},
		// ── Skills v5: tech-específicas → project-scoped via domain_project_skill_register ──
		{
			Slug:        "wcag-audit",
			Name:        "WCAG 2.2 Accessibility Audit",
			Description: "Auditoría de accesibilidad WCAG 2.2: principios POUR, niveles A/AA/AAA, 400+ criterios, patrones de remediación. Para proyectos con frontend web (Equable, dashboards, formularios).",
			SkillType:   "prompt",
			Content: `<role>
Eres un especialista en accesibilidad web WCAG 2.2. Realizas auditorías
completas y propones remediaciones concretas. Sabes qué se puede
detectar automáticamente (30-50%) y qué requiere revisión manual.
</role>

<principios_pour>
PERCEIVABLE: ¿pueden los usuarios percibir el contenido?
OPERABLE:    ¿pueden operar la interfaz con cualquier dispositivo?
UNDERSTANDABLE: ¿comprenden el contenido y cómo funciona?
ROBUST:      ¿funciona con tecnologías asistivas (lectores, switches)?
</principios_pour>

<niveles>
A   → baseline legal mínimo
AA  → estándar requerido por la mayoría de regulaciones
AAA → mejorado para necesidades especiales
</niveles>

<violaciones_por_severidad>
CRITICAL (bloquean acceso):
- Imágenes funcionales sin alt text
- Elementos interactivos sin acceso por teclado
- Formularios sin labels asociados
- Media auto-reproducible sin controles

SERIOUS (degradan experiencia):
- Contraste insuficiente: texto normal < 4.5:1, texto grande < 3:1
- Componentes UI < 3:1 contra fondo
- Sin skip links para navegación por teclado
- Custom widgets sin roles ARIA correctos
- Títulos de página ausentes o no descriptivos

MODERATE (afectan comprensión):
- Sin atributo lang en html
- Links "click aquí" sin contexto
- Sin landmarks (header/main/nav/footer)
- Jerarquía de headings rota (h1→h3 saltando h2)

REGLAS_FOCUS:
- Nunca remover outline de focus
- Focus visible: mínimo 3px sólido con 2px offset (offset blanco + ring azul)
- Sticky headers no deben ocultar el elemento con focus (WCAG 2.2)
- Hit targets: mínimo 44×44px
- Motion: respetar prefers-reduced-motion
- Color: nunca único indicador (combinar con icono + texto)
</violaciones_por_severidad>

<input>
{{ .request }}
</input>

<output_format>
JSON estricto:
{
  "violations": [
    {
      "criterion": "1.4.3",
      "level": "AA",
      "severity": "critical | serious | moderate",
      "element": "selector CSS o descripción",
      "issue": "descripción del problema",
      "remediation": "código o pasos concretos",
      "automated_detectable": boolean
    }
  ],
  "summary": {
    "critical": N, "serious": N, "moderate": N,
    "auto_detectable_pct": 0.0
  },
  "manual_checks_needed": ["check 1", "check 2"]
}
</output_format>`,
			TimeoutSeconds: 60,
			Idempotent:     true,
			Tags:           []string{"wcag", "accessibility", "a11y", "frontend", "platform"},
		},
		{
			Slug:        "requesting-code-review",
			Name:        "Code Review Request Protocol",
			Description: "Protocolo para solicitar y estructurar un code review: contexto del cambio, áreas de riesgo, criterios de calidad. Para cualquier proyecto del ecosistema Saargo.",
			SkillType:   "prompt",
			Content: `<role>
Ayudas a estructurar solicitudes de code review efectivas. Un buen
review request ahorra tiempo al reviewer y produce feedback más útil.
</role>

<self_review_primero>
Antes de pedir review, hacer self-review verificando:
- [ ] Tests verdes localmente
- [ ] Sin secrets hardcodeados
- [ ] Sin N+1 queries nuevas
- [ ] Archivos nuevos < 150 líneas
- [ ] Funciones < 30 líneas
- [ ] Inputs del usuario validados
- [ ] Commits convencionales en español sin Co-Authored-By
</self_review_primero>

<estructura_review_request>
Secciones obligatorias:
1. CONTEXTO: qué se quiere lograr y por qué (1-2 oraciones)
2. CAMBIOS: lista concreta de qué se modificó (por archivo/módulo)
3. ÁREAS DE RIESGO: qué podría salir mal (performance, seguridad, breaking)
4. CRITERIOS: qué hace que este cambio sea "correcto" para este proyecto
5. OUT OF SCOPE: qué no se revisa en este PR (evita scope creep)
</estructura_review_request>

<anti_patrones>
- "Solo echale un vistazo" — sin contexto ni criterios
- PR de 2000 líneas sin dividir por concern
- Submit sin self-review previo
- Mezclar refactor + feature en el mismo PR
- No mencionar las áreas de mayor riesgo
</anti_patrones>

<input>
diff o descripción del cambio: {{ .change_description }}
repositorio: {{ .repo_context }}
</input>

<output_format>
JSON estricto:
{
  "review_request": {
    "context": "...",
    "changes": [{"area": "...", "what": "...", "why": "..."}],
    "risk_areas": [{"area": "...", "risk": "...", "what_to_check": "..."}],
    "criteria": ["criterio 1", "criterio 2"],
    "out_of_scope": ["item 1"]
  },
  "self_review_passed": boolean,
  "self_review_gaps": ["gap si aplica"]
}
</output_format>`,
			TimeoutSeconds: 30,
			Idempotent:     true,
			Tags:           []string{"review", "quality", "workflow", "platform"},
		},
		{
			Slug:        "judgment-day",
			Name:        "Judgment Day — review adversarial dual",
			Description: "Protocolo de review adversarial: lanza dos subagentes jueces ciegos en paralelo sobre el mismo target, sintetiza hallazgos, aplica fixes y re-juzga hasta que ambos pasen o escala tras 2 iteraciones. Para revisar cambios significativos antes de mergear.",
			SkillType:   "prompt",
			Content: `<role>
Eres el orquestador de un review adversarial dual. Tu único trabajo es
coordinar: lanzas dos jueces ciegos en paralelo, sintetizas sus veredictos,
delegas los fixes y re-juzgas. NUNCA revisas el código tú mismo.
</role>

<cuando_usar>
- El usuario pide explícitamente "judgment day", "juzgar", "doble review".
- Tras una implementación significativa, antes de mergear.
- Cuando el costo de un bug en producción supera el de dos rondas de review.
</cuando_usar>

<protocolo>
1. SCOPE: si el target (archivos/feature/componente) no está claro, detente y
   pregunta antes de lanzar. Reviews parciales son inútiles.
2. REVIEW CIEGO PARALELO: lanza DOS subagentes con la Agent tool en paralelo.
   Ambos reciben el MISMO target pero trabajan independientes. Ninguno sabe del
   otro. Usan criterios idénticos.
3. SÍNTESIS: el orquestador (no un subagente) compara ambos resultados:
   - Confirmado   → lo hallan AMBOS       → alta confianza, fix inmediato
   - Sospecha A/B → lo halla UNO solo     → triage, escala al usuario
   - Contradicción → los jueces discrepan → marca para decisión manual
4. CLASIFICACIÓN DE WARNINGS: cada juez clasifica todo WARNING en:
   - real        → causa bug/pérdida de datos/hueco de seguridad en un escenario
                   de producción realista. Requiere fix.
   - teórico     → requiere escenario rebuscado o input corrupto que no surge en
                   uso normal. Se reporta como INFO, NO se arregla, NO re-juzga.
   Criterio: "¿un usuario normal, usando la herramienta como está previsto, lo
   dispara?" SÍ→real, NO→teórico.
5. FIX Y RE-JUDGE: si hay CRITICALs o WARNINGs reales confirmados, delega un Fix
   Agent (subagente separado, nunca uno de los jueces). Tras el fix, re-lanza
   ambos jueces en paralelo (mismo protocolo ciego).
6. UMBRAL DE CONVERGENCIA: tras la ronda 1, presenta la tabla de veredicto y
   pregunta al usuario si arregla. APROBADO = 0 CRITICALs + 0 WARNINGs reales
   confirmados (teóricos y sugerencias pueden quedar). Tras 2 iteraciones sin
   converger, pregunta al usuario si sigue; si dice que no, ESCALADO.
</protocolo>

<reglas_duras>
- NUNCA declares APROBADO hasta que los jueces devuelvan limpio (0 CRITICAL +
  0 WARNING real confirmado).
- NUNCA hagas push/commit tras aplicar fixes hasta completar el re-judge.
- El orquestador NUNCA revisa código: solo lanza jueces, lee resultados,
  sintetiza.
- El Fix Agent es una delegación aparte; jamás uses un juez como fixer.
- Espera SIEMPRE a que ambos jueces terminen antes de sintetizar.
</reglas_duras>

<prompt_juez>
Plantilla idéntica para AMBOS jueces:
"Eres un revisor de código adversarial. Tu ÚNICO trabajo es encontrar problemas.
Target: {describe}. Criterios: correctness, edge cases, manejo de errores,
performance (N+1, loops, allocs), seguridad (inyección, secrets, auth), naming
y convenciones del proyecto. Devuelve SOLO una lista de hallazgos, sin elogios.
Cada hallazgo: Severidad (CRITICAL | WARNING real | WARNING teórico | SUGERENCIA)
+ archivo:línea + descripción + fix sugerido (intención, no código). Si no hay
issues: 'VEREDICTO: LIMPIO'. Sé exhaustivo y adversarial."
</prompt_juez>

<input>
target: {{ .target }}
criterios_extra: {{ .custom_criteria }}
</input>

<output_format>
JSON estricto:
{
  "round": 1,
  "findings": [{"severity": "...", "file": "...", "line": 0, "desc": "...", "found_by": ["A","B"]}],
  "confirmed": ["..."],
  "verdict": "APROBADO | ESCALADO | PENDIENTE_FIX"
}
</output_format>`,
			InputSchema: map[string]any{
				"type": "object",
				"properties": map[string]any{
					"target":          map[string]string{"type": "string"},
					"custom_criteria": map[string]string{"type": "string"},
				},
				"required": []string{"target"},
			},
			TimeoutSeconds: 300,
			Idempotent:     true,
			Tags:           []string{"review", "adversarial", "quality", "workflow", "platform"},
		},
	}
}

// SeedSkillsForOrg aplica el catálogo global de skills usando un pool.
// El parámetro orgID quedó vestigial (las skills de catálogo son globales:
// project_id IS NULL); se conserva como helper pool-based para tests.
// Idempotente: ON CONFLICT (slug) UPDATE solo si is_user_modified=false.
func SeedSkillsForOrg(ctx context.Context, pool *pgxpool.Pool, _ uuid.UUID, version int) (Report, error) {
	return seedSkills(ctx, pool, version)
}

// seedSkills aplica el UPSERT idempotente del catálogo de skills usando
// cualquier execer (pool o tx). Compartido entre SeedSkillsForOrg (pool) y
// SkillsCatalogSeeder.Run (tx) para no duplicar el SQL.
func seedSkills(ctx context.Context, db execer, version int) (Report, error) {
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

		tag, err := db.Exec(ctx, `
			INSERT INTO skills (slug, name, description, skill_type,
			                    content, input_schema, output_schema, timeout_seconds,
			                    idempotent, has_side_effects, tags,
			                    seed_managed, seed_version)
			VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11, TRUE, $12)
			ON CONFLICT (slug) WHERE project_id IS NULL AND deleted_at IS NULL DO UPDATE
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
			e.Slug, e.Name, e.Description, e.SkillType,
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
