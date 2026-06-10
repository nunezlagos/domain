package orchestrator

import (
	"context"
	"errors"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"

	"nunezlagos/domain/internal/audit"
	"nunezlagos/domain/internal/llm"
	"nunezlagos/domain/internal/metrics"
	"nunezlagos/domain/internal/service/orchestrator/modes"
	"nunezlagos/domain/internal/service/orchestrator/phases"
	"nunezlagos/domain/internal/tracing"

	"go.opentelemetry.io/otel/trace"
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
	// Repo encapsula la persistencia (flow lookup + flow_runs + steps).
	// Si nil, Service.Run en modo Express NO persistirá y devolverá
	// IDs in-memory — útil para tests unit sin DB. En boot real,
	// pasarlo explícitamente via NewPGRepository(pool).
	Repo Repository
	// Env replica config.Env. Empty o "dev" deshabilita enforcements
	// estrictos para iteración local; "prod" los habilita.
	Env string
	// Clock inyectable (default time.Now UTC). Tests sustituyen para
	// hacer determinista StartedAt.
	Clock Clock
	// Metrics opcional (issue-08.10 obs-001). Si nil, las métricas no
	// se incrementan; usado en tests que no levantan Prometheus.
	Metrics *metrics.Registry
	// LLM Factory para Mode=Solo (issue-08.10 svc-005). Si nil, RunSolo
	// devuelve ErrLLMFactoryRequired. Tests inyectan un factory con
	// providers fake; cmd/domain-mcp usa el global con providers reales.
	LLM *llm.Factory
}

// New construye un Service. El registry debe venir poblado por el
// caller (boot wiring) — el orquestador no se auto-registra fases para
// permitir testing con handlers fake.
//
// Si pool != nil, se construye un PGRepository automáticamente. Tests
// que quieran fakearlo pueden override `s.Repo` después de New.
func New(pool *pgxpool.Pool, audit audit.Recorder, reg *phases.Registry, env string) *Service {
	s := &Service{Pool: pool, Audit: audit, Phases: reg, Env: env, Clock: systemClock{}}
	if pool != nil {
		s.Repo = NewPGRepository(pool)
	}
	return s
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
	// OTel span: orchestrator.run. SafeAttrs whitelist evita PII; raw_text
	// NO se incluye porque puede contener datos sensibles del usuario.
	ctx, span := tracing.Tracer("orchestrator").Start(ctx, "orchestrator.run",
		trace.WithAttributes(
			tracing.SafeAttr("orchestrator.mode", string(in.Mode)),
			tracing.SafeAttr("org.id", in.OrganizationID.String()),
			tracing.SafeAttr("user.id", in.UserID.String()),
		),
	)
	defer span.End()

	if err := s.validate(in); err != nil {
		span.RecordError(err)
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
	if mode == ModeSolo {
		// Solo NO admite Repo nil (necesita BD para agent_templates y flow_runs)
		// ni LLM nil. Express/Full/Detect tienen un fallback in-memory útil
		// para tests; Solo es server-side puro y no tiene sentido sin BD.
		if s.Repo == nil {
			return nil, errors.New("orchestrator: Repo required for Solo mode")
		}
		flowID, err := s.Repo.GetFlowIDBySlug(ctx, in.OrganizationID, "sdd-pipeline-v1")
		if err != nil {
			return nil, err
		}
		plan, err := modes.BuildFullPlan(ctx, s.Phases, phases.Input{
			OrganizationID: in.OrganizationID,
			UserID:         in.UserID,
			FlowRunID:      res.FlowRunID,
			RawText:        in.RawText,
		}, phases.PhaseSlug(in.StartingPhase),
			convertSkipPhases(in.SkipPhases), now)
		if err != nil {
			return nil, err
		}
		if err := s.hydrateSystemPrompts(ctx, in.OrganizationID, plan); err != nil {
			return nil, err
		}
		if err := s.persistPlan(ctx, in, mode,
			res.OrchestratorRunID, flowID, res.FlowRunID, plan, now); err != nil {
			return nil, err
		}
		if s.Metrics != nil {
			s.Metrics.OrchestratorRunsTotal.WithLabelValues(string(mode), "started").Inc()
		}
		if err := s.runSolo(ctx, in, flowID, res.FlowRunID, res.OrchestratorRunID, plan); err != nil {
			return nil, err
		}
		res.Plan = exportPlan(plan)
		span.SetAttributes(
			tracing.SafeAttr("orchestrator.run_id", res.OrchestratorRunID.String()),
			tracing.SafeAttr("flow_run.id", res.FlowRunID.String()),
		)
		return res, nil
	}
	if mode == ModeExpress || mode == ModeFull || mode == ModeDetect {
		// Si hay Repo configurado, resolver el flow_id ANTES de armar
		// el plan: queremos fallar rápido (ErrFlowNotSeeded) sin haber
		// hecho trabajo de prompts si la org no está inicializada.
		// Detect requiere el flow seedeado igual (queremos validar que
		// la org está lista para Full antes de devolver preview).
		var flowID uuid.UUID
		if s.Repo != nil {
			var err error
			flowID, err = s.Repo.GetFlowIDBySlug(ctx, in.OrganizationID, "sdd-pipeline-v1")
			if err != nil {
				return nil, err
			}
		}
		phaseInput := phases.Input{
			OrganizationID: in.OrganizationID,
			UserID:         in.UserID,
			FlowRunID:      res.FlowRunID,
			RawText:        in.RawText,
		}
		var (
			plan *modes.PhasePlan
			err  error
		)
		switch mode {
		case ModeExpress:
			plan, err = modes.BuildExpressPlan(ctx, s.Phases, phaseInput, now)
		case ModeFull, ModeDetect:
			// Detect usa el mismo planner que Full — la diferencia es
			// que NO persistimos abajo (dry-run).
			plan, err = modes.BuildFullPlan(ctx, s.Phases, phaseInput,
				phases.PhaseSlug(in.StartingPhase),
				convertSkipPhases(in.SkipPhases), now)
		}
		if err != nil {
			return nil, err
		}
		// Hydrate los system_prompts desde agent_templates (BD source-of-truth).
		// Los handlers devuelven SystemPrompt="" intencionalmente; el Service
		// hace lookup acá por cada step usando AgentTemplateSlug. Esto
		// permite que operadores customicen prompts vía UI/MCP sin
		// recompilar binario. Detect TAMBIÉN hidrata: el preview es útil
		// sólo si refleja los prompts reales que correrá Full.
		if s.Repo != nil {
			if err := s.hydrateSystemPrompts(ctx, in.OrganizationID, plan); err != nil {
				return nil, err
			}
		}
		// Persistir SÓLO en modos no-dry-run. Detect devuelve plan
		// in-memory para inspección sin ensuciar BD; si el caller
		// quiere ejecutar de verdad, invoca el orquestador de nuevo
		// con Mode=ModeFull.
		if s.Repo != nil && mode != ModeDetect {
			if err := s.persistPlan(ctx, in, mode,
				res.OrchestratorRunID, flowID, res.FlowRunID, plan, now); err != nil {
				return nil, err
			}
		}
		res.Plan = exportPlan(plan)
		if len(res.Plan.Steps) > 0 {
			res.SnapshotPrompt = res.Plan.Steps[0].UserPrompt
		}
		// Métricas: orchestrator_runs_total{mode, status="started"}.
		// El status terminal (completed/failed) se incrementa cuando el
		// flow_run cambia de estado vía propagateFlowStatusAfterFailure
		// o cuando la última fase termina vía RecordPhaseResult.
		if s.Metrics != nil {
			s.Metrics.OrchestratorRunsTotal.WithLabelValues(string(mode), "started").Inc()
		}
		span.SetAttributes(
			tracing.SafeAttr("orchestrator.run_id", res.OrchestratorRunID.String()),
			tracing.SafeAttr("flow_run.id", res.FlowRunID.String()),
		)
	}
	return res, nil
}

// convertSkipPhases pasa el slice del API público al tipo del subpaquete
// phases sin reexportar el tipo desde el service.
func convertSkipPhases(in []PhaseSlug) []phases.PhaseSlug {
	if len(in) == 0 {
		return nil
	}
	out := make([]phases.PhaseSlug, len(in))
	for i, p := range in {
		out[i] = phases.PhaseSlug(p)
	}
	return out
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

// hydrateSystemPrompts rellena step.SystemPrompt para cada step del plan
// desde agent_templates en BD. Si el lookup falla con
// ErrAgentTemplateNotFound, devolvemos error para que el caller corrija
// el seed (no hay default sano para un prompt en blanco).
func (s *Service) hydrateSystemPrompts(ctx context.Context, orgID uuid.UUID, plan *modes.PhasePlan) error {
	if plan == nil {
		return nil
	}
	// Cache local por slug para no repetir queries cuando dos steps
	// comparten template (raro pero posible).
	cache := make(map[string]string, len(plan.Steps))
	for i := range plan.Steps {
		slug := plan.Steps[i].AgentTemplateSlug
		if slug == "" {
			continue
		}
		if cached, ok := cache[slug]; ok {
			plan.Steps[i].SystemPrompt = cached
			continue
		}
		prompt, err := s.Repo.GetAgentTemplateSystemPrompt(ctx, orgID, slug)
		if err != nil {
			return err
		}
		cache[slug] = prompt
		plan.Steps[i].SystemPrompt = prompt
	}
	return nil
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
