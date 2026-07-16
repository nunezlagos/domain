package orchestrator

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"

	"nunezlagos/domain/internal/llm"
	"nunezlagos/domain/internal/llm/anthropic"
	skillsvc "nunezlagos/domain/internal/service/skill"
)

// REQ-54 issue-54.2: preparación de contexto server-side.
//
// Antes de entregarle el prompt de una fase al cliente, el servidor corre tools
// read-only baratas (policies del proyecto, skills aplicables, mem_context) y
// arma un bloque "prepared_context" que se inyecta en el user_prompt. Si hay un
// LLM barato disponible (Minimax), refina ese bloque filtrando lo pertinente a
// la fase. TODO es best-effort: nunca bloquea ni falla la fase.

// prepMaxPolicies / prepMaxSkills / prepMaxObs acotan el tamaño del bloque para
// no inflar el prompt del cliente.
const (
	prepMaxPolicies    = 10
	prepMaxSkills      = 10
	prepMaxObs         = 5
	prepMinimaxTimeout = 5 * time.Second
)

// prepPhaseToolCalls mapea cada fase a las categorías de contexto read-only que
// le sirven. El server decide QUÉ preparar por PhaseSlug (no via interfaz de
// handler, para no tocar los 11 handlers — decisión de alcance issue-54.2).
// Vacío/ausente = no se prepara nada (no-op, retrocompat).
// REQ-54 issue-54.6: las 11 fases registradas tienen entrada EXPLÍCITA —
// vacía = "sin prep, deliberado" (tasks/verify/archive: el contexto útil ya
// viene en PriorOutputs de la fase anterior). TestPrepContext_AllPhasesMapped
// congela la invariante: fase nueva sin entrada = test rojo.
var prepPhaseContext = map[string]struct {
	policies bool
	skills   bool
	obs      bool
}{
	"sdd-explore": {obs: true},
	"sdd-spec":    {obs: true},      // decisiones previas informan el contrato
	"sdd-propose": {policies: true}, // tradeoffs contra las reglas vigentes
	"sdd-design":  {policies: true},
	"sdd-tasks":   {}, // el design (prior output) es el contexto
	"sdd-apply":   {policies: true, skills: true},
	"sdd-verify":  {},                          // valida contra el issue.md, no contra contexto
	"sdd-judge":   {policies: true},            // juzga también conformidad con las reglas
	"sdd-4r":      {policies: true, obs: true}, // review 4R contra reglas + contexto reciente
	"sdd-review":  {policies: true},
	"sdd-archive": {},          // cierre administrativo
	"sdd-onboard": {obs: true}, // qué se aprendió antes de documentar
}

// prepareContext arma el bloque de contexto para una fase (crudo) y, si hay LLM,
// lo refina. Devuelve "" si la fase no tiene contexto configurado o si todo
// falla — el caller debe tratar "" como "sin prepared_context" (no error).
func (s *Service) prepareContext(ctx context.Context, orgID, projectID uuid.UUID, slug string) string {
	cfg, ok := prepPhaseContext[slug]
	if !ok {
		return "" // fase sin contexto configurado: no-op
	}
	raw := s.prepareContextRaw(ctx, orgID, projectID, cfg.policies, cfg.skills, cfg.obs)
	if strings.TrimSpace(raw) == "" {
		return ""
	}
	return s.refineWithMinimax(ctx, slug, raw)
}

// prepareContextRaw corre las lecturas read-only server-side y arma un bloque
// markdown. Best-effort por sección: si un servicio no está inyectado o falla,
// esa sección se omite (no aborta).
func (s *Service) prepareContextRaw(ctx context.Context, orgID, projectID uuid.UUID, wantPolicies, wantSkills, wantObs bool) string {
	var b strings.Builder

	if wantPolicies && s.ProjectPolicies != nil {
		if pols, err := s.ProjectPolicies.List(ctx, orgID, projectID, ""); err == nil && len(pols) > 0 {
			fmt.Fprintln(&b, "### Policies del proyecto (vigentes)")
			n := 0
			for _, p := range pols {
				if !p.IsActive {
					continue
				}
				fmt.Fprintf(&b, "- **%s** (%s): %s\n", p.Name, p.Kind, firstLine(p.BodyMD))
				if n++; n >= prepMaxPolicies {
					break
				}
			}
			fmt.Fprintln(&b)
		}
	}

	if wantSkills && s.Skills != nil {
		if skills, err := s.Skills.List(ctx, orgID, skillsvc.ListFilter{Limit: prepMaxSkills}); err == nil && len(skills) > 0 {
			fmt.Fprintln(&b, "### Skills disponibles")
			for _, sk := range skills {
				fmt.Fprintf(&b, "- **%s**: %s\n", sk.Name, firstLine(sk.Description))
			}
			fmt.Fprintln(&b)
		}
	}

	if wantObs && s.Observations != nil {
		if obs, err := s.Observations.List(ctx, projectID, prepMaxObs); err == nil && len(obs) > 0 {
			fmt.Fprintln(&b, "### Contexto reciente (memoria)")
			for _, o := range obs {
				fmt.Fprintf(&b, "- [%s] %s\n", o.ObservationType, firstLine(o.Content))
			}
			fmt.Fprintln(&b)
		}
	}

	return b.String()
}

// refineWithMinimax pasa el bloque crudo por el LLM barato (Minimax) para filtrar
// lo pertinente a la fase, con timeout corto. DEGRADA al bloque crudo ante
// cualquier problema (sin LLM, timeout, error): nunca aborta la fase.
func (s *Service) refineWithMinimax(ctx context.Context, slug, raw string) string {
	if s.LLM == nil {
		return raw
	}
	provider, err := s.LLM.Get(anthropic.MiniMaxProviderName)
	if err != nil {
		return raw // sin provider (falta LLM_API_KEY): degradar a crudo
	}
	cctx, cancel := context.WithTimeout(ctx, prepMinimaxTimeout)
	defer cancel()
	resp, err := provider.Complete(cctx, llm.CompletionOptions{
		Model:       anthropic.MiniMaxModel,
		Temperature: 0.2,
		MaxTokens:   1024,
		SystemPrompt: "Sos un asistente que filtra contexto para una fase de desarrollo. " +
			"Recibís policies, skills y memoria de un proyecto. Devolvé SOLO lo pertinente a la fase '" + slug + "', " +
			"conciso, en markdown. No inventes; si nada aplica, devolvé el input tal cual.",
		Messages: []llm.Message{{Role: "user", Content: raw}},
	})
	if err != nil || resp == nil || strings.TrimSpace(resp.Content) == "" {
		return raw // timeout/error/respuesta vacía: degradar a crudo
	}
	return resp.Content
}

// firstLine devuelve la primera línea no vacía de s, recortada, para resúmenes
// de una línea en el bloque de contexto.
func firstLine(s string) string {
	for _, ln := range strings.Split(s, "\n") {
		ln = strings.TrimSpace(ln)
		if ln != "" {
			if len(ln) > 160 {
				return ln[:157] + "..."
			}
			return ln
		}
	}
	return ""
}
