package openspec

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"strings"
)

// Render proyecta un change al árbol de archivos openspec. archived decide si
// el change vive bajo changes/ o changes/archive/<fecha>-<slug>/; la fecha la
// pasa el caller (este paquete no lee el reloj para mantenerse puro).
func Render(c *Change, archived bool, archiveDate string) *Rendered {
	dir := "openspec/changes/" + c.IssueSlug + "/"
	if archived {
		dir = "openspec/changes/archive/" + archiveDate + "-" + c.IssueSlug + "/"
	}
	files := map[string]string{
		fileProposal:          renderProposal(c),
		fileDesign:            renderDesign(c),
		fileTasks:             renderTasks(c),
		specFile(c.IssueSlug): renderSpec(c),
	}
	hashes := make(map[string]string, len(files))
	for name, body := range files {
		hashes[name] = ContentHash(body)
	}
	files[fileMeta] = renderMeta(c, hashes)
	return &Rendered{Dir: dir, Files: files, Hashes: hashes}
}

// ContentHash es el sha256 hex del cuerpo, usado para detectar drift repo↔DB.
func ContentHash(s string) string {
	sum := sha256.Sum256([]byte(s))
	return hex.EncodeToString(sum[:])
}

func renderProposal(c *Change) string {
	p := c.Proposal
	if p == nil {
		p = &ProposalDoc{}
	}
	var b strings.Builder
	fmt.Fprintf(&b, "# %s\n\n", c.Title)
	writeSection(&b, hWhy, p.Intention)
	writeSection(&b, hScope, p.Scope)
	writeSection(&b, hApproach, p.Approach)
	writeSection(&b, hRisks, p.Risks)
	writeSection(&b, hTesting, p.TestingNotes)
	return b.String()
}

func renderDesign(c *Change) string {
	d := c.Design
	if d == nil {
		d = &DesignDoc{}
	}
	var b strings.Builder
	fmt.Fprintf(&b, "# %s — Design\n\n", c.Title)
	writeSection(&b, hDecisions, d.ArchDecisions)
	writeSection(&b, hAlternatives, d.Alternatives)
	writeSection(&b, hDataFlow, d.DataFlow)
	writeSection(&b, hTDDPlan, d.TDDPlan)
	writeSection(&b, hRiskMitigation, d.RisksMitigation)
	return b.String()
}

func renderTasks(c *Change) string {
	var b strings.Builder
	fmt.Fprintf(&b, "# %s — Tasks\n\n", c.Title)
	var section string
	for _, t := range c.Tasks {
		if t.Section != section {
			section = t.Section
			fmt.Fprintf(&b, "## %s\n\n", section)
		}
		box := " "
		if t.Completed {
			box = "x"
		}
		fmt.Fprintf(&b, "- [%s] %s %s%s%s\n", box, t.Text, taskIDPrefix, t.ID, taskIDSuffix)
	}
	return b.String()
}

func renderSpec(c *Change) string {
	var b strings.Builder
	feature := c.Title
	if len(c.Scenarios) > 0 && c.Scenarios[0].Feature != "" {
		feature = c.Scenarios[0].Feature
	}
	fmt.Fprintf(&b, "# %s\n\n", feature)
	for _, sc := range c.Scenarios {
		fmt.Fprintf(&b, "## Scenario: %s\n\n", sc.Scenario)
		for _, g := range sc.Given {
			fmt.Fprintf(&b, "- Given %s\n", g)
		}
		if sc.When != "" {
			fmt.Fprintf(&b, "- When %s\n", sc.When)
		}
		for _, t := range sc.Then {
			fmt.Fprintf(&b, "- Then %s\n", t)
		}
		b.WriteString("\n")
	}
	return b.String()
}

func renderMeta(c *Change, hashes map[string]string) string {
	var b strings.Builder
	b.WriteString("schema: spec-driven\n")
	b.WriteString("domain:\n")
	fmt.Fprintf(&b, "  issue_id: %s\n", c.IssueID)
	fmt.Fprintf(&b, "  issue_slug: %s\n", c.IssueSlug)
	fmt.Fprintf(&b, "  req: %s\n", c.ReqSlug)
	fmt.Fprintf(&b, "  status: %s\n", c.Status)
	fmt.Fprintf(&b, "  priority: %s\n", c.Priority)
	fmt.Fprintf(&b, "  proposal_version: %d\n", c.ProposalVersion)
	fmt.Fprintf(&b, "  design_version: %d\n", c.DesignVersion)
	b.WriteString("hashes:\n")
	for _, name := range []string{fileProposal, fileDesign, fileTasks, specFile(c.IssueSlug)} {
		fmt.Fprintf(&b, "  %q: %s\n", name, hashes[name])
	}
	return b.String()
}

func writeSection(b *strings.Builder, heading, body string) {
	body = strings.TrimSpace(body)
	if body == "" {
		body = "_(vacío)_"
	}
	fmt.Fprintf(b, "%s\n\n%s\n\n", heading, body)
}
