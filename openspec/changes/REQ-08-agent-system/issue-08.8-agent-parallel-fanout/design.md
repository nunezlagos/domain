# Design: issue-08.8-agent-parallel-fanout

## Decisión arquitectónica

**Concurrency:** `golang.org/x/sync/errgroup` con context compartido.
**Budget:** atomic counter checked antes de cada LLM call por sub-run.
**Max concurrent:** 10 sub-runs por fan-out (config `DOMAIN_FANOUT_MAX_CONCURRENT`).
**Strategies:** función `Merger func([]Output) (Output, error)` registry built-in + custom via reduce_skill.

## Alternativas descartadas

- **goroutine pool global:** mezcla budgets entre fan-outs, peor accounting
- **Solo first/all (no vote/best):** la diferencia de implementación es pequeña y son patterns comunes
- **Streaming inline:** complejidad alta, deferir

## Schema diff

```sql
ALTER TABLE agent_runs
  ADD COLUMN fanout_group_id UUID,
  ADD COLUMN fanout_role VARCHAR(20);  -- parent | child | merger
CREATE INDEX ON agent_runs (fanout_group_id) WHERE fanout_group_id IS NOT NULL;
```

## Merger registry

```go
type Merger func(outputs []Output, opts MergeOpts) (Output, error)
var builtin = map[string]Merger{
  "first_completed": FirstCompleted,
  "all_results":     AllResults,
  "majority_vote":   MajorityVote,
  "best_of_n":       BestOfN,
  "reduce_skill":    nil, // delegado a skill externa
}
```

## Engine flow (pseudocódigo)

```go
func ParallelFanout(ctx context.Context, parent *AgentRun, p FanoutParams) (*Output, error) {
  ctx, cancel := context.WithTimeout(ctx, p.Timeout)
  defer cancel()
  g, gctx := errgroup.WithContext(ctx)
  g.SetLimit(min(len(p.Targets), MaxConcurrent))
  outputs := make([]Output, len(p.Targets))
  budget := atomic.Int64{}; budget.Store(p.TotalBudget)
  groupID := uuid.New()
  for i, target := range p.Targets {
    i, target := i, target
    g.Go(func() error {
      run := CreateChildRun(parent, target, fanoutGroup=groupID, role="child")
      run.BudgetChecker = func(needed int64) bool {
        return budget.Add(-needed) >= 0
      }
      out, err := Execute(gctx, run)
      if err != nil { outputs[i] = Output{Error: err}; return nil } // partial
      outputs[i] = out
      if p.MergeStrategy == "first_completed" { cancel() }
      return nil
    })
  }
  _ = g.Wait()
  merger := resolveMerger(p.MergeStrategy)
  return merger(outputs, MergeOpts{ReduceSkill: p.ReduceSkillSlug})
}
```

## TDD plan

1. 3 paralelos → all_results devuelve 3
2. first_completed cancela otros
3. timeout cancela pendientes, partial=true
4. error en 1 → otros OK en merge
5. budget pool exceeded → cancel early
6. reduce_skill invocado con outputs
7. max 10 concurrent enforcement
8. fanout_group_id permite ver grupo en tree
