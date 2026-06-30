package steptypes

import (
	"context"
	"fmt"
	"strconv"
	"strings"
)

// TransformRunner evaluates a JSONPath or jq expression over the merged context.
//
// Config:
//
//	{"expression": "$.users[?(@.active==true)].email", "engine": "jsonpath"}
//
// Or:
//
//	{"expression": ".items | map(select(.price > 10))", "engine": "jq"}
//
// Output: the evaluated result (type varies by expression).
type TransformRunner struct{}

func (r *TransformRunner) Run(_ context.Context, input RunInput) (any, error) {
	expr := configString(input.Config, "expression")
	if expr == "" {
		return nil, fmt.Errorf("transform: expression required")
	}
	engine := configString(input.Config, "engine")
	if engine == "" {
		engine = "jsonpath"
	}

	ctx := buildTransformCtx(input.Inputs, input.StepOutputs)

	switch engine {
	case "jsonpath":
		return evalJSONPath(ctx, expr)
	case "jq":
		return evalJQ(ctx, expr)
	default:
		return nil, fmt.Errorf("transform: unsupported engine %q (supported: jsonpath, jq)", engine)
	}
}

// buildTransformCtx merges inputs and step outputs into a flat document.
// Keys from step outputs take precedence over inputs.
func buildTransformCtx(inputs, outputs map[string]any) map[string]any {
	ctx := make(map[string]any, len(inputs)+len(outputs)+1)
	for k, v := range inputs {
		ctx[k] = v
	}
	for k, v := range outputs {
		ctx[k] = v
	}


	stepsCtx := make(map[string]any, len(outputs))
	for k, v := range outputs {
		stepsCtx[k] = v
	}
	ctx["steps"] = stepsCtx
	return ctx
}



// evalJSONPath evaluates a simplified JSONPath expression.
// Supported: $.field, $.a.b, $.arr[n], $.arr[*], $.arr[?(@.f==v)].
func evalJSONPath(doc map[string]any, expr string) (any, error) {
	if !strings.HasPrefix(expr, "$") && !strings.HasPrefix(expr, "@") {
		return resolveSimplePath(doc, expr)
	}

	path := expr
	if strings.HasPrefix(path, "$.") {
		path = path[2:]
	} else if strings.HasPrefix(path, "$") {
		path = path[1:]
	} else if strings.HasPrefix(path, "@.") {
		path = path[2:]
	} else if strings.HasPrefix(path, "@") {
		path = path[1:]
	}

	if path == "" {
		return doc, nil
	}

	return resolveJSONPath(doc, path)
}

func resolveJSONPath(current any, path string) (any, error) {
	for path != "" {

		for len(path) > 0 && path[0] == '.' {
			path = path[1:]
		}
		if path == "" {
			return current, nil
		}


		if strings.HasPrefix(path, "[?(") {
			end := strings.Index(path, ")]")
			if end < 0 {
				return nil, fmt.Errorf("jsonpath: unmatched filter bracket")
			}
			filter := path[3:end]
			arr, ok := current.([]any)
			if !ok {
				return nil, fmt.Errorf("jsonpath: filter requires array, got %T", current)
			}
			condField, condVal, condOp := parseFilter(filter)
			var filtered []any
			for _, item := range arr {
				itemMap, ok := item.(map[string]any)
				if !ok {
					continue
				}
				actual := resolveJSONPathSimple(itemMap, condField)
				if compareValues(actual, condVal, condOp) {
					filtered = append(filtered, item)
				}
			}
			current = filtered
			path = path[end+2:]
			continue
		}


		if path[0] == '[' {
			end := strings.Index(path, "]")
			if end < 0 {
				return nil, fmt.Errorf("jsonpath: unmatched bracket")
			}
			idxStr := path[1:end]
			path = path[end+1:]

			if idxStr == "*" {
				arr, ok := current.([]any)
				if !ok {
					return nil, fmt.Errorf("jsonpath: wildcard requires array, got %T", current)
				}
				if path == "" {
					return arr, nil
				}

				rest := strings.TrimPrefix(path, ".")
				result := make([]any, 0, len(arr))
				for _, item := range arr {
					v, err := resolveJSONPath(item, rest)
					if err == nil {
						result = append(result, v)
					}
				}
				return result, nil
			}

			idx, err := strconv.Atoi(idxStr)
			if err != nil {
				return nil, fmt.Errorf("jsonpath: invalid index %q", idxStr)
			}
			arr, ok := current.([]any)
			if !ok {
				return nil, fmt.Errorf("jsonpath: index requires array, got %T", current)
			}
			if idx < 0 || idx >= len(arr) {
				return nil, fmt.Errorf("jsonpath: index %d out of bounds (len=%d)", idx, len(arr))
			}
			current = arr[idx]
			continue
		}


		end := len(path)
		for i := 0; i < len(path); i++ {
			if path[i] == '.' || path[i] == '[' {
				end = i
				break
			}
		}
		field := path[:end]
		path = path[end:]

		m, ok := current.(map[string]any)
		if !ok {
			return nil, fmt.Errorf("jsonpath: cannot access field %q on %T", field, current)
		}
		current = m[field]
	}
	return current, nil
}

func parseFilter(filter string) (field, val, op string) {
	op = "=="
	operators := []string{"!=", ">=", "<=", ">", "<", "=="}
	for _, o := range operators {
		idx := strings.Index(filter, o)
		if idx > 0 {
			op = o
			left := strings.TrimSpace(filter[:idx])
			right := strings.TrimSpace(filter[idx+len(o):])
			left = strings.TrimPrefix(left, "@.")
			left = strings.TrimPrefix(left, "@")
			return left, strings.Trim(right, "'\""), op
		}
	}

	return strings.TrimPrefix(filter, "@."), "true", "=="
}

func compareValues(actual any, expected string, op string) bool {
	actualStr := fmt.Sprint(actual)
	switch op {
	case "==":
		return actualStr == expected
	case "!=":
		return actualStr != expected
	case ">":
		av, aerr := strconv.ParseFloat(actualStr, 64)
		ev, eerr := strconv.ParseFloat(expected, 64)
		if aerr == nil && eerr == nil {
			return av > ev
		}
		return actualStr > expected
	case "<":
		av, aerr := strconv.ParseFloat(actualStr, 64)
		ev, eerr := strconv.ParseFloat(expected, 64)
		if aerr == nil && eerr == nil {
			return av < ev
		}
		return actualStr < expected
	case ">=":
		av, aerr := strconv.ParseFloat(actualStr, 64)
		ev, eerr := strconv.ParseFloat(expected, 64)
		if aerr == nil && eerr == nil {
			return av >= ev
		}
		return actualStr >= expected
	case "<=":
		av, aerr := strconv.ParseFloat(actualStr, 64)
		ev, eerr := strconv.ParseFloat(expected, 64)
		if aerr == nil && eerr == nil {
			return av <= ev
		}
		return actualStr <= expected
	}
	return false
}

func resolveJSONPathSimple(m map[string]any, path string) any {
	parts := strings.Split(path, ".")
	current := any(m)
	for _, p := range parts {
		switch v := current.(type) {
		case map[string]any:
			current = v[p]
		default:
			return nil
		}
	}
	return current
}

func resolveSimplePath(doc map[string]any, path string) (any, error) {
	parts := strings.Split(path, ".")
	current := any(doc)
	for _, p := range parts {
		m, ok := current.(map[string]any)
		if !ok {
			return nil, fmt.Errorf("transform: cannot resolve path %q at %q", path, p)
		}
		current = m[p]
	}
	return current, nil
}



func evalJQ(doc map[string]any, expr string) (any, error) {
	expr = strings.TrimSpace(expr)
	if expr == "." {
		return doc, nil
	}


	if strings.Contains(expr, " | ") {
		parts := strings.Split(expr, " | ")
		current := any(doc)
		for _, part := range parts {
			part = strings.TrimSpace(part)
			var err error
			current, err = evalJQStep(current, part)
			if err != nil {
				return nil, fmt.Errorf("jq pipe: %w", err)
			}
		}
		return current, nil
	}

	return evalJQStep(doc, expr)
}

func evalJQStep(current any, step string) (any, error) {
	step = strings.TrimSpace(step)


	if strings.HasPrefix(step, "map(") && strings.HasSuffix(step, ")") {
		inner := step[4 : len(step)-1]
		arr, ok := current.([]any)
		if !ok {
			return nil, fmt.Errorf("jq: map requires array, got %T", current)
		}
		result := make([]any, 0, len(arr))
		for _, item := range arr {
			v, err := evalJQStep(item, inner)
			if err != nil {
				return nil, err
			}
			if v != nil {
				result = append(result, v)
			}
		}
		return result, nil
	}


	if strings.HasPrefix(step, "select(") && strings.HasSuffix(step, ")") {
		inner := step[7 : len(step)-1]
		return evalJQSelect(current, inner)
	}


	step = strings.TrimPrefix(step, ".")
	if strings.HasSuffix(step, "[]") {
		field := strings.TrimSuffix(step, "[]")
		if field == "" {
			arr, ok := current.([]any)
			if !ok {
				return nil, fmt.Errorf("jq: [] requires array, got %T", current)
			}
			return arr, nil
		}
		m, ok := current.(map[string]any)
		if !ok {
			return nil, fmt.Errorf("jq: cannot access field %q on %T", field, current)
		}
		return m[field], nil
	}


	if step == "" {
		return current, nil
	}
	return resolveSimplePathAny(current, step)
}

func evalJQStepSimple(item any, expr string) (any, error) {
	expr = strings.TrimSpace(expr)
	expr = strings.TrimPrefix(expr, ".")
	if expr == "" || expr == "." {
		return item, nil
	}
	return resolveSimplePathAny(item, expr)
}

func evalJQSelect(current any, inner string) (any, error) {
	inner = strings.TrimSpace(inner)
	operators := []string{"!=", ">=", "<=", "==", ">", "<"}
	var foundOp string
	var opIdx int = -1
	for _, op := range operators {
		idx := strings.Index(inner, op)
		if idx > 0 {
			foundOp = op
			opIdx = idx
			break
		}
	}
	if opIdx < 0 {

		field := strings.TrimPrefix(inner, ".")
		return filterSingle(current, field, "true", "==")
	}

	left := strings.TrimSpace(inner[:opIdx])
	right := strings.TrimSpace(inner[opIdx+len(foundOp):])
	left = strings.TrimPrefix(left, ".")
	right = strings.Trim(right, "'\"")

	if arr, ok := current.([]any); ok {
		return filterArray(arr, left, right, foundOp)
	}
	return filterSingle(current, left, right, foundOp)
}

func filterArray(arr []any, field, expected, op string) ([]any, error) {
	var result []any
	for _, item := range arr {
		m, ok := item.(map[string]any)
		if !ok {
			continue
		}
		actual := resolveJSONPathSimple(m, field)
		if compareValues(actual, expected, op) {
			result = append(result, item)
		}
	}
	return result, nil
}

func filterSingle(current any, field, expected, op string) (any, error) {
	m, ok := current.(map[string]any)
	if !ok {
		return current, nil // pass through non-map values
	}
	actual := resolveJSONPathSimple(m, field)
	if compareValues(actual, expected, op) {
		return current, nil
	}
	return nil, nil // filtered out
}

func resolveSimplePathAny(current any, path string) (any, error) {
	if path == "" {
		return current, nil
	}
	parts := strings.Split(path, ".")
	v := current
	for _, p := range parts {
		if p == "" {
			continue
		}
		m, ok := v.(map[string]any)
		if !ok {
			return nil, fmt.Errorf("jq: cannot access field %q on %T", p, v)
		}
		v = m[p]
	}
	return v, nil
}

// parseNumber attempts to parse a number from string.
func parseNumber(s string) (float64, bool) {
	v, err := strconv.ParseFloat(s, 64)
	return v, err == nil
}
