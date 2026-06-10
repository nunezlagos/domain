# Design: issue-10.2-conflict-semantic-judge

## Decisión arquitectónica

### Agent CLI interface

```go
type AgentCLI interface {
    Judge(ctx context.Context, prompt string) (*Judgment, error)
    Name() string
}

type ClaudeAgent struct{}
type OpenCodeAgent struct{}

func resolveAgent(agentName string) AgentCLI {
    switch strings.ToLower(agentName) {
    case "opencode":
        return &OpenCodeAgent{}
    case "claude":
        fallthrough
    default:
        return &ClaudeAgent{}
    }
}
```

### Prompt template

```
You are a memory deduplication judge. Given two memory observations,
determine their relationship.

Source: {source_title}
{source_content}

Target: {target_title}
{target_content}

Respond with JSON only:
{"verdict": "supersedes|conflicts_with|duplicate|unrelated", "confidence": 0.0-1.0, "reason": "..."}

Verdict meanings:
- supersedes: source replaces target (source has more recent/complete info)
- conflicts_with: both discuss same topic but contradict
- duplicate: same information, effectively identical
- unrelated: different topics, no relation
```

### ClaudeAgent implementation

```go
func (a *ClaudeAgent) Judge(ctx context.Context, prompt string) (*Judgment, error) {
    ctx, cancel := context.WithTimeout(ctx, 30*time.Second)
    defer cancel()

    cmd := exec.CommandContext(ctx, "claude", "-p", prompt)
    output, err := cmd.Output()
    if err != nil {
        if ctx.Err() == context.DeadlineExceeded {
            return nil, fmt.Errorf("timeout")
        }
        return nil, fmt.Errorf("exec error: %w", err)
    }
    return parseJudgment(string(output))
}
```

### OpenCodeAgent implementation

```go
func (a *OpenCodeAgent) Judge(ctx context.Context, prompt string) (*Judgment, error) {
    ctx, cancel := context.WithTimeout(ctx, 30*time.Second)
    defer cancel()

    cmd := exec.CommandContext(ctx, "opencode", "prompt", prompt)
    output, err := cmd.Output()
    if err != nil {
        return nil, fmt.Errorf("exec error: %w", err)
    }
    return parseJudgment(string(output))
}
```

### Judgment parsing

```go
type Judgment struct {
    Verdict    string  `json:"verdict"`
    Confidence float64 `json:"confidence"`
    Reason     string  `json:"reason"`
}

var validVerdicts = map[string]bool{
    "supersedes": true, "conflicts_with": true,
    "duplicate": true, "unrelated": true,
}

func parseJudgment(raw string) (*Judgment, error) {
    // Try to extract JSON from response (LLM may add extra text)
    jsonStart := strings.Index(raw, "{")
    jsonEnd := strings.LastIndex(raw, "}")
    if jsonStart == -1 || jsonEnd == -1 {
        return nil, fmt.Errorf("invalid_verdict: no JSON found")
    }
    var j Judgment
    if err := json.Unmarshal([]byte(raw[jsonStart:jsonEnd+1]), &j); err != nil {
        return nil, fmt.Errorf("invalid_verdict: %w", err)
    }
    if !validVerdicts[j.Verdict] {
        return nil, fmt.Errorf("invalid_verdict: unknown verdict %q", j.Verdict)
    }
    return &j, nil
}
```

### JudgePending with concurrency

```go
type JudgeOpts struct {
    Concurrency int
    MaxJudgments int // 0 = unlimited
    Agent       string
}

func JudgePending(ctx context.Context, db *sql.DB, opts JudgeOpts) (*JudgeReport, error) {
    rows, _ := db.QueryContext(ctx,
        "SELECT id, source_id, target_id, confidence, evidence FROM memory_relations WHERE judgment_status = 'pending'")

    var pending []MemoryRelation
    for rows.Next() { /* scan */ }

    if len(pending) == 0 {
        return &JudgeReport{Message: "no pending candidates to judge"}, nil
    }

    sem := make(chan struct{}, opts.Concurrency)
    results := make(chan JudgeResult, len(pending))
    ctx, cancel := context.WithCancel(ctx)
    defer cancel()

    for i, rel := range pending {
        if opts.MaxJudgments > 0 && i >= opts.MaxJudgments {
            break
        }
        go func(rel MemoryRelation) {
            sem <- struct{}{} // acquire
            defer func() { <-sem }() // release

            judgment, err := judgeOne(ctx, db, rel, resolveAgent(opts.Agent))
            results <- JudgeResult{RelationID: rel.ID, Judgment: judgment, Error: err}
        }(rel)
    }

    // Collect results
    report := &JudgeReport{}
    for i := 0; i < min(len(pending), opts.MaxJudgments) || (opts.MaxJudgments == 0 && i < len(pending)); i++ {
        r := <-results
        if r.Error != nil {
            report.Errors++
            updateJudgmentError(ctx, db, r.RelationID, r.Error.Error())
        } else {
            report.Success++
            updateJudgment(ctx, db, r.RelationID, r.Judgment)
        }
    }
    return report, nil
}
```

### DB update after judgment

```go
func updateJudgment(ctx context.Context, db *sql.DB, relID int64, j *Judgment) error {
    _, err := db.ExecContext(ctx,
        `UPDATE memory_relations SET
         relation = ?, judgment_status = 'judged',
         confidence = ?, reason = ?,
         marked_by_kind = 'llm', marked_by_model = ?,
         updated_at = datetime('now')
         WHERE id = ?`,
        j.Verdict, j.Confidence, j.Reason, currentAgent, relID)
    return err
}
```

## Alternativas descartadas

| Alternativa | Razón de descarte |
|-------------|-------------------|
| Embedding + cosine similarity | No permite juicio nuanced; LLM entiende contexto y contradicciones |
| API directa a LLM (OpenAI API) | Requiere API keys y credenciales; CLI es más universal |
| Sin concurrencia | Muy lento con cientos de candidates; worker pool es necesario |
| Sin timeout | LLM puede colgarse; timeout previene bloqueo |

## TDD plan

1. **Red:** parseJudgment extrae verdict válido → falla
2. **Green:** Implement parseJudgment → pasa
3. **Red:** JudgeBySemantic ejecuta CLI y persiste → falla (mock)
4. **Green:** Implement judgeOne con mock agent → pasa
5. **Red:** Concurrency control limita goroutines → falla
6. **Green:** Implement worker pool con semáforo → pasa
7. **Sabotaje:** No validar verdict → test de validación falla → restaurar

## Riesgos y mitigación

| Riesgo | Mitigación |
|--------|------------|
| CLI no instalada `claude`/`opencode` | Detect al inicio; error claro con sugerencia de instalación |
| LLM responde lento | Timeout 30s por juicio; concurrente para throughput |
| Respuesta no parseable | Raw response guardada en evidence field; no se pierde información |
| Costo de LLM por muchos juicios | --max-semantic limita por ejecución; threshold alto en scan léxico reduce candidates |
