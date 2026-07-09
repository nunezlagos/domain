package openspec

import (
	"context"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	tasksvc "nunezlagos/domain/internal/service/task"
)

// fakeTaskReader/Writer para ejercitar applyTasks (R2) sin BD.
type fakeTaskRW struct {
	existing []tasksvc.Task
	updates  int
}

func (f *fakeTaskRW) ListTasks(_ context.Context, _ uuid.UUID) ([]tasksvc.Task, error) {
	return f.existing, nil
}

func (f *fakeTaskRW) UpdateTaskStatus(_ context.Context, _ uuid.UUID, _, _ string) (*tasksvc.Task, error) {
	f.updates++
	return &tasksvc.Task{}, nil
}

// --- R2: applyTasks cuenta las tasks sin marcador ignoradas ---

func TestApplyTasks_TasksSinMarcador_ReportaIgnoradas(t *testing.T) {
	frw := &fakeTaskRW{}
	e := &Engine{TasksR: frw, TasksW: frw}
	// 2 tasks sin marcador <!-- t:uuid --> y ninguna con marcador.
	md := "# Tasks\n\n## Implementación\n\n- [ ] tarea sin id uno\n- [x] tarea sin id dos\n"
	ignored, err := e.applyTasks(context.Background(), uuid.New(), md, "tester")
	require.NoError(t, err)
	assert.Equal(t, 2, ignored, "ambas tasks sin marcador deben contarse como ignoradas")
	assert.Equal(t, 0, frw.updates, "no debe intentar actualizar tasks sin id")
}

func TestApplyTasks_TaskConMarcador_NoIgnorada(t *testing.T) {
	id := uuid.New()
	frw := &fakeTaskRW{existing: []tasksvc.Task{{ID: id, Status: "pending"}}}
	e := &Engine{TasksR: frw, TasksW: frw}
	md := "# Tasks\n\n## Implementación\n\n- [x] tarea con id <!-- t:" + id.String() + " -->\n"
	ignored, err := e.applyTasks(context.Background(), uuid.New(), md, "tester")
	require.NoError(t, err)
	assert.Equal(t, 0, ignored, "una task con marcador no debe contarse como ignorada")
	assert.Greater(t, frw.updates, 0, "una task completada con id debe avanzar de estado")
}

// --- R2: IgnoredTasks se cuenta aun cuando tasks.md queda en 'skip' (hallazgo juez A) ---

func TestApplyFiles_TasksSkip_IgualCuentaIgnoradas(t *testing.T) {
	frw := &fakeTaskRW{}
	e := &Engine{TasksR: frw, TasksW: frw, SpecW: &fakeSpecWriter{}}
	tasksMd := "# Tasks\n\n## Implementación\n\n- [ ] sin marcador uno\n- [ ] sin marcador dos\n"
	// hash stored == hash del contenido -> decideFile devuelve 'skip'.
	a := applyCtx{
		issueID: uuid.New(), slug: "issue-x",
		meta:  Meta{Hashes: map[string]string{"tasks.md": ContentHash(tasksMd)}},
		files: map[string]string{"tasks.md": tasksMd},
		db:    &Rendered{Hashes: map[string]string{}},
	}
	res := e.applyFiles(context.Background(), a)
	assert.Equal(t, 2, res.ignoredTasks, "las tasks sin marcador se cuentan aunque tasks.md quede en skip")
	assert.NotContains(t, res.applied, "tasks.md", "tasks.md no cambió, no debe estar en applied")
}

// --- R7: applyChange marca unknown_issue con hint ---

func TestApplyChange_IssueIDInvalido_UnknownIssueConHint(t *testing.T) {
	e := &Engine{}
	// .openspec.yaml con issue_id no-uuid.
	files := map[string]string{".openspec.yaml": "domain:\n  issue_id: NO-ES-UUID\n"}
	res := e.applyChange(context.Background(), "dir/x", files, false, "tester")
	assert.True(t, res.UnknownIssue, "issue_id inválido debe marcar unknown_issue")
	assert.Contains(t, res.Error, "domain_openspec_export", "el error debe incluir hint accionable")
	assert.Empty(t, res.Conflicts, "un issue desconocido no es un conflicto de hash")
}

// --- R7: applyFiles clasifica un archivo omitido como not_sent, no conflict ---

func TestApplyFiles_ArchivoOmitido_EsNotSent(t *testing.T) {
	frw := &fakeTaskRW{}
	e := &Engine{TasksR: frw, TasksW: frw, SpecW: &fakeSpecWriter{}}
	// files trae solo proposal.md (con hash igual al stored -> skip, no dispara
	// CreateProposal). design/spec/tasks quedan omitidos del array -> not_sent.
	propMd := "# P\n\n## Why\ncontenido\n"
	a := applyCtx{
		issueID: uuid.New(), slug: "issue-x",
		meta:  Meta{Hashes: map[string]string{"proposal.md": ContentHash(propMd)}},
		files: map[string]string{"proposal.md": propMd},
		db:    &Rendered{Hashes: map[string]string{}},
	}
	res := e.applyFiles(context.Background(), a)
	// design.md, specs/.../spec.md y tasks.md no vinieron -> not_sent.
	assert.Contains(t, res.notSent, "design.md")
	assert.Contains(t, res.notSent, "tasks.md")
	assert.Contains(t, res.notSent, "specs/issue-x/spec.md")
	// ninguno de los omitidos debe aparecer como conflicto.
	for _, c := range res.conflicts {
		assert.NotContains(t, c, "design.md")
		assert.NotContains(t, c, "tasks.md")
	}
}
