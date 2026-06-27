package skill_ab_test

import (
	"context"
	"errors"
	"fmt"

	"github.com/google/uuid"
)

// Service expone el ciclo de vida de los experimentos A/B:
// Create / Start / GetResults / DeclareWinner / Cancel. La logica estadistica
// vive en Analyzer (analyzer.go); el enrutamiento en Router (router.go).
type Service struct {
	repo Repository
}

// NewService inyecta el Repository.
func NewService(repo Repository) *Service {
	return &Service{repo: repo}
}

var (
	// ErrInvalidVersions: version_a == version_b (no tiene sentido un A/B con la
	// misma version en ambas ramas).
	ErrInvalidVersions = errors.New("version_a y version_b deben ser distintas")
	// ErrEmptySlug: skill_slug requerido.
	ErrEmptySlug = errors.New("skill_slug requerido")
	// ErrAlreadyRunning: ya existe un test 'running' para el slug (opt-in unico).
	ErrAlreadyRunning = errors.New("ya hay un A/B test running para este skill")
	// ErrNotFound: el test no existe.
	ErrNotFound = errors.New("A/B test no encontrado")
)

// Create valida y persiste un experimento. Normaliza defaults (split 0.50,
// min_invocations 100). Falla con ErrAlreadyRunning si ya hay uno running para el
// slug (el indice parcial lo garantiza tambien en DB).
func (s *Service) Create(ctx context.Context, p CreateParams) (*ABTest, error) {
	if p.SkillSlug == "" {
		return nil, ErrEmptySlug
	}
	if p.VersionA == p.VersionB {
		return nil, ErrInvalidVersions
	}
	if p.TrafficSplitA <= 0 || p.TrafficSplitA >= 1 {
		// 0 y 1 son validos como "todo a una rama" pero entonces no es A/B real;
		// fuera de (0,1) usamos el default. Permitimos exactamente 0/1 abajo.
		if p.TrafficSplitA != 0 && p.TrafficSplitA != 1 {
			p.TrafficSplitA = DefaultTrafficSplitA
		}
	}
	if p.MinInvocations <= 0 {
		p.MinInvocations = DefaultMinInvocations
	}

	// Pre-check de unicidad (la DB tambien lo garantiza via indice parcial).
	existing, err := s.repo.GetRunningBySlug(ctx, p.SkillSlug)
	if err != nil {
		return nil, fmt.Errorf("check running: %w", err)
	}
	if existing != nil {
		return nil, ErrAlreadyRunning
	}

	t, err := s.repo.Create(ctx, p)
	if err != nil {
		return nil, fmt.Errorf("create ab test: %w", err)
	}
	return t, nil
}

// Start marca el arranque temporal del experimento (started_at=NOW()).
func (s *Service) Start(ctx context.Context, id uuid.UUID) error {
	if err := s.repo.Start(ctx, id); err != nil {
		return fmt.Errorf("start ab test: %w", err)
	}
	return nil
}

// GetResults devuelve los agregados de ambas variantes del test.
func (s *Service) GetResults(ctx context.Context, id uuid.UUID) ([]VariantResult, error) {
	res, err := s.repo.GetResults(ctx, id)
	if err != nil {
		return nil, fmt.Errorf("get results: %w", err)
	}
	return res, nil
}

// Get devuelve un experimento por id.
func (s *Service) Get(ctx context.Context, id uuid.UUID) (*ABTest, error) {
	t, err := s.repo.Get(ctx, id)
	if err != nil {
		return nil, fmt.Errorf("get ab test: %w", err)
	}
	if t == nil {
		return nil, ErrNotFound
	}
	return t, nil
}

// DeclareWinner cierra el experimento con un ganador ('a'|'b'|'inconclusive') y
// su confidence. Valida el valor de winner.
func (s *Service) DeclareWinner(ctx context.Context, id uuid.UUID, winner string, confidence *float64) error {
	switch winner {
	case WinnerA, WinnerB, WinnerInconclusive:
	default:
		return fmt.Errorf("winner invalido: %q", winner)
	}
	if err := s.repo.DeclareWinner(ctx, id, winner, confidence); err != nil {
		return fmt.Errorf("declare winner: %w", err)
	}
	return nil
}

// Cancel cancela el experimento (status 'cancelled').
func (s *Service) Cancel(ctx context.Context, id uuid.UUID) error {
	if err := s.repo.Cancel(ctx, id); err != nil {
		return fmt.Errorf("cancel ab test: %w", err)
	}
	return nil
}

// AnalyzeResult resume el veredicto de UN test tras correr el analyzer.
type AnalyzeResult struct {
	TestID    uuid.UUID
	SkillSlug string
	Verdict   Verdict
	Declared  bool // se cerro el test con un ganador en esta pasada
	PinApplied bool // se aplico el pin del ganador (auto_apply)
}

// AnalyzeRunning corre el Analyzer sobre TODOS los tests 'running' y, por cada
// uno que alcanzo min_invocations, declara el resultado del z-test. Si el test
// tiene auto_apply_winner=TRUE (o autoApplyGlobal) y hay un ganador concreto
// ('a'|'b'), pinea esa version en el skill via skills.pinned_version (single-
// tenant, sin organization_id).
//
// ESTADISTICA PURA: el z-test corre en memoria (Analyzer). El cron resuelve la
// persistencia. Devuelve un resumen por test (los que aun no estan ready se
// incluyen con Declared=false).
func (s *Service) AnalyzeRunning(ctx context.Context, analyzer *Analyzer, autoApplyGlobal bool) ([]AnalyzeResult, error) {
	tests, err := s.repo.ListRunning(ctx)
	if err != nil {
		return nil, fmt.Errorf("list running: %w", err)
	}
	out := make([]AnalyzeResult, 0, len(tests))
	for _, t := range tests {
		ar, err := s.analyzeOne(ctx, analyzer, t, autoApplyGlobal)
		if err != nil {
			return out, fmt.Errorf("analyze test %s: %w", t.ID, err)
		}
		out = append(out, ar)
	}
	return out, nil
}

// analyzeOne corre el analyzer sobre un test y aplica el veredicto.
func (s *Service) analyzeOne(ctx context.Context, analyzer *Analyzer, t *ABTest, autoApplyGlobal bool) (AnalyzeResult, error) {
	ar := AnalyzeResult{TestID: t.ID, SkillSlug: t.SkillSlug}

	results, err := s.repo.GetResults(ctx, t.ID)
	if err != nil {
		return ar, fmt.Errorf("get results: %w", err)
	}
	resA, resB := splitResults(results)

	verdict := analyzer.Analyze(resA, resB, t.MinInvocations)
	ar.Verdict = verdict
	if !verdict.Ready {
		return ar, nil
	}

	var conf *float64
	if verdict.Winner != WinnerInconclusive {
		c := verdict.Confidence
		conf = &c
	}
	if err := s.repo.DeclareWinner(ctx, t.ID, verdict.Winner, conf); err != nil {
		return ar, fmt.Errorf("declare winner: %w", err)
	}
	ar.Declared = true

	// Auto-apply: por-test O global. Solo si hay ganador concreto.
	if (t.AutoApplyWinner || autoApplyGlobal) && verdict.Winner != WinnerInconclusive {
		if err := s.ApplyWinnerPin(ctx, t, verdict.Winner); err != nil {
			return ar, fmt.Errorf("apply winner pin: %w", err)
		}
		ar.PinApplied = true
	}
	return ar, nil
}

// splitResults separa los resultados en (A, B) por version, rellenando ceros si
// falta alguna variante (aun sin invocaciones).
func splitResults(results []VariantResult) (VariantResult, VariantResult) {
	resA := VariantResult{Version: string(VariantA)}
	resB := VariantResult{Version: string(VariantB)}
	for _, r := range results {
		switch r.Version {
		case string(VariantA):
			resA = r
		case string(VariantB):
			resB = r
		}
	}
	return resA, resB
}

// ApplyWinnerPin pinea la version ganadora en el skill (auto_apply_winner=TRUE).
// Resuelve skill_id desde el slug y setea pinned_version. Single-tenant: JAMAS
// usa organization_id (no existe en skills en runtime; bug HU-52.3 a NO repetir).
//
// No-op si winner es 'inconclusive' (no hay version a pinear).
func (s *Service) ApplyWinnerPin(ctx context.Context, t *ABTest, winner string) error {
	if winner != WinnerA && winner != WinnerB {
		return nil
	}
	skillID, err := s.repo.SkillIDBySlug(ctx, t.SkillSlug)
	if err != nil {
		return fmt.Errorf("resolve skill id: %w", err)
	}
	if skillID == uuid.Nil {
		// Skill renombrado/archivado: no rompe, solo no pinea.
		return nil
	}
	version := t.VersionA
	if winner == WinnerB {
		version = t.VersionB
	}
	if err := s.repo.PinSkillVersion(ctx, skillID, version); err != nil {
		return fmt.Errorf("pin skill version: %w", err)
	}
	return nil
}
