package openspec

import "strings"

const emptySentinel = "_(vacío)_"

// ParseProposal lee proposal.md de vuelta a sus campos por heading canónico.
func ParseProposal(md string) ProposalDoc {
	s := splitByH2(md)
	return ProposalDoc{
		Intention:    s[hWhy],
		Scope:        s[hScope],
		Approach:     s[hApproach],
		Risks:        s[hRisks],
		TestingNotes: s[hTesting],
	}
}

// ParseDesign lee design.md de vuelta a sus campos por heading canónico.
func ParseDesign(md string) DesignDoc {
	s := splitByH2(md)
	return DesignDoc{
		ArchDecisions:   s[hDecisions],
		Alternatives:    s[hAlternatives],
		DataFlow:        s[hDataFlow],
		TDDPlan:         s[hTDDPlan],
		RisksMitigation: s[hRiskMitigation],
	}
}

// ParseTasks reconstruye las tasks con su id (marcador) y estado del checkbox.
func ParseTasks(md string) []TaskDoc {
	var out []TaskDoc
	var section string
	for _, line := range strings.Split(md, "\n") {
		trimmed := strings.TrimSpace(line)
		if strings.HasPrefix(trimmed, "## ") {
			section = strings.TrimSpace(strings.TrimPrefix(trimmed, "## "))
			continue
		}
		t, ok := parseTaskLine(trimmed, section)
		if ok {
			out = append(out, t)
		}
	}
	return out
}

// ParseScenarios reconstruye los escenarios Gherkin de spec.md. El feature es
// el H1 del archivo y se propaga a cada escenario.
func ParseScenarios(md string) []ScenarioDoc {
	var out []ScenarioDoc
	var cur *ScenarioDoc
	var feature string
	flush := func() {
		if cur != nil {
			cur.Feature = feature
			out = append(out, *cur)
		}
	}
	for _, line := range strings.Split(md, "\n") {
		trimmed := strings.TrimSpace(line)
		switch {
		case strings.HasPrefix(trimmed, "# ") && !strings.HasPrefix(trimmed, "## "):
			feature = strings.TrimSpace(strings.TrimPrefix(trimmed, "# "))
		case scenarioHeading(trimmed) != "":
			flush()
			cur = &ScenarioDoc{Scenario: scenarioHeading(trimmed)}
		case cur == nil:
			continue
		default:
			if kw, val, ok := gwtLine(trimmed); ok {
				switch kw {
				case "Given":
					cur.Given = append(cur.Given, val)
				case "When":
					cur.When = val
				case "Then":
					cur.Then = append(cur.Then, val)
				}
			}
		}
	}
	flush()
	return out
}

// scenarioHeading reconoce el heading de un escenario en cualquiera de las
// variantes aceptadas (R3): "## Scenario:" (H2) o "#### Scenario:" (H4), y
// también "### Scenario:". Devuelve el nombre del escenario, o "" si la línea
// no es un heading de escenario.
func scenarioHeading(trimmed string) string {
	for _, prefix := range []string{"## Scenario:", "### Scenario:", "#### Scenario:"} {
		if strings.HasPrefix(trimmed, prefix) {
			return strings.TrimSpace(strings.TrimPrefix(trimmed, prefix))
		}
	}
	return ""
}

// gwtLine reconoce una línea Given/When/Then en cualquiera de las variantes
// aceptadas (R3): plano ("Given x"), con bullet ("- Given x") y con
// bullet+negrita ("- **Given** x"). Devuelve la keyword canónica, el valor y
// si hubo match.
func gwtLine(trimmed string) (keyword, value string, ok bool) {
	s := strings.TrimSpace(strings.TrimPrefix(trimmed, "-"))
	for _, kw := range []string{"Given", "When", "Then"} {
		// forma con negrita: **Given** valor
		if bold := "**" + kw + "**"; strings.HasPrefix(s, bold) {
			return kw, strings.TrimSpace(s[len(bold):]), true
		}
		// forma plana o con bullet ya despojado: Given valor
		if strings.HasPrefix(s, kw+" ") {
			return kw, strings.TrimSpace(s[len(kw):]), true
		}
	}
	return "", "", false
}

func parseTaskLine(trimmed, section string) (TaskDoc, bool) {
	var completed bool
	var rest string
	switch {
	case strings.HasPrefix(trimmed, "- [x] "), strings.HasPrefix(trimmed, "- [X] "):
		completed = true
		rest = trimmed[6:]
	case strings.HasPrefix(trimmed, "- [ ] "):
		rest = trimmed[6:]
	default:
		return TaskDoc{}, false
	}
	id := ""
	if i := strings.Index(rest, taskIDPrefix); i >= 0 {
		if j := strings.Index(rest[i:], taskIDSuffix); j >= 0 {
			id = strings.TrimSpace(rest[i+len(taskIDPrefix) : i+j])
			rest = strings.TrimSpace(rest[:i])
		}
	}
	return TaskDoc{ID: id, Section: section, Text: strings.TrimSpace(rest), Completed: completed}, true
}

// Meta es el subconjunto de .openspec.yaml que el round-trip necesita.
type Meta struct {
	IssueID   string
	IssueSlug string
	Status    string
	Hashes    map[string]string
}

// ParseMeta extrae los campos domain.* y el bloque hashes sin dependencia yaml.
func ParseMeta(y string) Meta {
	m := Meta{Hashes: map[string]string{}}
	inHashes := false
	for _, line := range strings.Split(y, "\n") {
		if strings.HasPrefix(line, "hashes:") {
			inHashes = true
			continue
		}
		if inHashes && strings.HasPrefix(line, "  ") {
			k, v := splitKV(line)
			if k != "" {
				m.Hashes[strings.Trim(k, `"`)] = v
			}
			continue
		}
		inHashes = false
		k, v := splitKV(line)
		switch k {
		case "issue_id":
			m.IssueID = v
		case "issue_slug":
			m.IssueSlug = v
		case "status":
			m.Status = v
		}
	}
	return m
}

func splitKV(line string) (string, string) {
	i := strings.Index(line, ":")
	if i < 0 {
		return "", ""
	}
	return strings.TrimSpace(line[:i]), strings.TrimSpace(line[i+1:])
}

// splitByH2 devuelve el cuerpo bajo cada heading "## ", con el sentinel de
// vacío revertido a cadena vacía.
func splitByH2(md string) map[string]string {
	out := map[string]string{}
	var heading string
	var body []string
	flush := func() {
		if heading != "" {
			out[heading] = unwrapEmpty(strings.TrimSpace(strings.Join(body, "\n")))
		}
	}
	for _, line := range strings.Split(md, "\n") {
		if strings.HasPrefix(line, "## ") {
			flush()
			heading = strings.TrimRight(line, " ")
			body = nil
			continue
		}
		body = append(body, line)
	}
	flush()
	return out
}

func unwrapEmpty(s string) string {
	if s == emptySentinel {
		return ""
	}
	return s
}
