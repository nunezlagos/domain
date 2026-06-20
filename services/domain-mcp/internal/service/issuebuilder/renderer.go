package issuebuilder

import (
	"fmt"
	"strings"
)

// renderFeaturePreview renderiza los archivos SDD desde answers de mode=feature.
// Output mínimo viable: hu.md + state.yaml. proposal/design/tasks quedan como
// stub para que el humano + agente IA los expandan en paso siguiente.
func renderFeaturePreview(d *Draft, answers map[string]any) (*Preview, error) {
	slug, _ := answers["slug"].(string)
	reqParent, _ := answers["req_parent"].(string)
	audience, _ := answers["audience"].(string)
	effort, _ := answers["effort"].(string)
	priority, _ := answers["priority"].(string)
	changeType, _ := answers["change_type"].(string)
	goal, _ := answers["goal"].(string)
	summary, _ := answers["summary"].(string)

	if slug == "" || reqParent == "" {
		return nil, fmt.Errorf("missing required answers: slug, req_parent")
	}

	huNumber := "{auto-incremented}"
	suggested := fmt.Sprintf("HU-%s-%s", huNumber, slug)
	targetPath := fmt.Sprintf("openspec/changes/%s/%s/", reqParent, suggested)

	files := map[string]string{
		"issue.md":       renderHUMd(suggested, reqParent, priority, changeType, audience, goal, d.InitialIdea, summary),
		"proposal.md": renderProposalMd(suggested, summary, reqParent, effort),
		"design.md":   renderDesignMd(suggested, summary),
		"tasks.md":    renderTasksMd(suggested),
		"state.yaml":  renderStateYaml(),
	}

	return &Preview{
		Files:         files,
		TargetPath:    targetPath,
		SuggestedSlug: suggested,
	}, nil
}

func renderHUMd(slug, req, prio, changeType, audience, goal, idea, summary string) string {
	var b strings.Builder
	fmt.Fprintf(&b, "# %s\n\n", slug)
	fmt.Fprintf(&b, "**Origen:** `%s`\n", req)
	fmt.Fprintf(&b, "**Prioridad tentativa:** %s\n", prio)
	fmt.Fprintf(&b, "**Tipo:** %s\n", changeType)
	fmt.Fprintf(&b, "**Audiencia:** %s\n\n", audience)
	fmt.Fprintf(&b, "## Historia de usuario\n\n")
	fmt.Fprintf(&b, "**Como** %s\n", audience)
	fmt.Fprintf(&b, "**Quiero** %s\n", idea)
	fmt.Fprintf(&b, "**Para** %s\n\n", goal)
	fmt.Fprintf(&b, "## Resumen\n\n%s\n\n", summary)
	fmt.Fprintf(&b, "## Criterios de aceptación\n\n")
	fmt.Fprintf(&b, "<!-- TODO: agregar 3-5 escenarios Gherkin -->\n\n")
	fmt.Fprintf(&b, "```gherkin\nFeature: %s\n\n  Scenario: TODO\n    Dado que TODO\n    Cuando TODO\n    Entonces TODO\n```\n", slug)
	return b.String()
}

func renderProposalMd(slug, summary, req, effort string) string {
	return fmt.Sprintf(`# Proposal: %s

**REQ padre:** %s
**Esfuerzo estimado:** %s

## Intention
%s

## Scope
<!-- qué entra / qué queda fuera -->

## Approach
<!-- enfoque técnico de alto nivel -->

## Risks
<!-- riesgos identificados -->
`, slug, req, effort, summary)
}

func renderDesignMd(slug, summary string) string {
	return fmt.Sprintf(`# Design: %s

## Decisión
<!-- ADR conciso -->

## Alternativas
<!-- al menos 2 alternativas evaluadas -->

## Data flow
<!-- diagrama o descripción del flujo -->

## TDD plan
<!-- red → green → refactor → sabotaje -->

## Resumen contexto
%s
`, slug, summary)
}

func renderTasksMd(slug string) string {
	return fmt.Sprintf(`# Tasks: %s

## Implementación

- [ ] TODO: definir tasks atómicas

## Tests

- [ ] Test happy path
- [ ] Test edge cases
- [ ] Test sabotaje (romper invariante → verificar detección)

## Documentación

- [ ] Actualizar CHANGELOG Unreleased
- [ ] Actualizar state.yaml a implemented
`, slug)
}

func renderStateYaml() string {
	return "status: proposed\ncreated: 2026-06-09\narchived: ~\n"
}
