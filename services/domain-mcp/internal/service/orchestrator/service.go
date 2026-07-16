package orchestrator

import (
	"context"
	"errors"
	"log/slog"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"

	"nunezlagos/domain/internal/audit"
	"nunezlagos/domain/internal/llm"
	"nunezlagos/domain/internal/metrics"
	"nunezlagos/domain/internal/service/flow"
	"nunezlagos/domain/internal/service/orchestrator/modes"
	"nunezlagos/domain/internal/service/orchestrator/phases"
	usvc "nunezlagos/domain/internal/service/issue"
	obssvc "nunezlagos/domain/internal/service/observation"
	ppolicysvc "nunezlagos/domain/internal/service/projectpolicy"
	skillsvc "nunezlagos/domain/internal/service/skill"
	specsvc "nunezlagos/domain/internal/service/spec"
	tsvc "nunezlagos/domain/internal/service/task"
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

	Spec     *specsvc.Service
	Tasks    *tsvc.Service
	IssueSvc *usvc.Service



	Repo Repository


	Env string


	Clock Clock


	Metrics *metrics.Registry



	LLM *llm.Factory




	SignalStore *flow.SignalStore


	Skills *skillsvc.Service

	// REQ-54 issue-54.2: servicios read-only para preparar contexto server-side
	// (auto-policies / auto-mem). Opcionales (nil-safe): si no están inyectados,
	// la preparación degrada a lo que haya. Skills ya está arriba.
	ProjectPolicies *ppolicysvc.Service
	Observations    *obssvc.Service
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
		sig := analyzeComplexity(in.RawText)
		mode = decideMode(sig, in, s.LLM != nil)
	}
	now := s.now()
	res := &OrchestrateResult{
		OrchestratorRunID: uuid.New(),
		FlowRunID:         uuid.New(),
		Mode:              mode,
		StartedAt:         now,
	}
	if mode == ModeSolo {



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
		if err := s.hydrateSystemPrompts(ctx, in.OrganizationID, in.ProjectID, plan); err != nil {
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
		res.Plan = exportPlan(plan, true)
		span.SetAttributes(
			tracing.SafeAttr("orchestrator.run_id", res.OrchestratorRunID.String()),
			tracing.SafeAttr("flow_run.id", res.FlowRunID.String()),
		)
		return res, nil
	}
	if mode == ModeAsync {
		if s.Repo == nil {
			return nil, errors.New("orchestrator: Repo required for Async mode")
		}
		flowID, err := s.Repo.GetFlowIDBySlug(ctx, in.OrganizationID, "sdd-pipeline-v1")
		if err != nil {
			return nil, err
		}
		plan, err := modes.BuildAsyncPlan(ctx, s.Phases, phases.Input{
			OrganizationID: in.OrganizationID,
			UserID:         in.UserID,
			FlowRunID:      res.FlowRunID,
			RawText:        in.RawText,
		}, phases.PhaseSlug(in.StartingPhase),
			convertSkipPhases(in.SkipPhases), now)
		if err != nil {
			return nil, err
		}
		if err := s.hydrateSystemPrompts(ctx, in.OrganizationID, in.ProjectID, plan); err != nil {
			return nil, err
		}
		if err := s.persistPlan(ctx, in, mode,
			res.OrchestratorRunID, flowID, res.FlowRunID, plan, now); err != nil {
			return nil, err
		}
		result, err := s.runAsync(ctx, in, flowID, res.FlowRunID, res.OrchestratorRunID, plan)
		if err != nil {
			span.RecordError(err)
			return nil, err
		}
		span.SetAttributes(
			tracing.SafeAttr("orchestrator.run_id", result.OrchestratorRunID.String()),
			tracing.SafeAttr("flow_run.id", result.FlowRunID.String()),
		)
		return result, nil
	}
	if mode == ModeExpress || mode == ModeLite || mode == ModeFull || mode == ModeDetect {





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
		case ModeLite:




			plan, err = modes.BuildLitePlan(ctx, s.Phases, phaseInput, now)
		case ModeFull, ModeDetect:


			plan, err = modes.BuildFullPlan(ctx, s.Phases, phaseInput,
				phases.PhaseSlug(in.StartingPhase),
				convertSkipPhases(in.SkipPhases), now)
		}
		if err != nil {
			return nil, err
		}






		if s.Repo != nil {
			if err := s.hydrateSystemPrompts(ctx, in.OrganizationID, in.ProjectID, plan); err != nil {
				return nil, err
			}
		}




		persisted := s.Repo != nil && mode != ModeDetect
		if persisted {
			if err := s.persistPlan(ctx, in, mode,
				res.OrchestratorRunID, flowID, res.FlowRunID, plan, now); err != nil {
				return nil, err
			}
		}
		res.Plan = exportPlan(plan, persisted)
		if len(res.Plan.Steps) > 0 {
			res.SnapshotPrompt = res.Plan.Steps[0].UserPrompt
		}




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
func exportPlan(p *modes.PhasePlan, persisted bool) *PhasePlanSummary {
	if p == nil {
		return nil
	}
	out := &PhasePlanSummary{Mode: p.Mode, Steps: make([]PhaseStepSummary, len(p.Steps))}
	for i, st := range p.Steps {
		saves := make([]SuggestedSaveSummary, len(st.SuggestedSaves))
		for j, s := range st.SuggestedSaves {
			saves[j] = SuggestedSaveSummary{Type: s.Type, Required: s.Required, Hint: s.Hint}
		}
		// R4 / DOMAINSERV-3 (payload a dieta): en CUALQUIER modo persistido solo el
		// step 0 lleva el SystemPrompt (template + rulesBlock ~34KB) en el payload
		// inicial. Los steps 1..N lo reciben reconstruido vía NextStepSystemPrompt
		// en cada phase_result; su SystemPrompt completo sigue persistido en
		// step.Inputs. Los modos cortos (express/lite) SÍ sufrían el payload obeso
		// (medido: 129.601 chars en lite, el rulesBlock ~34KB duplicado por step),
		// por eso el strip ya no se limita a full.
		//
		// El step 0 se conserva (i > 0): no tiene phase_result previo que emita su
		// NextStepSystemPrompt, así que debe viajar inline.
		//
		// Solo se strippea si el plan se PERSISTIÓ: el modo detect (preview) NO
		// persiste, así que ahí el SystemPrompt no podría reconstruirse desde
		// step.Inputs — en ese caso se conserva en la salida.
		sysPrompt := st.SystemPrompt
		if persisted && i > 0 {
			sysPrompt = ""
		}
		out.Steps[i] = PhaseStepSummary{
			ID:                st.ID,
			Slug:              PhaseSlug(st.Slug),
			AgentTemplateSlug: st.AgentTemplateSlug,
			SystemPrompt:      sysPrompt,
			UserPrompt:        st.UserPrompt,
			SuggestedSaves:    saves,
			RetryPolicy:       string(st.RetryPolicy),
			SkillThreshold:    st.SkillThreshold,
			RequiredToolCalls: st.RequiredToolCalls,
			OutputSchema:      st.OutputSchema,
		}
	}
	return out
}

// hydrateSystemPrompts rellena step.SystemPrompt para cada step del plan
// desde agent_templates en BD. Si el lookup falla con
// ErrAgentTemplateNotFound, devolvemos error para que el caller corrija
// el seed (no hay default sano para un prompt en blanco).
//
// También extrae SkillThreshold desde agent_templates.metadata (D3).
func (s *Service) hydrateSystemPrompts(ctx context.Context, orgID, projectID uuid.UUID, plan *modes.PhasePlan) error {
	if plan == nil {
		return nil
	}


	// DOMAINSERV-24: el step 0 (único que viaja inline, DOMAINSERV-3) lleva las
	// policies extensas verbatim; los steps 1..N las stubbean. Se carga una vez y
	// se formatea dos veces (full para step 0, stub para el resto).
	platform, project := s.loadRulePolicies(ctx, projectID)
	rulesFull := formatRulesBlock(platform, project, false)
	rulesStub := formatRulesBlock(platform, project, true)
	type cached struct {
		base         string
		threshold    float64
		subagentPlan string
	}
	cache := make(map[string]cached, len(plan.Steps))
	for i := range plan.Steps {
		slug := plan.Steps[i].AgentTemplateSlug
		if slug == "" {
			continue
		}
		c, ok := cache[slug]
		if !ok {
			t, err := s.Repo.GetAgentTemplate(ctx, orgID, slug)
			if err != nil {
				return err
			}
			c = cached{base: t.SystemPrompt, threshold: t.SkillThreshold(), subagentPlan: t.SubagentPlan()}
			cache[slug] = c
		}
		rules := rulesStub
		if i == 0 {
			rules = rulesFull
		}
		plan.Steps[i].SystemPrompt = c.base + rules
		plan.Steps[i].SkillThreshold = c.threshold
	}
	// REQ-54 issue-54.2: preparación de contexto server-side por fase. Se inyecta
	// en el UserPrompt (no en SystemPrompt) para que sobreviva el lazy rebuild de
	// Full, que descarta SystemPrompt. Best-effort: prepareContext devuelve "" si
	// la fase no tiene contexto configurado o si todo falla — no bloquea.
	// Se hace por PhaseSlug (plan.Steps[i].Slug), no cacheado por template slug,
	// porque el contexto pertinente depende de la fase concreta.
	for i := range plan.Steps {
		step := &plan.Steps[i]
		// REQ-54 issue-54.5: plan de subagentes paralelos. El override del
		// template (BD) gana sobre el default del handler. Se inyecta ANTES
		// del prep para que el orden final sea prep → plan → prompt original.
		effPlan := step.SubagentPlan
		if c, ok := cache[step.AgentTemplateSlug]; ok && c.subagentPlan != "" {
			effPlan = c.subagentPlan
		}
		if effPlan != "" {
			step.UserPrompt = injectSubagentPlan(step.UserPrompt, effPlan)
		}
		prep := s.prepareContext(ctx, orgID, projectID, string(step.Slug))
		if prep != "" {
			step.UserPrompt = injectPreparedContext(step.UserPrompt, prep)
		}
	}
	return nil
}

// injectSubagentPlan antepone el plan de subagentes paralelos al user prompt
// de la fase (REQ-54 issue-54.5). El fan-out lo ejecuta el cliente.
func injectSubagentPlan(userPrompt, plan string) string {
	return "## Plan de subagentes (ejecutar EN PARALELO con tus subagentes)\n" +
		plan + "\n---\n" + userPrompt
}

// injectPreparedContext antepone el bloque de contexto preparado al user prompt
// de una fase, con un encabezado claro para el agente cliente.
func injectPreparedContext(userPrompt, prep string) string {
	return "## Contexto preparado por el servidor (REQ-54)\n" +
		prep + "\n---\n" + userPrompt
}

// maxInlinePolicyBody: las platform_policies con body más largo que esto no se
// re-embeben verbatim en el rulesBlock (DOMAINSERV-3): se reemplazan por un
// puntero a domain_policy_get para no inflar el SystemPrompt de cada fase por
// encima del límite del tool result de domain_orchestrate. Solo aplica a
// platform; las project_policies van SIEMPRE verbatim (son las convenciones
// específicas que el subagente más necesita inline). El umbral queda entre la
// 2da policy más grande (~1.9KB) y agent-protocol (~17.7KB): hoy solo se stubbea
// agent-protocol, y cualquier policy que crezca a futuro queda acotada sola.
const maxInlinePolicyBody = 4000

// policyBodyStub reemplaza un body extenso por un puntero a su texto vivo.
func policyBodyStub(slug string) string {
	return `(Body extenso no re-embebido en el rulesBlock. Texto vivo: domain_policy_get(slug="` + slug + `").)`
}

// rulePolicy es una policy ya cargada, lista para formatear en el rulesBlock.
type rulePolicy struct {
	slug, name, body, kind string
	override               bool
}

// loadRulePolicies lee las policies vigentes (platform + project). Best-effort
// PERO con señal: DOMAINSERV-23 — loguea en warn cuando no puede leer (pool nil o
// query fallida) en vez de dejar al agente sin reglas en silencio. Las reglas son
// aditivas, no bloqueantes, así que no propaga el error.
func (s *Service) loadRulePolicies(ctx context.Context, projectID uuid.UUID) (platform, project []rulePolicy) {
	if s.Pool == nil {
		slog.Default().Warn("orchestrator: rulesBlock vacío, pool nil — el agente no recibirá policies")
		return nil, nil
	}

	if rows, err := s.Pool.Query(ctx,
		`SELECT slug, name, COALESCE(body_md,''), COALESCE(kind,'')
		   FROM platform_policies WHERE is_active = TRUE
		   ORDER BY kind, slug`); err == nil {
		for rows.Next() {
			var p rulePolicy
			if rows.Scan(&p.slug, &p.name, &p.body, &p.kind) == nil && p.body != "" {
				platform = append(platform, p)
			}
		}
		rows.Close()
	} else {
		slog.Default().Warn("orchestrator: fallo al leer platform_policies para el rulesBlock", "err", err)
	}

	if projectID != uuid.Nil {
		if rows, err := s.Pool.Query(ctx,
			`SELECT slug, name, COALESCE(body_md,''), COALESCE(kind,''), override_platform
			   FROM project_policies
			   WHERE project_id = $1 AND is_active = TRUE
			     AND deleted_at IS NULL AND proposed = FALSE
			   ORDER BY kind, slug`, projectID); err == nil {
			for rows.Next() {
				var p rulePolicy
				if rows.Scan(&p.slug, &p.name, &p.body, &p.kind, &p.override) == nil && p.body != "" {
					project = append(project, p)
				}
			}
			rows.Close()
		} else {
			slog.Default().Warn("orchestrator: fallo al leer project_policies para el rulesBlock", "err", err, "project_id", projectID)
		}
	}
	return platform, project
}

// buildRulesBlock lee las policies vigentes y las formatea. stubLarge controla si
// las platform_policies extensas se stubbean (DOMAINSERV-3): true para los steps
// 1..N (payload a dieta), false para el step 0 que las lleva verbatim (DOMAINSERV-24).
func (s *Service) buildRulesBlock(ctx context.Context, projectID uuid.UUID, stubLarge bool) string {
	platform, project := s.loadRulePolicies(ctx, projectID)
	return formatRulesBlock(platform, project, stubLarge)
}

// formatRulesBlock arma el markdown de reglas. stubLarge=true stubbea las
// platform_policies extensas (steps 1..N, DOMAINSERV-3); stubLarge=false las
// embebe verbatim (step 0, DOMAINSERV-24). Las project_policies van siempre verbatim.
func formatRulesBlock(platform, project []rulePolicy, stubLarge bool) string {
	overridden := make(map[string]bool)
	for _, p := range project {
		if p.override && p.kind != "" {
			overridden[p.kind] = true
		}
	}

	var b strings.Builder
	write := func(p rulePolicy, canStub bool) {
		b.WriteString("\n### ")
		b.WriteString(p.name)
		b.WriteString("\n")
		if canStub && len(p.body) > maxInlinePolicyBody {
			b.WriteString(policyBodyStub(p.slug))
		} else {
			b.WriteString(p.body)
		}
		b.WriteString("\n")
	}
	for _, p := range platform {
		if !overridden[p.kind] {
			write(p, stubLarge)
		}
	}
	for _, p := range project {
		write(p, false)
	}
	if b.Len() == 0 {
		return ""
	}
	return "\n\n## Reglas vigentes (plataforma + proyecto)\n" + b.String()
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


	if in.ProjectID == uuid.Nil {
		return ErrProjectIDRequired
	}
	if in.Mode != "" && !in.Mode.IsValid() {
		return ErrInvalidMode
	}
	switch in.ExecMode {
	case "", "auto", "manual", "hybrid":
	default:
		return ErrInvalidExecMode
	}
	if in.Mode == ModeAsync && in.ExpressMaxLines > 0 {


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
