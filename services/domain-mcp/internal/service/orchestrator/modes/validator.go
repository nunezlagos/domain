package modes

import (
	"fmt"

	"nunezlagos/domain/internal/service/orchestrator/phases"
)

// phaseDependencies define las dependencias directas del pipeline SDD.
// Cada entrada mapea una fase a sus fases precedentes REQUERIDAS.
// Una fase sólo puede ejecutarse si todas sus dependencias están
// presentes (no fueron skipeadas).
//
// explore no tiene dependencias (es la raíz del DAG).
var phaseDependencies = map[phases.PhaseSlug][]phases.PhaseSlug{
	"sdd-spec":    {"sdd-explore"},
	"sdd-propose": {"sdd-spec"},
	"sdd-design":  {"sdd-propose"},
	"sdd-tasks":   {"sdd-design"},
	"sdd-apply":   {"sdd-tasks"},
	"sdd-verify":  {"sdd-apply"},
	"sdd-judge":   {"sdd-verify"},
	"sdd-4r":      {"sdd-judge"},
	"sdd-review":  {"sdd-4r"},
	"sdd-archive": {"sdd-review"},
	"sdd-onboard": {"sdd-archive"},
}

// ValidateDAG verifica que SkipPhases no deje fases sin sus
// dependencias. Cada fase del pipeline SDD necesita que ciertas fases
// precedentes hayan sido ejecutadas (ver phaseDependencies). Si una
// fase dependiente se conserva pero su dependencia se salta, el DAG
// es inválido.
//
// La validación sólo considera el rango [startingPhase..fin] del
// catálogo base. Las fases anteriores a startingPhase se asumen ya
// ejecutadas (resume cross-session).
//
// Ejemplos:
//
//	Skip [judge, archive, onboard]         → OK  (terminal, nadie depende)
//	Skip [spec] pero keep [propose]        → ERROR (propose necesita spec)
//	Skip [apply] pero keep [verify]        → ERROR (verify necesita apply)
//	Skip [judge] pero keep [archive]       → ERROR (archive necesita judge)
//	Skip [archive] pero keep [onboard]     → ERROR (onboard necesita archive)
func ValidateDAG(base []phases.PhaseSlug, skipPhases []phases.PhaseSlug, startingPhase phases.PhaseSlug) error {
	if len(skipPhases) == 0 {
		return nil
	}
	skip := make(map[phases.PhaseSlug]struct{}, len(skipPhases))
	for _, s := range skipPhases {
		skip[s] = struct{}{}
	}
	startIdx := 0
	if startingPhase != "" {
		startIdx = -1
		for i, slug := range base {
			if slug == startingPhase {
				startIdx = i
				break
			}
		}
		if startIdx < 0 {
			return fmt.Errorf("validate_dag: starting_phase %q not found in catalog", startingPhase)
		}
	}

	kept := make(map[phases.PhaseSlug]struct{}, len(base)-startIdx)
	for _, slug := range base[startIdx:] {
		if _, ok := skip[slug]; !ok {
			kept[slug] = struct{}{}
		}
	}

	for slug := range kept {
		deps, hasDeps := phaseDependencies[slug]
		if !hasDeps {
			continue
		}
		for _, dep := range deps {

			if isBeforeStart(base, dep, startIdx) {
				continue
			}
			if _, ok := kept[dep]; !ok {
				return fmt.Errorf("validate_dag: phase %q requires %q but it was skipped", slug, dep)
			}
		}
	}
	return nil
}

// isBeforeStart reporta si slug aparece antes de startIdx en base.
func isBeforeStart(base []phases.PhaseSlug, slug phases.PhaseSlug, startIdx int) bool {
	for i, s := range base {
		if s == slug {
			return i < startIdx
		}
	}
	return false
}
