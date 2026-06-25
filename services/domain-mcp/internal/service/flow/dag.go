package flow

import (
	"fmt"
	"sort"
)

// ErrCycleDetected se retorna cuando el DAG tiene un ciclo.
// El ciclo listo se incluye en el mensaje de error.
var ErrCycleDetected = fmt.Errorf("DAG cycle detected")

// ValidateDAG verifica que los steps formen un DAG acíclico usando
// Kahn's algorithm. Valida también que depends_on referencie steps existentes.
// Retorna error si se detecta un ciclo o referencia inválida.
func ValidateDAG(steps []Step) error {
	stepIDs := map[string]bool{}
	for _, s := range steps {
		stepIDs[s.ID] = true
	}
	for _, s := range steps {
		for _, dep := range s.DependsOn {
			if !stepIDs[dep] {
				return fmt.Errorf("%w: step '%s' depends_on unknown step '%s'", ErrSpecInvalid, s.ID, dep)
			}
		}
	}

	graph := buildGraph(steps)
	_, err := kahnSort(graph)
	if err != nil {
		return fmt.Errorf("%w: %v", ErrCycleDetected, err)
	}
	return nil
}

// TopologicalSort ordena los steps según sus dependencias (depends_on).
// Steps sin dependencias mantienen su orden relativo original.
// Retorna error si hay ciclo.
func TopologicalSort(steps []Step) ([]Step, error) {
	graph := buildGraph(steps)
	order, err := kahnSort(graph)
	if err != nil {
		return nil, err
	}
	stepByID := map[string]Step{}
	for _, s := range steps {
		stepByID[s.ID] = s
	}
	out := make([]Step, 0, len(order))
	for _, id := range order {
		out = append(out, stepByID[id])
	}
	return out, nil
}

type node struct {
	id       string
	indegree int
	children []string
}

func buildGraph(steps []Step) map[string]*node {
	nodes := map[string]*node{}
	for _, s := range steps {
		nodes[s.ID] = &node{id: s.ID}
	}
	for _, s := range steps {
		for _, dep := range s.DependsOn {
			nodes[dep].children = append(nodes[dep].children, s.ID)
			nodes[s.ID].indegree++
		}
	}
	return nodes
}

// kahnSort ejecuta Kahn's algorithm.
// Retorna el orden topológico o error con el ciclo detectado.
func kahnSort(nodes map[string]*node) ([]string, error) {
	queue := []string{}
	for _, n := range nodes {
		if n.indegree == 0 {
			queue = append(queue, n.id)
		}
	}

	sort.Strings(queue)
	sorted := []string{}
	visited := map[string]bool{}

	for len(queue) > 0 {
		id := queue[0]
		queue = queue[1:]
		if visited[id] {
			continue
		}
		visited[id] = true
		sorted = append(sorted, id)
		n := nodes[id]
		children := make([]string, len(n.children))
		copy(children, n.children)
		sort.Strings(children)
		for _, child := range children {
			nodes[child].indegree--
			if nodes[child].indegree == 0 {
				queue = append(queue, child)
			}
		}
		sort.Strings(queue)
	}

	if len(sorted) != len(nodes) {
		cycle := detectCycle(nodes)
		return nil, fmt.Errorf("cycle detected: %v", cycle)
	}
	return sorted, nil
}

// detectCycle encuentra un ciclo en el grafo usando DFS.
func detectCycle(nodes map[string]*node) []string {
	WHITE, GRAY, BLACK := 0, 1, 2
	color := map[string]int{}
	for id := range nodes {
		color[id] = WHITE
	}
	cycle := []string{}

	var dfs func(id string) bool
	dfs = func(id string) bool {
		color[id] = GRAY
		cycle = append(cycle, id)
		for _, child := range nodes[id].children {
			if color[child] == GRAY {
				cycle = append(cycle, child)
				return true
			}
			if color[child] == WHITE {
				if dfs(child) {
					return true
				}
			}
		}
		color[id] = BLACK
		cycle = cycle[:len(cycle)-1]
		return false
	}

	for id := range nodes {
		if color[id] == WHITE {
			cycle = []string{}
			if dfs(id) {

				last := cycle[len(cycle)-1]
				for i, c := range cycle {
					if c == last && i < len(cycle)-1 {
						return cycle[i:]
					}
				}
				return cycle
			}
		}
	}
	return nil
}
