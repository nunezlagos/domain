package steptypes

import (
	"fmt"
	"strings"
)

// ResolveTemplate reemplaza placeholders {{path}} usando inputs y outputs.
//
// Formatos:
//   - {{input.field}}         → inputs["field"]
//   - {{steps.step_id}}       → stepOutputs["step_id"]
//   - {{steps.step_id.field}} → stepOutputs["step_id"].(map)["field"]
//
// Si el placeholder no se resuelve, se deja intacto.
func ResolveTemplate(s string, inputs, outputs map[string]any) string {
	if !strings.Contains(s, "{{") || !strings.Contains(s, "}}") {
		return s
	}
	var buf strings.Builder
	buf.Grow(len(s))
	i := 0
	for i < len(s) {
		start := strings.Index(s[i:], "{{")
		if start < 0 {
			buf.WriteString(s[i:])
			break
		}
		buf.WriteString(s[i : i+start])
		end := strings.Index(s[i+start:], "}}")
		if end < 0 {
			buf.WriteString(s[i+start:])
			break
		}
		path := strings.TrimSpace(s[i+start+2 : i+start+end])
		resolved := resolvePath(path, inputs, outputs)
		if resolved == "" {

			buf.WriteString(s[i+start : i+start+end+2])
		} else {
			buf.WriteString(resolved)
		}
		i += start + end + 2
	}
	return buf.String()
}

// resolvePath resuelve una ruta como "input.field" o "steps.s1.result.status".
// Retorna un string vacío si no se puede resolver (el caller decide cómo manejar).
func resolvePath(path string, inputs, outputs map[string]any) string {
	parts := strings.SplitN(path, ".", 2)
	if len(parts) == 0 {
		return ""
	}
	root := parts[0]
	switch root {
	case "input", "inputs":
		if len(parts) < 2 {
			return ""
		}
		return resolveNested(parts[1], inputs)
	case "step", "steps", "output", "outputs":
		if len(parts) < 2 {
			return ""
		}

		subParts := strings.SplitN(parts[1], ".", 2)
		if len(subParts) == 0 {
			return ""
		}
		stepID := subParts[0]
		stepVal, ok := outputs[stepID]
		if !ok {
			return ""
		}
		if len(subParts) == 1 {
			return fmt.Sprint(stepVal)
		}
		stepMap, ok := stepVal.(map[string]any)
		if !ok {
			return ""
		}
		return resolveNested(subParts[1], stepMap)
	default:

		if v, ok := inputs[path]; ok {
			return fmt.Sprint(v)
		}
		if v, ok := outputs[path]; ok {
			return fmt.Sprint(v)
		}
		return ""
	}
}

// resolvePlaceholder retorna la string original si no pudo resolver.
func resolvePlaceholder(path string, inputs, outputs map[string]any) string {
	v := resolvePath(path, inputs, outputs)
	if v == "" {

		parts := strings.SplitN(path, ".", 2)
		if len(parts) == 0 {
			return ""
		}
		switch parts[0] {
		case "input", "inputs":
			if len(parts) > 1 && pathExists(parts[1], inputs) {
				return v
			}
		case "step", "steps", "output", "outputs":
			if len(parts) > 1 {
				subParts := strings.SplitN(parts[1], ".", 2)
				if len(subParts) > 0 {
					if _, ok := outputs[subParts[0]]; ok {
						return v
					}
				}
			}
		}
		return ""
	}
	return v
}

func resolvePlaceholderWithDefault(path string, inputs, outputs map[string]any, unresolved string) string {
	v := resolvePath(path, inputs, outputs)
	if v == "" {
		return unresolved
	}
	return v
}

// ResolveBarePaths resuelve referencias a steps.X.Y.Z en un string sin {{}}.
// Ej: "steps.s1.result.status == 'approved'" → "approved == 'approved'"
func ResolveBarePaths(s string, inputs, outputs map[string]any) string {
	if !strings.Contains(s, "steps.") && !strings.Contains(s, "step.") &&
		!strings.Contains(s, "inputs.") && !strings.Contains(s, "input.") {
		return s
	}
	var buf strings.Builder
	i := 0
	for i < len(s) {

		bestIdx := len(s)
		bestPrefix := ""
		for _, prefix := range []string{"steps.", "step.", "inputs.", "input."} {
			idx := strings.Index(s[i:], prefix)
			if idx >= 0 && i+idx < bestIdx {
				bestIdx = i + idx
				bestPrefix = prefix
			}
		}
		if bestPrefix == "" {
			buf.WriteString(s[i:])
			break
		}
		buf.WriteString(s[i:bestIdx])


		pathStart := bestIdx
		pathStr := bestPrefix
		j := bestIdx + len(bestPrefix)
		for j < len(s) && (isPathChar(s[j]) || s[j] == '.') {
			pathStr += string(s[j])
			j++
		}

		pathStr = strings.TrimRight(pathStr, ".")

		resolved := resolvePath(pathStr, inputs, outputs)
		if resolved != "" {
			buf.WriteString(resolved)
		} else {

			buf.WriteString(s[pathStart:j])
		}
		i = j
	}
	return buf.String()
}

// isPathChar returns true for valid identifier characters in paths.
func isPathChar(c byte) bool {
	return (c >= 'a' && c <= 'z') || (c >= 'A' && c <= 'Z') ||
		(c >= '0' && c <= '9') || c == '_'
}

// pathExists verifica si una ruta como "result.status" existe en un map.
func pathExists(path string, m map[string]any) bool {
	parts := strings.Split(path, ".")
	current := m
	for i, p := range parts {
		v, ok := current[p]
		if !ok {
			return false
		}
		if i == len(parts)-1 {
			return true
		}
		next, ok := v.(map[string]any)
		if !ok {
			return false
		}
		current = next
	}
	return true
}

// resolveNested resuelve una ruta punteada como "result.status" desde un map.
func resolveNested(path string, m map[string]any) string {
	parts := strings.Split(path, ".")
	current := m
	for i, p := range parts {
		if i == len(parts)-1 {
			if v, ok := current[p]; ok {
				return fmt.Sprint(v)
			}
			return ""
		}
		next, ok := current[p].(map[string]any)
		if !ok {
			return ""
		}
		current = next
	}
	return ""
}

// ResolveAllStrings recorre todos los string fields de config y aplica
// ResolveTemplate. Útil para pre-procesar params de un step.
func ResolveAllStrings(cfg map[string]any, inputs, outputs map[string]any) map[string]any {
	out := make(map[string]any, len(cfg))
	for k, v := range cfg {
		switch val := v.(type) {
		case string:
			out[k] = ResolveTemplate(val, inputs, outputs)
		default:
			out[k] = v
		}
	}
	return out
}
