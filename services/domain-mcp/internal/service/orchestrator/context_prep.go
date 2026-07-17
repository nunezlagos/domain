package orchestrator

import (
	"context"
	"strings"
	"time"

	"github.com/google/uuid"

	"nunezlagos/domain/internal/llm"
	"nunezlagos/domain/internal/llm/anthropic"
)

// REQ-54 issue-54.2: preparación de contexto server-side.
//
// Antes de entregarle el prompt de una fase al cliente, el servidor corre tools
// read-only baratas (policies del proyecto, skills aplicables, mem_context) y
// arma un bloque "prepared_context" que se inyecta en el user_prompt. Si hay un
// LLM barato disponible (Minimax), refina ese bloque filtrando lo pertinente a
// la fase. TODO es best-effort: nunca bloquea ni falla la fase.

// prepMaxPolicies / prepMaxSkills / prepMaxObs acotan el tamaño del bloque para
// no inflar el prompt del cliente. prepSkillBodyMax acota el body de skill
// inyectado (DOMAINSERV-38: antes solo la primera línea; ahora descripción útil).
const (
	prepMaxPolicies    = 10
	prepMaxSkills      = 5
	prepMaxObs         = 5
	prepSkillBodyMax   = 500
	prepMinimaxTimeout = 5 * time.Second
)

// skillsSectionHeader marca el bloque de skills en el crudo. refineWithMinimax lo
// usa para detectar drop del LLM y degradar a crudo (DOMAINSERV-38).
const skillsSectionHeader = "### Skills disponibles"

// prepPhaseToolCalls mapea cada fase a las categorías de contexto read-only que
// le sirven. El server decide QUÉ preparar por PhaseSlug (no via interfaz de
// handler, para no tocar los 11 handlers — decisión de alcance issue-54.2).
// Vacío/ausente = no se prepara nada (no-op, retrocompat).
// REQ-54 issue-54.6: las 12 fases registradas tienen entrada EXPLÍCITA —
// vacía = "sin prep, deliberado" (tasks/verify/archive: el contexto útil ya
// viene en PriorOutputs de la fase anterior). TestPrepContext_AllPhasesMapped
// congela la invariante: fase nueva sin entrada = test rojo.
// DOMAINSERV-38: skills se inyecta en las fases donde importa (apply + review),
// no solo apply.
var prepPhaseContext = map[string]struct {
	policies bool
	skills   bool
	obs      bool
}{
	"sdd-explore": {obs: true},
	"sdd-spec":    {obs: true},      // decisiones previas informan el contrato
	"sdd-propose": {policies: true}, // tradeoffs contra las reglas vigentes
	"sdd-design":  {policies: true, skills: true},
	"sdd-tasks":   {}, // el design (prior output) es el contexto
	"sdd-apply":   {policies: true, skills: true},
	"sdd-verify":  {},                                        // valida contra el issue.md, no contra contexto
	"sdd-judge":   {policies: true},                          // juzga también conformidad con las reglas
	"sdd-4r":      {policies: true, skills: true, obs: true}, // review 4R contra reglas + skills + contexto
	"sdd-review":  {policies: true, skills: true},
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
	var b strings.Builder
	if cfg.policies {
		s.prepPolicies(ctx, orgID, projectID, &b)
	}
	if cfg.skills {
		s.prepSkills(ctx, orgID, projectID, slug, &b)
	}
	if cfg.obs {
		s.prepObs(ctx, projectID, &b)
	}
	raw := b.String()
	if strings.TrimSpace(raw) == "" {
		return ""
	}
	return s.refineWithMinimax(ctx, slug, raw)
}

// refineWithMinimax pasa el bloque crudo por el LLM barato (Minimax) para filtrar
// lo pertinente a la fase, con timeout corto. DEGRADA al bloque crudo ante
// cualquier problema (sin LLM, timeout, error) o si el refine DROPPEA el bloque
// de skills que sí estaba en el crudo (DOMAINSERV-38): nunca aborta la fase.
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
		SystemPrompt: "Eres un asistente que filtra contexto para una fase de desarrollo. " +
			"Recibes policies, skills y memoria de un proyecto. Devuelve SOLO lo pertinente a la fase '" + slug + "', " +
			"conciso, en markdown. No inventes; si nada aplica, devuelve el input tal cual.",
		Messages: []llm.Message{{Role: "user", Content: raw}},
	})
	if err != nil || resp == nil || strings.TrimSpace(resp.Content) == "" {
		return raw // timeout/error/respuesta vacía: degradar a crudo
	}
	if strings.Contains(raw, skillsSectionHeader) && !strings.Contains(resp.Content, skillsSectionHeader) {
		return raw // el LLM droppeó el bloque de skills: degradar a crudo
	}
	return resp.Content
}
