package orchestrator

import (
	"fmt"
	"strings"

	"nunezlagos/domain/internal/service/orchestrator/phases"
)

// MissingRequiredSave describe qué save no apareció en el reporte del
// cliente. El service lo envuelve dentro de RequiredSaveError para que
// el caller pueda re-emitir el phase con los memory_refs faltantes.
type MissingRequiredSave struct {
	Type string
	Hint string
}

// RequiredSaveError envuelve ErrRequiredSaveMissing añadiendo el detalle
// de qué types específicos faltaron. errors.Is(err, ErrRequiredSaveMissing)
// sigue funcionando porque Unwrap apunta al sentinel.
type RequiredSaveError struct {
	Phase   phases.PhaseSlug
	Missing []MissingRequiredSave
}

func (e *RequiredSaveError) Error() string {
	types := make([]string, len(e.Missing))
	for i, m := range e.Missing {
		types[i] = m.Type
	}
	return fmt.Sprintf("orchestrator: phase %s missing required saves: %s",
		e.Phase, strings.Join(types, ", "))
}

func (e *RequiredSaveError) Unwrap() error { return ErrRequiredSaveMissing }

// MissingRequiredSaveInfo es la forma serializable de un save faltante que
// viaja al cliente en PhaseResultResult.MissingRequiredSaves. El cliente lo
// usa para saber qué tipo persistir (ej. code_reference) y con qué hint antes
// de reintentar el reporte de la fase.
type MissingRequiredSaveInfo struct {
	Type string `json:"type"`
	Hint string `json:"hint,omitempty"`
}

// ValidateRequiredSaves aplica D5: cada SuggestedSave con Required=true
// debe estar presente en ClientResult.MemoryRefsSaved por Type. Si
// alguno falta, devuelve *RequiredSaveError detallando cuáles.
//
// La comparación es case-sensitive sobre Type (los tipos son enums:
// "adr", "code_reference", "sabotage_record", "knowledge_doc", …).
// El service llama esto durante Validate() de cada handler; un
// handler concreto puede hacer chequeos adicionales (p.ej. shape del
// output) pero el contrato D5 está centralizado acá.
func ValidateRequiredSaves(phase phases.PhaseSlug, out *phases.Output, result phases.ClientResult) error {
	if out == nil {
		return nil
	}
	seen := make(map[string]struct{}, len(result.MemoryRefsSaved))
	for _, ref := range result.MemoryRefsSaved {
		seen[ref.Type] = struct{}{}
	}
	var missing []MissingRequiredSave
	for _, s := range out.SuggestedSaves {
		if !s.Required {
			continue
		}
		if _, ok := seen[s.Type]; !ok {
			missing = append(missing, MissingRequiredSave{Type: s.Type, Hint: s.Hint})
		}
	}
	if len(missing) == 0 {
		return nil
	}
	return &RequiredSaveError{Phase: phase, Missing: missing}
}
