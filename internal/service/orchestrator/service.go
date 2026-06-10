package orchestrator

import (
	"context"
	"errors"
	"strings"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"

	"nunezlagos/domain/internal/audit"
	"nunezlagos/domain/internal/service/orchestrator/phases"
)

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
}

// New construye un Service. El registry debe venir poblado por el
// caller (boot wiring) — el orquestador no se auto-registra fases para
// permitir testing con handlers fake.
func New(pool *pgxpool.Pool, audit audit.Recorder, reg *phases.Registry, env string) *Service {
	return &Service{Pool: pool, Audit: audit, Phases: reg, Env: env}
}

// Run despacha el orquestador según el Mode. Devuelve OrchestrateResult
// con los IDs del flow_run creado (que el cliente IDE puede usar para
// pollear o reanudar) y el SnapshotPrompt si aplica.
//
// Esta función es el contrato externo y se va llenando incrementalmente.
// Hoy: validación + creación de flow_run + retorno de IDs. Los handlers
// de fases concretas se enchufan en svc-009..svc-018.
func (s *Service) Run(ctx context.Context, in OrchestrateInput) (*OrchestrateResult, error) {
	if err := s.validate(in); err != nil {
		return nil, err
	}
	mode := in.Mode
	if mode == "" {
		mode = ModeFull
	}
	// Stub: persistir el flow_run real cae en la siguiente iteración
	// (svc-003..svc-007 implementan dispatch + step creation). Aquí
	// devolvemos un OrchestrateResult válido con IDs nuevos para que
	// el caller pueda integrar el wiring contra una API estable.
	return &OrchestrateResult{
		OrchestratorRunID: uuid.New(),
		FlowRunID:         uuid.New(),
		Mode:              mode,
	}, nil
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
