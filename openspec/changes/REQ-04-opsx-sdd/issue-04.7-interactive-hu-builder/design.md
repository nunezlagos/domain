# Design: issue-04.7-interactive-hu-builder

## Decisión arquitectónica

**State machine declarativa per mode** (no LLM-driven dinámico) — preguntas son deterministas y reproducibles. El LLM del agente IA usuario las contesta, pero el orden y validación está en el orquestador.

**State en BD**, no en memoria — porque MCP es stateless entre tool calls. Reanudación natural si el agente cae.

**Files en filesystem para git**, BD como audit. Source of truth para SDD git operations sigue siendo `openspec/changes/**` (issue-04.1 confirma). El wizard es generador-asistente, no replacement del filesystem.

## Componentes

```
internal/sdd/wizard/
  wizard.go              # Service: Start, Answer, Preview, Commit, Abandon
  state.go               # tipos Draft, Answer, Step, Mode
  steps.go               # registry de step flows per mode
  flow_feature.go        # 8 steps para mode=feature
  flow_bugfix.go         # 6 steps para mode=bug-fix
  flow_refactor.go       # 5 steps para mode=refactor
  flow_doc.go            # 3 steps para mode=doc
  flow_rfc.go            # 7 steps para mode=rfc
  validators.go          # audience exists, req exists, slug format, path safe
  templates/             # go:embed hu.md.tmpl, proposal.md.tmpl, design.md.tmpl, tasks.md.tmpl, state.yaml.tmpl, rfc.md.tmpl
  renderer.go            # render templates con answers
  inferpath.go           # next HU number, slug-ify, target_path

internal/store/pg/issue_drafts/
  store.go               # CRUD pgx wrap

internal/mcp/tools/sdd/
  wizard.go              # 6 MCP tools wrappers
```

## Step interface

```go
type Step struct {
  Key          string                                    // "audience", "req_parent", etc.
  Prompt       string                                    // human text
  OptionsFn    func(ctx, draft) ([]Option, error)        // dynamic options
  Validate     func(answer any, draft *Draft) error      // input validation
  NextStepFn   func(draft *Draft) string                 // optional branching
}

type Option struct {
  Value       string
  Label       string
  Description string
  Recommended bool
}
```

## Flow execution

```
start:
  draft := newDraft(mode, initialIdea)
  step := flowsByMode[mode][0]
  return question(step, ctx, draft)

answer(answer):
  draft := load(draftId)
  currentStep := flowsByMode[draft.mode][draft.currentStep]
  if err := currentStep.Validate(answer, draft); err != nil:
    return error_with_options_again
  draft.answers[currentStep.Key] = answer
  next := nextStep(currentStep, draft)
  if next == nil: status=finished; return preview
  return question(next, ctx, draft)

preview(draftId):
  draft := load
  return renderTemplates(draft.answers, draft.mode)

commit(draftId, confirm):
  preview := preview(draftId)
  writeFiles(preview.target_path, preview.files)
  draft.status = "committed"
  audit_log("hu.created", draft)
```

## Templates strategy

Templates Go embebidos via `go:embed templates/*.tmpl`. Cada uno espera mismo shape `Answers` map.

Ejemplo simplificado `hu.md.tmpl`:
```
# issue-{{.Number}}-{{.Slug}}

**Origen:** `{{.ParentREQ}}`
**Audience:** {{.Audiences}}
**Prioridad tentativa:** {{.Priority}}
**Tipo:** {{.Type}}

## Historia de usuario

**Como** {{.AudienceHuman}}
**Quiero** {{.Goal}}
**Para** {{.WhyValue}}

## Criterios de aceptación
{{range .Scenarios}}
### {{.Title}}
```gherkin
{{.Body}}
```
{{end}}
...
```

## TDD plan

1. Service.Start crea row con expires_at = now()+24h
2. Answer válida → next step retornado
3. Answer inválida (audience slug no existe) → ErrValidation
4. Flujo full mode=feature: 8 respuestas → status=finished + preview ready
5. Preview render genera 5 archivos coherentes
6. Commit escribe filesystem en target_path + audit log
7. Expired draft → ErrExpired (sin Commit)
8. Cron purge limpia >7d
9. MCP tools envueltos correctamente sobre Service
10. Sabotaje: race answer concurrente sobre mismo draft → optimistic locking con `updated_at`
