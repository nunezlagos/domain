package openspec

// headings canónicos: render los emite y parse los busca. Cambiar un literal
// rompe el round-trip de los changes ya exportados, no tocar a la ligera.
const (
	hWhy      = "## Why"
	hScope    = "## Scope"
	hApproach = "## Approach"
	hRisks    = "## Risks"
	hTesting  = "## Testing"

	hDecisions      = "## Decisions"
	hAlternatives   = "## Alternatives"
	hDataFlow       = "## Data Flow"
	hTDDPlan        = "## TDD Plan"
	hRiskMitigation = "## Risk Mitigation"

	taskIDPrefix = "<!-- t:"
	taskIDSuffix = " -->"

	fileProposal = "proposal.md"
	fileDesign   = "design.md"
	fileTasks    = "tasks.md"
	fileMeta     = ".openspec.yaml"
)

func specFile(slug string) string { return "specs/" + slug + "/spec.md" }
