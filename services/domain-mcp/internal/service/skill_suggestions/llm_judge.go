package skill_suggestions

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"

	"nunezlagos/domain/internal/llm"
	"nunezlagos/domain/internal/llm/failover"
)

// ErrJudgeUnavailable se devuelve cuando se pide el judge LLM pero MiniMax no
// esta configurado (sin MINIMAX_API_KEY). El cron, al recibirlo, loguea limpio y
// NO corre (regla dura 7). No crashea.
var ErrJudgeUnavailable = errors.New("judge LLM requiere MINIMAX_API_KEY")

// judgeMaxTokens techo de la respuesta del judge (lista de sugerencias).
const judgeMaxTokens = 4096

// refineMaxTokens techo del content generado en un REFINE/SPLIT.
const refineMaxTokens = 8192

// LLMJudge razona sobre un skill (metricas 30d + feedback + top-3 similares por
// similitud lexica, SIN embeddings) y propone sugerencias split|merge|refine|
// archive. Tambien implementa Refiner (genera el content de un REFINE/SPLIT al
// momento del Apply). Resuelve el provider 'minimax' del Factory; si no esta,
// degrada con ErrJudgeUnavailable.
type LLMJudge struct {
	LLM *llm.Factory
}

// provider resuelve el provider/modelo del rol "judge" (config-driven,
// DOMAINSERV-57) o ErrJudgeUnavailable. Env: DOMAIN_LLM_JUDGE_{PROVIDER,MODEL}.
func (j *LLMJudge) provider() (llm.Provider, string, error) {
	if j == nil || j.LLM == nil {
		return nil, "", ErrJudgeUnavailable
	}
	p, model, err := failover.ForRole(j.LLM, llm.RoleJudge, nil)
	if err != nil {
		return nil, "", ErrJudgeUnavailable
	}
	return p, model, nil
}

// Available indica si el judge puede correr (provider resoluble). El cron lo
// consulta para degradar limpio antes de escanear.
func (j *LLMJudge) Available() bool {
	_, _, err := j.provider()
	return err == nil
}

// RefineContent implementa Refiner: genera el nuevo content de un skill a partir
// del actual + una instruccion. Lo usa Apply (REFINE/SPLIT) cuando el payload no
// trae content ya resuelto. Degrada con ErrJudgeUnavailable si no hay LLM.
func (j *LLMJudge) RefineContent(ctx context.Context, skillSlug, currentContent, instruction string) (string, error) {
	provider, model, err := j.provider()
	if err != nil {
		return "", err
	}
	system := "Eres un editor experto de skills (prompts/instrucciones reutilizables) de un agente. " +
		"Te dan el CONTENIDO ACTUAL de un skill y una INSTRUCCION de mejora. " +
		"Devuelves UNICAMENTE el nuevo contenido del skill, listo para guardar, sin explicaciones, " +
		"sin markdown de cierre, sin prefijos. Conserva el formato y el idioma del original."
	var user strings.Builder
	fmt.Fprintf(&user, "Skill: %s\n\n", skillSlug)
	fmt.Fprintf(&user, "Instruccion de mejora:\n%s\n\n", strings.TrimSpace(instruction))
	fmt.Fprintf(&user, "Contenido actual:\n%s\n", currentContent)

	resp, err := provider.Complete(ctx, llm.CompletionOptions{
		Model:        model,
		Temperature:  0,
		MaxTokens:    refineMaxTokens,
		SystemPrompt: system,
		Messages:     []llm.Message{{Role: "user", Content: user.String()}},
	})
	if err != nil || resp == nil {
		return "", fmt.Errorf("refine LLM fallo: %w", err)
	}
	out := strings.TrimSpace(resp.Content)
	if out == "" {
		return "", fmt.Errorf("refine LLM devolvio vacio")
	}
	return out, nil
}

// SkillInput es el contexto que el cron arma por skill para que el judge razone.
// Todo es data ya leida (metricas 30d, feedback, similares lexicos): el judge no
// toca la DB, solo el LLM. Esto lo hace testeable y desacoplado.
type SkillInput struct {
	Slug        string
	Name        string
	Description string
	Content     string // recortado por el caller
	SeedManaged bool

	// Senales (de skill_metrics_daily / skill_feedback, ventana 30d).
	InvocationsPerDay  float64
	FailureRate        float64 // 0..100
	AvgDurationSeconds float64
	NegativeFeedback   int
	DaysSinceLastUse   int // -1 si nunca / desconocido

	// Top-3 similares por similitud lexica (tsvector ts_rank), no embeddings.
	Similar []SimilarSkill
}

// SimilarSkill es un skill similar (lexico) al objetivo.
type SimilarSkill struct {
	Slug  string  `json:"slug"`
	Name  string  `json:"name"`
	Score float64 `json:"score"`
}

// judgeSuggestion es una sugerencia cruda del LLM (antes de validar/persistir).
type judgeSuggestion struct {
	Kind       string          `json:"kind"`
	Confidence float64         `json:"confidence"`
	Rationale  string          `json:"rationale"`
	Payload    json.RawMessage `json:"payload"`
}

type judgeResponse struct {
	Suggestions []judgeSuggestion `json:"suggestions"`
}

// Evaluate pide al LLM sugerencias para UN skill. Devuelve solo las validas
// (kind valido + confidence >= ConfidenceThreshold). El caller (cron) las
// persiste via Service.Create (que ademas deduplica). Degrada con
// ErrJudgeUnavailable si no hay LLM.
//
// Reglas que el prompt le pide al modelo (la spec):
//   - SPLIT:   invocations/dia alto Y content grande Y args variados.
//   - MERGE:   similar (lexico alto) Y uso bajo en ambos.
//   - REFINE:  failure_rate>30% O avg_duration>30s O negative_feedback>=3 (30d).
//   - ARCHIVE: 0 invocaciones en 90 dias Y NO seed_managed.
func (j *LLMJudge) Evaluate(ctx context.Context, in SkillInput) ([]CreateInput, error) {
	provider, model, err := j.provider()
	if err != nil {
		return nil, err
	}

	system, user := buildJudgePrompt(in)
	resp, err := provider.Complete(ctx, llm.CompletionOptions{
		Model:        model,
		Temperature:  0,
		MaxTokens:    judgeMaxTokens,
		SystemPrompt: system,
		Messages:     []llm.Message{{Role: "user", Content: user}},
	})
	if err != nil || resp == nil {
		return nil, fmt.Errorf("judge LLM fallo: %w", err)
	}

	parsed, perr := parseJudgeResponse(resp.Content)
	if perr != nil {
		return nil, fmt.Errorf("parseo respuesta judge: %w", perr)
	}

	out := make([]CreateInput, 0, len(parsed))
	for _, sg := range parsed {
		kind := strings.ToLower(strings.TrimSpace(sg.Kind))
		if !validKind(kind) {
			continue // anti-alucinacion
		}
		if sg.Confidence < ConfidenceThreshold {
			continue // confidence_threshold
		}
		// ARCHIVE de seed jamas (refuerzo; el Apply tambien lo bloquea).
		if kind == KindArchive && in.SeedManaged {
			continue
		}
		conf := sg.Confidence
		var ratPtr *string
		if r := strings.TrimSpace(sg.Rationale); r != "" {
			ratPtr = &r
		}
		mdl := model
		payload := []byte(sg.Payload)
		if len(payload) == 0 {
			payload = []byte("{}")
		}
		out = append(out, CreateInput{
			SkillSlug:  in.Slug,
			Kind:       kind,
			Payload:    payload,
			Rationale:  ratPtr,
			LLMModel:   &mdl,
			Confidence: &conf,
		})
	}
	return out, nil
}

// buildJudgePrompt arma un prompt determinista que pide JSON estricto.
func buildJudgePrompt(in SkillInput) (system, user string) {
	system = "Eres un auditor de skills (prompts/herramientas reutilizables) de un agente. " +
		"Te dan UN skill con sus metricas de 30 dias, feedback y skills similares. " +
		"Proponer 0 o mas acciones, eligiendo kind entre:\n" +
		"  split   : el skill hace demasiado (invocaciones/dia alto Y contenido grande Y argumentos variados). Dividir en hijos cohesivos.\n" +
		"  merge   : hay otro skill MUY similar y ambos se usan poco. Consolidar.\n" +
		"  refine  : el skill falla mucho (failure_rate>30%) O es lento (avg>30s) O junta feedback negativo (>=3 en 30d). Mejorar el contenido.\n" +
		"  archive : 0 invocaciones en 90 dias Y NO es seed. Archivar.\n" +
		"Se CONSERVADOR: si ninguna regla aplica con claridad, devuelve lista vacia. NO inventes skills ni slugs que no esten en los similares.\n" +
		"El payload depende del kind:\n" +
		"  split   -> {\"children\":[{\"slug\":\"\",\"name\":\"\",\"description\":\"\",\"instruction\":\"que debe hacer este hijo\"}, ...]}  (>=2 hijos)\n" +
		"  merge   -> {\"with\":[\"<slug>\"],\"merged_slug\":\"\",\"merged_name\":\"\",\"merged_content\":\"(opcional)\"}\n" +
		"  refine  -> {\"instruction\":\"que mejorar\",\"changelog\":\"\"}  (el contenido nuevo se genera al aplicar)\n" +
		"  archive -> {\"reason\":\"\"}\n" +
		"Responde EXCLUSIVAMENTE un objeto JSON:\n" +
		"{\"suggestions\":[{\"kind\":\"\",\"confidence\":0.0,\"rationale\":\"breve\",\"payload\":{...}}]}\n" +
		"confidence en 0..1. Sin texto adicional, sin markdown, sin fences."

	var sb strings.Builder
	fmt.Fprintf(&sb, "Skill objetivo:\n  slug=%s\n  name=%s\n  seed_managed=%t\n", in.Slug, in.Name, in.SeedManaged)
	fmt.Fprintf(&sb, "  description=%s\n", oneLineJudge(in.Description))
	fmt.Fprintf(&sb, "  content_len=%d chars\n", len(in.Content))
	fmt.Fprintf(&sb, "  content=%s\n", oneLineJudge(clipJudge(in.Content, 1500)))
	sb.WriteString("\nMetricas (30 dias):\n")
	fmt.Fprintf(&sb, "  invocaciones/dia=%.2f\n", in.InvocationsPerDay)
	fmt.Fprintf(&sb, "  failure_rate=%.1f%%\n", in.FailureRate)
	fmt.Fprintf(&sb, "  avg_duration=%.1fs\n", in.AvgDurationSeconds)
	fmt.Fprintf(&sb, "  feedback_negativo=%d\n", in.NegativeFeedback)
	fmt.Fprintf(&sb, "  dias_sin_uso=%d\n", in.DaysSinceLastUse)
	sb.WriteString("\nSkills similares (similitud lexica, top-3):\n")
	if len(in.Similar) == 0 {
		sb.WriteString("  (ninguno)\n")
	}
	for _, s := range in.Similar {
		fmt.Fprintf(&sb, "  - %s (%s) score=%.3f\n", s.Slug, s.Name, s.Score)
	}
	sb.WriteString("\nDevuelve solo el JSON {\"suggestions\":[...]} (puede estar vacio).")
	return system, sb.String()
}

// parseJudgeResponse extrae las sugerencias del JSON tolerando prosa/fences
// alrededor (mismo enfoque defensivo que parseInferenceDecisions).
func parseJudgeResponse(raw string) ([]judgeSuggestion, error) {
	s := strings.TrimSpace(raw)
	if s == "" {
		return nil, fmt.Errorf("respuesta vacia")
	}
	start := strings.Index(s, "{")
	if start < 0 {
		return nil, fmt.Errorf("sin objeto JSON")
	}
	depth, end := 0, -1
	for i := start; i < len(s); i++ {
		switch s[i] {
		case '{':
			depth++
		case '}':
			depth--
			if depth == 0 {
				end = i + 1
			}
		}
		if end > 0 {
			break
		}
	}
	if end < 0 {
		return nil, fmt.Errorf("JSON sin terminar")
	}
	var jr judgeResponse
	if err := json.Unmarshal([]byte(s[start:end]), &jr); err != nil {
		return nil, fmt.Errorf("unmarshal: %w", err)
	}
	return jr.Suggestions, nil
}

func clipJudge(s string, n int) string {
	if len(s) > n {
		return s[:n]
	}
	return s
}

func oneLineJudge(s string) string {
	return strings.ReplaceAll(strings.ReplaceAll(s, "\n", " "), "\r", " ")
}
