// issue-05.6 agent-skill-contract — valida input/output de skills contra
// JSON Schema antes/después de invocar. Garantiza que el agente IA no
// envíe basura ni reciba data que rompe el contrato.
package skill

import (
	"encoding/json"
	"errors"
	"fmt"
)

// ErrSchemaInvalid se devuelve cuando un payload no satisface el schema.
var ErrSchemaInvalid = errors.New("payload no satisface JSON Schema")

// ValidationError detalla qué campo y por qué falló.
type ValidationError struct {
	Field   string `json:"field"`
	Reason  string `json:"reason"`
	Got     any    `json:"got,omitempty"`
}

// ValidationResult agrega errors si la validación falla.
type ValidationResult struct {
	Valid  bool              `json:"valid"`
	Errors []ValidationError `json:"errors,omitempty"`
}

// ValidatePayload aplica una validación minimalista (best-effort) contra
// un JSON Schema. Para MVP: verifica required, type básico, enum.
//
// Para precisión total integrar gojsonschema o similar.
func ValidatePayload(schema, payload []byte) ValidationResult {
	if len(schema) == 0 {
		return ValidationResult{Valid: true} // sin schema → todo válido
	}
	var sch map[string]any
	if err := json.Unmarshal(schema, &sch); err != nil {
		return ValidationResult{
			Valid: false,
			Errors: []ValidationError{{
				Field: "$schema", Reason: "schema no es JSON válido: " + err.Error(),
			}},
		}
	}
	var data map[string]any
	if err := json.Unmarshal(payload, &data); err != nil {
		return ValidationResult{
			Valid: false,
			Errors: []ValidationError{{
				Field: "$payload", Reason: "payload no es JSON válido: " + err.Error(),
			}},
		}
	}

	var errs []ValidationError


	if req, ok := sch["required"].([]any); ok {
		for _, r := range req {
			name, _ := r.(string)
			if _, ok := data[name]; !ok {
				errs = append(errs, ValidationError{
					Field: name, Reason: "required field missing",
				})
			}
		}
	}


	if props, ok := sch["properties"].(map[string]any); ok {
		for field, ps := range props {
			v, ok := data[field]
			if !ok {
				continue
			}
			pmap, ok := ps.(map[string]any)
			if !ok {
				continue
			}
			expectedType, _ := pmap["type"].(string)
			if expectedType != "" && !matchesType(v, expectedType) {
				errs = append(errs, ValidationError{
					Field: field,
					Reason: fmt.Sprintf("expected type %s", expectedType),
					Got:    v,
				})
			}

			if enum, ok := pmap["enum"].([]any); ok {
				if !inEnum(v, enum) {
					errs = append(errs, ValidationError{
						Field:  field,
						Reason: fmt.Sprintf("value not in enum %v", enum),
						Got:    v,
					})
				}
			}
		}
	}

	return ValidationResult{
		Valid:  len(errs) == 0,
		Errors: errs,
	}
}

func matchesType(v any, expected string) bool {
	switch expected {
	case "string":
		_, ok := v.(string)
		return ok
	case "number":
		switch v.(type) {
		case float64, float32, int, int64:
			return true
		}
		return false
	case "integer":
		switch x := v.(type) {
		case int, int64:
			return true
		case float64:
			return x == float64(int64(x))
		}
		return false
	case "boolean":
		_, ok := v.(bool)
		return ok
	case "array":
		_, ok := v.([]any)
		return ok
	case "object":
		_, ok := v.(map[string]any)
		return ok
	case "null":
		return v == nil
	}
	return true // tipo desconocido (no en JSON Schema spec) → permisivo
}

func inEnum(v any, enum []any) bool {
	for _, e := range enum {
		if e == v {
			return true
		}
	}
	return false
}
