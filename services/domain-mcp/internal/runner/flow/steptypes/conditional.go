package steptypes

import (
	"context"
	"fmt"
	"strings"
)

// ConditionalRunner evaluates an expression and executes the matching branch.
//
// Config:
//
//	{
//	  "condition": "steps.s1.result.status == 'approved'",
//	  "if_branch": [{"id": "s3a", "type": "skill_call", "params": {"skill_slug": "send-welcome"}}],
//	  "else_branch": [{"id": "s3b", "type": "human_input", "params": {"question": "Revisar manualmente"}}]
//	}
//
// Output: {"branch": "if"|"else", "condition_result": true|false, "condition": "<expr>"}.
type ConditionalRunner struct{}

func (r *ConditionalRunner) Run(_ context.Context, input RunInput) (any, error) {
	condition := configString(input.Config, "condition")
	if condition == "" {
		return nil, fmt.Errorf("conditional: condition required")
	}

	ifBranch := configSlice(input.Config, "if_branch")
	elseBranch := configSlice(input.Config, "else_branch")


	resolved := ResolveTemplate(condition, input.Inputs, input.StepOutputs)
	resolved = ResolveBarePaths(resolved, input.Inputs, input.StepOutputs)

	result, err := evalBool(resolved)
	if err != nil {
		return nil, fmt.Errorf("conditional: %w", err)
	}

	out := map[string]any{
		"branch":           "else",
		"condition_result": result,
		"condition":        condition,
	}
	if result {
		out["branch"] = "if"
	}
	_ = ifBranch // available for the flow runner to schedule
	_ = elseBranch

	return out, nil
}

// evalBool evaluates a simple boolean expression.
// Supports: ==, !=, true, false, numbers.
func evalBool(expr string) (bool, error) {

	i := 0
	for i < len(expr) && (expr[i] == ' ' || expr[i] == '\t') {
		i++
	}
	expr = expr[i:]


	if expr == "true" {
		return true, nil
	}
	if expr == "false" {
		return false, nil
	}



	ops := []string{"==", "!=", ">=", "<=", ">", "<"}
	var opIdx int = -1
	var opLen int
	var foundOp string
	for _, op := range ops {
		idx := findOp(expr, op)
		if idx >= 0 {
			opIdx = idx
			opLen = len(op)
			foundOp = op
			break
		}
	}
	if opIdx < 0 {
		return false, fmt.Errorf("cannot evaluate expression: %q", expr)
	}

	left := trimQuotes(strings.TrimSpace(expr[:opIdx]))
	right := trimQuotes(strings.TrimSpace(expr[opIdx+opLen:]))

	leftVal, leftIsNum := parseNumber(left)
	rightVal, rightIsNum := parseNumber(right)

	switch foundOp {
	case "==":
		if leftIsNum && rightIsNum {
			return leftVal == rightVal, nil
		}

		if (left == "true" || left == "false") && (right == "true" || right == "false") {
			return (left == "true") == (right == "true"), nil
		}
		return left == right, nil
	case "!=":
		if leftIsNum && rightIsNum {
			return leftVal != rightVal, nil
		}
		if (left == "true" || left == "false") && (right == "true" || right == "false") {
			return (left == "true") != (right == "true"), nil
		}
		return left != right, nil
	case ">":
		if leftIsNum && rightIsNum {
			return leftVal > rightVal, nil
		}
		return left > right, nil
	case "<":
		if leftIsNum && rightIsNum {
			return leftVal < rightVal, nil
		}
		return left < right, nil
	case ">=":
		if leftIsNum && rightIsNum {
			return leftVal >= rightVal, nil
		}
		return left >= right, nil
	case "<=":
		if leftIsNum && rightIsNum {
			return leftVal <= rightVal, nil
		}
		return left <= right, nil
	}
	return false, fmt.Errorf("unsupported operator: %s", foundOp)
}

// findOp finds the first occurrence of op in expr, outside of quotes.
func findOp(expr, op string) int {
	inSingle := false
	inDouble := false
	for i := 0; i < len(expr); i++ {
		ch := expr[i]
		if ch == '\'' && !inDouble {
			inSingle = !inSingle
			continue
		}
		if ch == '"' && !inSingle {
			inDouble = !inDouble
			continue
		}
		if !inSingle && !inDouble {
			if i+len(op) <= len(expr) && expr[i:i+len(op)] == op {


				if op == "=" || op == "!" || op == ">" || op == "<" {
					if i+len(op) < len(expr) && expr[i+len(op)] == '=' {
						continue
					}
				}
				return i
			}
		}
	}
	return -1
}

func trimQuotes(s string) string {
	if len(s) >= 2 {
		if (s[0] == '\'' && s[len(s)-1] == '\'') || (s[0] == '"' && s[len(s)-1] == '"') {
			return s[1 : len(s)-1]
		}
	}
	return s
}
