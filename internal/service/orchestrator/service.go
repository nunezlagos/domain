package orchestrator

import (
	"context"
	"errors"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"

	"nunezlagos/domain/internal/audit"
	"nunezlagos/domain/internal/service/orchestrator/modes"
	"nunezlagos/domain/internal/service/orchestrator/phases"
)

// Clock permite inyectar wall-clock para tests deterministas (regla
// .claude/rules/testing.md: nada de time.Now() directo en lógica).
type Clock interface{ Now() time.Time }

type systemClock struct{}

func (systemClock) Now() time.Time { return time.Now().UTC() }

// Service coordina la ejecución del pipeline SDD. Responsable de:
//   - validar OrchestrateInput (modos, fases, combinaciones D6)
//   - resolver el DAG sdd-pipeline-v1 según Mode + SkipPhases
//   - crear flow_runs + flow_run_steps coherentes
//   - despachar fases vía phases.Registry (Build → cliente IDE → Validate)
//   - aplicar D5 (suggested_saves required) sobre los results del cliente
//
// La implementación concreta de Run() y de los modos vive en
// modes/{express,full,solo,detect,async}.go (svc-003..svc-007). Este
// archivo declara el skeleton público para que el resto del wiring
// (MCP tools, PromptRouter, CLI) pueda referenciarlo sin esperar a las
// fases.
type Service struct {
	Pool    *pgxpool.Pool
	Audit   audit.Recorder
	Phases  *phases.Registry
	// Env replica config.Env. Empty o "dev" deshabilita enforcements
	// estrictos para iteración local; "prod" los habilita.
	Env string
	// Clock inyectable (default time.Now UTC). Tests sustituyen para
	// hacer determinista StartedAt.
	Clock Clock
}

// New construye un Service. El registry debe venir poblado por el
// caller (boot wiring) — el orquestador no se auto-registra fases para
// permitir testing con handlers fake.
func New(pool *pgxpool.Pool, audit audit.Recorder, reg *phases.Registry, env string) *Service {
	return &Service{Pool: pool, Audit: audit, Phases: reg, Env: env, Clock: systemClock{}}
}

// Run despacha el orquestador según el Mode. Devuelve OrchestrateResult
// con los IDs del flow_run creado (que el cliente IDE puede usar para
// pollear o reanudar) y el SnapshotPrompt si aplica.
//
// Estado de implementación por modo:
//   - Express: dispatcher in-memory completo (ya devuelve Plan ejecutable)
//   - Full / Solo / Detect / Async: stub temporal — devuelve IDs + Mode
//     sin Plan. Los modos restantes vienen en próximos chunks junto con
//     la persistencia flow_runs + dispatch loop MCP.
func (s *Service) Run(ctx context.Context, in OrchestrateInput) (*OrchestrateResult, error) {
	if err := s.validate(in); err != nil {
		return nil, err
	}
	mode := in.Mode
	if mode == "" {
		mode = ModeFull
	}
	now := s.now()
	res := &OrchestrateResult{
		OrchestratorRunID: uuid.New(),
		FlowRunID:         uuid.New(),
		Mode:              mode,
		StartedAt:         now,
	}
	if mode == ModeExpress {
		plan, err := modes.BuildExpressPlan(ctx, s.Phases, phases.Input{
			OrganizationID: in.OrganizationID,
			UserID:         in.UserID,
			FlowRunID:      res.FlowRunID,
			RawText:        in.RawText,
		}, now)
		if err != nil {
			return nil, err
		}
		res.Plan = exportPlan(plan)
		if len(res.Plan.Steps) > 0 {
			res.SnapshotPrompt = res.Plan.Steps[0].UserPrompt
		}
	}
	return res, nil
}

// exportPlan traduce el plan interno (modes.PhasePlan) al shape
// exportado (PhasePlanSummary). Mantiene aislado el package modes/
// del API público del service.
func exportPlan(p *modes.PhasePlan) *PhasePlanSummary {
	if p == nil {
		return nil
	}
	out := &PhasePlanSummary{Mode: p.Mode, Steps: make([]PhaseStepSummary, len(p.Steps))}
	for i, st := range p.Steps {
		saves := make([]SuggestedSaveSummary, len(st.SuggestedSaves))
		for j, s := range st.SuggestedSaves {
			saves[j] = SuggestedSaveSummary{Type: s.Type, Required: s.Required, Hint: s.Hint}
		}
		out.Steps[i] = PhaseStepSummary{
			ID:                st.ID,
			Slug:              PhaseSlug(st.Slug),
			AgentTemplateSlug: st.AgentTemplateSlug,
			SystemPrompt:      st.SystemPrompt,
			UserPrompt:        st.UserPrompt,
			SuggestedSaves:    saves,
			RetryPolicy:       string(st.RetryPolicy),
			SkillThreshold:    st.SkillThreshold,
		}
	}
	return out
}

// now devuelve la hora vía Clock o cae a UTC system si Clock fue nil
// (caso constructores que no usaron New).
func (s *Service) now() time.Time {
	if s.Clock == nil {
		return time.Now().UTC()
	}
	return s.Clock.Now()
}

// validate aplica las reglas del contrato (no del DAG):
//
//   - RawText no vacío (ErrEmptyRawText)
//   - Mode válido (ErrInvalidMode)
//   - D6: ModeAsync + ModeExpress = ErrAsyncModeUnsupported
//   - StartingPhase y SkipPhases referencian fases registradas
//
// La validación del DAG resultante (¿el SkipPhases deja un grafo
// ejecutable?) ocurre en modes/validator.go (svc-008).
func (s *Service) validate(in OrchestrateInput) error {
	if strings.TrimSpace(in.RawText) == "" {
		return ErrEmptyRawText
	}
	if in.Mode != "" && !in.Mode.IsValid() {
		return ErrInvalidMode
	}
	if in.Mode == ModeAsync && in.ExpressMaxLines > 0 {
		// ExpressMaxLines>0 sólo tiene sentido para Express. Si el caller
		// pidió async con ExpressMaxLines, es la combinación D6.
		return ErrAsyncModeUnsupported
	}
	if s.Phases != nil {
		if err := s.validatePhase(in.StartingPhase); err != nil {
			return err
		}
		for _, p := range in.SkipPhases {
			if err := s.validatePhase(p); err != nil {
				return err
			}
		}
	}
	return nil
}

// validatePhase tolera el zero value (sin override del default).
func (s *Service) validatePhase(p PhaseSlug) error {
	if p == "" {
		return nil
	}
	_, err := s.Phases.Lookup(phases.PhaseSlug(p))
	if err != nil {
		if errors.Is(err, phases.ErrPhaseNotRegistered) {
			return ErrUnknownPhase
		}
		return err
	}
	return nil
}
