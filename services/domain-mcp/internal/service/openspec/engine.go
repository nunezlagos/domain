package openspec

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"

	issuesvc "nunezlagos/domain/internal/service/issue"
	specsvc "nunezlagos/domain/internal/service/spec"
	tasksvc "nunezlagos/domain/internal/service/task"
	"nunezlagos/domain/internal/store/txctx"
)

// ErrEmptyDoc se devuelve cuando un .md llega vacío o borrado: versionar vacío
// pisaría la DB con contenido nulo, así que se rechaza antes de persistir.
var ErrEmptyDoc = errors.New("archivo vacío o borrado: no se versiona contenido vacío (restaurá el archivo o quitalo del apply)")

// Las interfaces son exactamente la superficie que el Engine necesita de los
// servicios de issue/spec/task. Se declaran acá (no en mcp/server) para que el
// Engine sea la ÚNICA fuente de la lógica de negocio openspec, reusable tanto
// por el handler MCP como por el HTTP REST.

type SpecReader interface {
	GetLatestProposal(ctx context.Context, issueID uuid.UUID) (*specsvc.Proposal, error)
	GetLatestDesign(ctx context.Context, issueID uuid.UUID) (*specsvc.Design, error)
}

type SpecWriter interface {
	CreateProposal(ctx context.Context, issueID uuid.UUID, intention, scope, approach, risks, testingNotes string) (*specsvc.Proposal, error)
	CreateDesign(ctx context.Context, issueID uuid.UUID, proposalID *uuid.UUID, archDecisions, alternatives, dataFlow, tddPlan, risksMitigation string) (*specsvc.Design, error)
}

type TaskReader interface {
	ListTasks(ctx context.Context, issueID uuid.UUID) ([]tasksvc.Task, error)
}

type TaskWriter interface {
	UpdateTaskStatus(ctx context.Context, taskID uuid.UUID, newStatus, completedBy string) (*tasksvc.Task, error)
}

type IssueReader interface {
	GetByID(ctx context.Context, id uuid.UUID) (*issuesvc.Issue, error)
}

type IssueWriter interface {
	Update(ctx context.Context, slug string, title, description, status, priority *string) (*issuesvc.Issue, error)
	AddScenario(ctx context.Context, huSlug string, sc issuesvc.Scenario) (*issuesvc.Scenario, error)
	RemoveScenario(ctx context.Context, scenarioID uuid.UUID) error
}

// querier es la superficie común de *pgxpool.Pool y pgx.Tx que el Engine usa
// para las queries crudas (issues, escenarios).
type querier interface {
	QueryRow(ctx context.Context, sql string, args ...any) pgx.Row
	Query(ctx context.Context, sql string, args ...any) (pgx.Rows, error)
	Exec(ctx context.Context, sql string, args ...any) (pgconn.CommandTag, error)
}

// Engine concentra la lógica de negocio openspec (export/status/apply) que
// antes vivía acoplada en internal/mcp/server/openspec_*.go. Recibe readers/
// writers + pool y NO depende de mcp ni de http: ambos transportes lo invocan.
type Engine struct {
	IssuesR IssueReader
	IssuesW IssueWriter
	SpecR   SpecReader
	SpecW   SpecWriter
	TasksR  TaskReader
	TasksW  TaskWriter
	Pool    *pgxpool.Pool
}

func (e *Engine) q(ctx context.Context) querier {
	if tx := txctx.TxFromContext(ctx); tx != nil {
		return tx
	}
	return e.Pool
}

// File es un archivo del árbol openspec ({path, content}) tal cual llega del
// repo (status/apply) o se devuelve al repo (export).
type File struct {
	Path    string `json:"path"`
	Content string `json:"content"`
}

// IssueRow es una fila de issues para export, con metadata de render.
type IssueRow struct {
	ID          uuid.UUID
	Slug        string
	Title       string
	Status      string
	Priority    string
	ReqSlug     string
	UpdatedDate string
}

// ───────────────────────── EXPORT ─────────────────────────

// QueryIssues lista los issues del proyecto, filtrando por scope ("all" trae
// todos; cualquier otro valor trae solo proposed+active).
func (e *Engine) QueryIssues(ctx context.Context, projectID uuid.UUID, scope string) ([]IssueRow, error) {
	sql := `SELECT i.id, i.slug, i.title, i.status, i.priority, COALESCE(r.slug,''),
	               to_char(i.updated_at, 'YYYY-MM-DD')
	          FROM issues i
	          LEFT JOIN sdd_requirements r ON r.id = i.req_id
	         WHERE i.project_id = $1`
	args := []any{projectID}
	if scope != "all" {
		sql += ` AND i.status = ANY($2::text[])`
		args = append(args, []string{"proposed", "active"})
	}
	sql += ` ORDER BY i.slug`
	rows, err := e.q(ctx).Query(ctx, sql, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []IssueRow
	for rows.Next() {
		var r IssueRow
		if err := rows.Scan(&r.ID, &r.Slug, &r.Title, &r.Status, &r.Priority, &r.ReqSlug, &r.UpdatedDate); err != nil {
			return nil, err
		}
		out = append(out, r)
	}
	return out, rows.Err()
}

// RenderedChange es un change exportado: dir + paths absolutos al repo.
type RenderedChange struct {
	IssueSlug string            `json:"issue_slug"`
	Dir       string            `json:"dir"`
	Status    string            `json:"status"`
	Files     map[string]string `json:"files"`
}

// RenderIssue proyecta un issue a su árbol de archivos openspec (paths
// relativos al repo, ya prefijados con el dir del change).
func (e *Engine) RenderIssue(ctx context.Context, row IssueRow) (*RenderedChange, error) {
	change := &Change{
		IssueID: row.ID.String(), IssueSlug: row.Slug, Title: row.Title,
		ReqSlug: row.ReqSlug, Status: row.Status, Priority: row.Priority,
	}
	change.Proposal, change.ProposalVersion = e.loadProposalDoc(ctx, row.ID)
	change.Design, change.DesignVersion = e.loadDesignDoc(ctx, row.ID)
	change.Tasks = e.loadTaskDocs(ctx, row.ID)
	scenarios, err := e.loadScenarioDocs(ctx, row.ID)
	if err != nil {
		return nil, err
	}
	change.Scenarios = scenarios
	archived := row.Status == "implemented" || row.Status == "archived"
	rendered := Render(change, archived, row.UpdatedDate)
	files := make(map[string]string, len(rendered.Files))
	for name, body := range rendered.Files {
		files[rendered.Dir+name] = body
	}
	return &RenderedChange{
		IssueSlug: row.Slug, Dir: rendered.Dir,
		Status: row.Status, Files: files,
	}, nil
}

// Export renderiza todos los issues del proyecto (según scope) como changes.
func (e *Engine) Export(ctx context.Context, projectID uuid.UUID, scope string) ([]*RenderedChange, error) {
	rows, err := e.QueryIssues(ctx, projectID, scope)
	if err != nil {
		return nil, fmt.Errorf("query issues: %w", err)
	}
	out := make([]*RenderedChange, 0, len(rows))
	for _, row := range rows {
		rc, err := e.RenderIssue(ctx, row)
		if err != nil {
			return nil, fmt.Errorf("render issue %s: %w", row.Slug, err)
		}
		out = append(out, rc)
	}
	return out, nil
}

// renderFromDB reconstruye el árbol de un issue desde la DB para comparar
// hashes (sin reqSlug ni fecha: no entran en los hashes trackeados).
func (e *Engine) renderFromDB(ctx context.Context, issueID uuid.UUID, slug string) (*Rendered, error) {
	iss, err := e.IssuesR.GetByID(ctx, issueID)
	if err != nil || iss == nil {
		return nil, fmt.Errorf("issue no encontrado en DB")
	}
	change := &Change{
		IssueID: issueID.String(), IssueSlug: slug, Title: iss.Title,
		Status: iss.Status, Priority: iss.Priority,
	}
	change.Proposal, change.ProposalVersion = e.loadProposalDoc(ctx, issueID)
	change.Design, change.DesignVersion = e.loadDesignDoc(ctx, issueID)
	change.Tasks = e.loadTaskDocs(ctx, issueID)
	scenarios, err := e.loadScenarioDocs(ctx, issueID)
	if err != nil {
		return nil, err
	}
	change.Scenarios = scenarios
	return Render(change, false, ""), nil
}

func (e *Engine) loadProposalDoc(ctx context.Context, issueID uuid.UUID) (*ProposalDoc, int) {
	p, err := e.SpecR.GetLatestProposal(ctx, issueID)
	if err != nil || p == nil {
		return nil, 0
	}
	return &ProposalDoc{
		Intention: p.Intention, Scope: p.Scope, Approach: p.Approach,
		Risks: ptrStr(p.Risks), TestingNotes: ptrStr(p.TestingNotes),
	}, p.Version
}

func (e *Engine) loadDesignDoc(ctx context.Context, issueID uuid.UUID) (*DesignDoc, int) {
	d, err := e.SpecR.GetLatestDesign(ctx, issueID)
	if err != nil || d == nil {
		return nil, 0
	}
	return &DesignDoc{
		ArchDecisions: d.ArchDecisions, Alternatives: ptrStr(d.Alternatives),
		DataFlow: ptrStr(d.DataFlow), TDDPlan: ptrStr(d.TDDPlan),
		RisksMitigation: ptrStr(d.RisksMitigation),
	}, d.Version
}

func (e *Engine) loadTaskDocs(ctx context.Context, issueID uuid.UUID) []TaskDoc {
	tasks, err := e.TasksR.ListTasks(ctx, issueID)
	if err != nil {
		return nil
	}
	out := make([]TaskDoc, 0, len(tasks))
	for _, t := range tasks {
		out = append(out, TaskDoc{
			ID: t.ID.String(), Section: t.Section, Text: t.Description,
			Completed: t.Status == "completed",
		})
	}
	return out
}

func (e *Engine) loadScenarioDocs(ctx context.Context, issueID uuid.UUID) ([]ScenarioDoc, error) {
	rows, err := e.q(ctx).Query(ctx,
		`SELECT feature, scenario, given, when_text, then_rows
		   FROM issue_gherkin_scenarios
		  WHERE issue_id = $1 ORDER BY position`, issueID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []ScenarioDoc
	for rows.Next() {
		var sc ScenarioDoc
		if err := rows.Scan(&sc.Feature, &sc.Scenario, &sc.Given, &sc.When, &sc.Then); err != nil {
			return nil, err
		}
		out = append(out, sc)
	}
	return out, rows.Err()
}

// ───────────────────────── STATUS ─────────────────────────

// StatusResult es el veredicto de drift de un change.
type StatusResult struct {
	Dir       string            `json:"dir"`
	IssueSlug string            `json:"issue_slug,omitempty"`
	Verdict   string            `json:"verdict"`
	Reason    string            `json:"reason,omitempty"`
	Files     map[string]string `json:"files,omitempty"`
}

// Status audita el drift repo↔DB de todos los changes presentes en files.
func (e *Engine) Status(ctx context.Context, files []File) []StatusResult {
	byDir := GroupByChange(files)
	out := make([]StatusResult, 0, len(byDir))
	for dir, fs := range byDir {
		out = append(out, e.statusForChange(ctx, dir, fs))
	}
	return out
}

func (e *Engine) statusForChange(ctx context.Context, dir string, files map[string]string) StatusResult {
	metaRaw, ok := files[".openspec.yaml"]
	if !ok {
		return StatusResult{Dir: dir, Verdict: "error", Reason: "falta .openspec.yaml"}
	}
	m := ParseMeta(metaRaw)
	issueID, err := uuid.Parse(m.IssueID)
	if err != nil {
		return StatusResult{Dir: dir, Verdict: "error", Reason: "issue_id inválido en meta"}
	}
	dbRendered, derr := e.renderFromDB(ctx, issueID, m.IssueSlug)
	if derr != nil {
		return StatusResult{Dir: dir, Verdict: "db_missing", Reason: derr.Error()}
	}
	perFile := map[string]string{}
	overall := "clean"
	for _, name := range trackedFiles(m.IssueSlug) {
		v := classifyDrift(m.Hashes[name], ContentHash(files[name]), dbRendered.Hashes[name])
		perFile[name] = v
		overall = escalate(overall, v)
	}
	return StatusResult{Dir: dir, IssueSlug: m.IssueSlug, Verdict: overall, Files: perFile}
}

func classifyDrift(stored, repoNow, dbNow string) string {
	repoChanged := repoNow != stored
	dbChanged := dbNow != stored
	switch {
	case repoChanged && dbChanged:
		return "conflict"
	case repoChanged:
		return "repo_modified"
	case dbChanged:
		return "db_modified"
	default:
		return "clean"
	}
}

func escalate(overall, v string) string {
	rank := map[string]int{"clean": 0, "repo_modified": 1, "db_modified": 2, "conflict": 3}
	if rank[v] > rank[overall] {
		return v
	}
	return overall
}

func trackedFiles(slug string) []string {
	return []string{"proposal.md", "design.md", "tasks.md", "specs/" + slug + "/spec.md"}
}

// ───────────────────────── APPLY ─────────────────────────

// ApplyResult es el resultado de persistir un change.
type ApplyResult struct {
	Dir       string   `json:"dir"`
	IssueSlug string   `json:"issue_slug,omitempty"`
	Error     string   `json:"error,omitempty"`
	Applied   []string `json:"applied,omitempty"`
	Conflicts []string `json:"conflicts,omitempty"`

	// NotSent: archivos del change que no vinieron en el array de entrada (R7).
	// NO son conflictos — simplemente se omitieron del apply. Antes caían en
	// Conflicts con "archivo vacío o borrado", confundiendo al usuario.
	NotSent []string `json:"not_sent,omitempty"`

	// UnknownIssue: true cuando el .openspec.yaml apunta a un issue que no está
	// en BD (R7). El Error incluye un hint accionable en ese caso.
	UnknownIssue bool `json:"unknown_issue,omitempty"`

	// IgnoredTasks: cantidad de tasks en tasks.md sin marcador <!-- t:uuid -->
	// que se ignoraron por no tener round-trip (R2). Antes se descartaban en
	// silencio y tasks.md se reportaba como "applied" sin distinción.
	IgnoredTasks int `json:"ignored_tasks,omitempty"`
}

// Apply persiste en la DB los .md editados de todos los changes en files.
// completedBy es el actor (user id o sentinel) que marca tasks completadas.
func (e *Engine) Apply(ctx context.Context, files []File, force bool, completedBy string) []ApplyResult {
	byDir := GroupByChange(files)
	out := make([]ApplyResult, 0, len(byDir))
	for dir, fs := range byDir {
		out = append(out, e.applyChange(ctx, dir, fs, force, completedBy))
	}
	return out
}

func (e *Engine) applyChange(ctx context.Context, dir string, files map[string]string, force bool, completedBy string) ApplyResult {
	m := ParseMeta(files[".openspec.yaml"])
	issueID, err := uuid.Parse(m.IssueID)
	if err != nil {
		return ApplyResult{Dir: dir, UnknownIssue: true,
			Error: "issue_id inválido o falta .openspec.yaml: revisá que el change tenga .openspec.yaml con domain.issue_id, o corré domain_openspec_export para regenerarlo"}
	}
	iss, err := e.IssuesR.GetByID(ctx, issueID)
	if err != nil || iss == nil {
		return ApplyResult{Dir: dir, UnknownIssue: true,
			Error: "issue no está en BD: creá el issue con domain_issue_create_* o corré domain_openspec_export antes de aplicar"}
	}
	dbRendered, err := e.renderFromDB(ctx, issueID, m.IssueSlug)
	if err != nil {
		return ApplyResult{Dir: dir, Error: err.Error()}
	}
	res := e.applyFiles(ctx, applyCtx{
		issueID: issueID, iss: iss, slug: m.IssueSlug,
		meta: m, files: files, db: dbRendered, force: force,
		completedBy: completedBy,
	})
	if st := e.applyStatus(ctx, iss, m.Status); st != "" {
		res.applied = append(res.applied, st)
	}
	return ApplyResult{
		Dir: dir, IssueSlug: m.IssueSlug,
		Applied: res.applied, Conflicts: res.conflicts,
		NotSent: res.notSent, IgnoredTasks: res.ignoredTasks,
	}
}

type applyCtx struct {
	issueID     uuid.UUID
	iss         *issuesvc.Issue
	slug        string
	meta        Meta
	files       map[string]string
	db          *Rendered
	force       bool
	completedBy string
}

// applyFilesResult agrupa la clasificación por archivo del apply (R7) más el
// conteo de tasks ignoradas (R2).
type applyFilesResult struct {
	applied      []string
	conflicts    []string
	notSent      []string
	ignoredTasks int
}

func (e *Engine) applyFiles(ctx context.Context, a applyCtx) applyFilesResult {
	var res applyFilesResult
	specName := "specs/" + a.slug + "/spec.md"
	steps := []struct {
		name string
		fn   func() error
	}{
		{"proposal.md", func() error { return e.applyProposal(ctx, a.issueID, a.files["proposal.md"]) }},
		{"design.md", func() error { return e.applyDesign(ctx, a.issueID, a.files["design.md"]) }},
		{specName, func() error { return e.applyScenarios(ctx, a.iss, a.files[specName]) }},
		{"tasks.md", func() error {
			_, err := e.applyTasks(ctx, a.issueID, a.files["tasks.md"], a.completedBy)
			return err
		}},
	}
	for _, s := range steps {
		// R7: un archivo ausente del array de entrada es not_sent, no conflict.
		if _, present := a.files[s.name]; !present {
			res.notSent = append(res.notSent, s.name)
			continue
		}
		// R2: el conteo de tasks sin marcador se calcula siempre que tasks.md
		// venga presente, independiente de si decideFile lo aplica o lo saltea
		// (skip/conflict). Antes solo se contaba en la rama 'apply', dejando
		// IgnoredTasks en 0 cuando el archivo no cambiaba pero igual tenía tasks
		// sin round-trip.
		if s.name == "tasks.md" {
			res.ignoredTasks = countUnmarkedTasks(a.files[s.name])
		}
		switch decideFile(a.meta.Hashes[s.name], ContentHash(a.files[s.name]), a.db.Hashes[s.name], a.force) {
		case "skip":
		case "conflict":
			res.conflicts = append(res.conflicts, s.name)
		case "apply":
			if err := s.fn(); err != nil {
				res.conflicts = append(res.conflicts, s.name+": "+err.Error())
			} else {
				res.applied = append(res.applied, s.name)
			}
		}
	}
	return res
}

// countUnmarkedTasks cuenta las tasks de tasks.md que no tienen marcador
// <!-- t:uuid --> (sin round-trip). Puro, no toca BD (R2).
func countUnmarkedTasks(md string) int {
	n := 0
	for _, td := range ParseTasks(md) {
		if td.ID == "" {
			n++
		}
	}
	return n
}

func decideFile(stored, repoNow, dbNow string, force bool) string {
	if repoNow == stored {
		return "skip"
	}
	if dbNow != stored && !force {
		return "conflict"
	}
	return "apply"
}

// applyProposal rechaza md vacío ANTES de versionar (BUG 1): un .md borrado no
// debe pisar la DB con una versión vacía.
func (e *Engine) applyProposal(ctx context.Context, issueID uuid.UUID, md string) error {
	if strings.TrimSpace(md) == "" {
		return ErrEmptyDoc
	}
	p := ParseProposal(md)
	_, err := e.SpecW.CreateProposal(ctx, issueID, p.Intention, p.Scope, p.Approach, p.Risks, p.TestingNotes)
	return err
}

// applyDesign rechaza md vacío ANTES de versionar (BUG 1).
func (e *Engine) applyDesign(ctx context.Context, issueID uuid.UUID, md string) error {
	if strings.TrimSpace(md) == "" {
		return ErrEmptyDoc
	}
	d := ParseDesign(md)
	_, err := e.SpecW.CreateDesign(ctx, issueID, nil, d.ArchDecisions, d.Alternatives, d.DataFlow, d.TDDPlan, d.RisksMitigation)
	return err
}

// applyScenarios reemplaza los escenarios Gherkin de forma atómica (BUG 2):
// PRIMERO parsea y valida TODOS los nuevos; recién si todo es válido borra los
// viejos e inserta los nuevos. Si la validación falla no se borra nada.
func (e *Engine) applyScenarios(ctx context.Context, iss *issuesvc.Issue, md string) error {
	if strings.TrimSpace(md) == "" {
		return ErrEmptyDoc
	}
	parsed := ParseScenarios(md)
	news := make([]issuesvc.Scenario, 0, len(parsed))
	for _, d := range parsed {
		if err := validateScenario(d); err != nil {
			return err
		}
		news = append(news, issuesvc.Scenario{
			Feature: d.Feature, Scenario: d.Scenario,
			Given: d.Given, When: d.When, Then: d.Then,
		})
	}
	if len(news) == 0 {
		return errors.New("spec.md no contiene escenarios válidos: no se reemplaza para no perder los existentes. " +
			"Formato esperado (el parser acepta heading ## o #### y Given/When/Then plano, con bullet o con negrita):\n" +
			"#### Scenario: descripción breve\n" +
			"- **Given** precondición\n" +
			"- **When** acción\n" +
			"- **Then** resultado verificable")
	}
	for _, sc := range iss.Scenarios {
		if err := e.IssuesW.RemoveScenario(ctx, sc.ID); err != nil {
			return err
		}
	}
	for _, sc := range news {
		if _, err := e.IssuesW.AddScenario(ctx, iss.Slug, sc); err != nil {
			return err
		}
	}
	return nil
}

func validateScenario(d ScenarioDoc) error {
	if strings.TrimSpace(d.Feature) == "" {
		return errors.New("escenario sin Feature (H1 del spec.md)")
	}
	if strings.TrimSpace(d.Scenario) == "" {
		return errors.New("escenario sin nombre (## Scenario:)")
	}
	if !hasNonEmpty(d.Given) {
		return errors.New("escenario '" + d.Scenario + "' sin Given")
	}
	if !hasNonEmpty(d.Then) {
		return errors.New("escenario '" + d.Scenario + "' sin Then")
	}
	return nil
}

func hasNonEmpty(xs []string) bool {
	for _, x := range xs {
		if strings.TrimSpace(x) != "" {
			return true
		}
	}
	return false
}

// applyTasks sincroniza el estado de las tasks con marcador <!-- t:uuid --> y
// devuelve cuántas tasks sin marcador se ignoraron (R2), para que el apply lo
// reporte en vez de descartarlas en silencio.
func (e *Engine) applyTasks(ctx context.Context, issueID uuid.UUID, md, completedBy string) (int, error) {
	current, err := e.TasksR.ListTasks(ctx, issueID)
	if err != nil {
		return 0, err
	}
	status := map[string]string{}
	for _, t := range current {
		status[t.ID.String()] = t.Status
	}
	ignored := 0
	for _, td := range ParseTasks(md) {
		if td.ID == "" {
			// Task sin marcador: no tiene round-trip, no se puede sincronizar.
			ignored++
			continue
		}
		if !td.Completed {
			continue
		}
		if err := e.advanceTaskToCompleted(ctx, td.ID, status[td.ID], completedBy); err != nil {
			return ignored, err
		}
	}
	return ignored, nil
}

func (e *Engine) advanceTaskToCompleted(ctx context.Context, idStr, current, by string) error {
	taskID, err := uuid.Parse(idStr)
	if err != nil {
		return nil
	}
	if current == "pending" {
		if _, err := e.TasksW.UpdateTaskStatus(ctx, taskID, "in_progress", by); err != nil {
			return err
		}
		current = "in_progress"
	}
	if current == "in_progress" {
		if _, err := e.TasksW.UpdateTaskStatus(ctx, taskID, "completed", by); err != nil {
			return err
		}
	}
	return nil
}

func (e *Engine) applyStatus(ctx context.Context, iss *issuesvc.Issue, newStatus string) string {
	if newStatus == "" || newStatus == iss.Status {
		return ""
	}
	if _, err := e.IssuesW.Update(ctx, iss.Slug, nil, nil, &newStatus, nil); err != nil {
		return "status: " + err.Error()
	}
	return "status -> " + newStatus
}

// ───────────────────────── helpers ─────────────────────────

// GroupByChange agrupa archivos por directorio de change, con keys relativas al
// dir del change (igual que groupFilesByChange del MCP, pero sobre []File).
func GroupByChange(files []File) map[string]map[string]string {
	byDir := map[string]map[string]string{}
	for _, f := range files {
		if f.Path == "" {
			continue
		}
		dir := changeDirOf(f.Path)
		if byDir[dir] == nil {
			byDir[dir] = map[string]string{}
		}
		byDir[dir][strings.TrimPrefix(f.Path, dir)] = f.Content
	}
	return byDir
}

func changeDirOf(path string) string {
	if i := strings.Index(path, "/specs/"); i >= 0 {
		return path[:i+1]
	}
	if i := strings.LastIndex(path, "/"); i >= 0 {
		return path[:i+1]
	}
	return ""
}

func ptrStr(s *string) string {
	if s == nil {
		return ""
	}
	return *s
}
