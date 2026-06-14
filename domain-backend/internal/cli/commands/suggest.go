// issue-14.3 — sugerencias "¿quisiste decir?" por distancia de Levenshtein
// para comandos y flags desconocidos.
package commands

import "strings"

// levenshtein calcula la distancia de edición entre dos strings.
func levenshtein(a, b string) int {
	la, lb := len(a), len(b)
	if la == 0 {
		return lb
	}
	if lb == 0 {
		return la
	}
	prev := make([]int, lb+1)
	cur := make([]int, lb+1)
	for j := 0; j <= lb; j++ {
		prev[j] = j
	}
	for i := 1; i <= la; i++ {
		cur[0] = i
		for j := 1; j <= lb; j++ {
			cost := 1
			if a[i-1] == b[j-1] {
				cost = 0
			}
			cur[j] = minInt(prev[j]+1, cur[j-1]+1, prev[j-1]+cost)
		}
		prev, cur = cur, prev
	}
	return prev[lb]
}

func minInt(vals ...int) int {
	m := vals[0]
	for _, v := range vals[1:] {
		if v < m {
			m = v
		}
	}
	return m
}

// suggest retorna el candidato más cercano si la distancia es razonable
// (≤2, o ≤3 para inputs largos). "" si nada está suficientemente cerca.
func suggest(input string, candidates []string) string {
	input = strings.ToLower(input)
	best := ""
	bestDist := 1 << 30
	for _, c := range candidates {
		d := levenshtein(input, strings.ToLower(c))
		if d < bestDist {
			bestDist = d
			best = c
		}
	}
	maxDist := 2
	if len(input) > 6 {
		maxDist = 3
	}
	if bestDist > maxDist {
		return ""
	}
	return best
}

// knownCommands para sugerencias del dispatcher.
var knownCommands = []string{
	"projects", "observations", "obs", "agents", "flows", "skills",
	"search", "context", "completion", "policies", "config", "man", "help",
}
