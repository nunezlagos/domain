// Package openspec materializa documentos SDD de la DB como un árbol de
// archivos en el layout openspec oficial (change-céntrico) y los parsea de
// vuelta. Render y parse son funciones puras sin dependencia de DB: el
// transporte hacia/desde el repo lo hace el cliente MCP, no este paquete.
package openspec

// Change es la unidad openspec: un issue de la DB proyectado a un cambio.
type Change struct {
	IssueID   string
	IssueSlug string
	Title     string
	ReqSlug   string
	Status    string
	Priority  string

	Proposal  *ProposalDoc
	Design    *DesignDoc
	Tasks     []TaskDoc
	Scenarios []ScenarioDoc

	ProposalVersion int
	DesignVersion   int
}

// ProposalDoc son los campos de sdd_proposals que viajan a proposal.md.
type ProposalDoc struct {
	Intention    string
	Scope        string
	Approach     string
	Risks        string
	TestingNotes string
}

// DesignDoc son los campos de sdd_designs que viajan a design.md.
type DesignDoc struct {
	ArchDecisions   string
	Alternatives    string
	DataFlow        string
	TDDPlan         string
	RisksMitigation string
}

// TaskDoc es una fila de issue_tasks proyectada a una línea de tasks.md. El
// ID viaja como marcador HTML invisible para reconciliar status al re-importar.
type TaskDoc struct {
	ID        string
	Section   string
	Text      string
	Completed bool
}

// ScenarioDoc es un escenario Gherkin de issue_gherkin_scenarios.
type ScenarioDoc struct {
	Feature  string
	Scenario string
	Given    []string
	When     string
	Then     []string
}

// Rendered es el árbol de archivos de un change, con paths relativos al repo.
type Rendered struct {
	Dir    string
	Files  map[string]string
	Hashes map[string]string
}
