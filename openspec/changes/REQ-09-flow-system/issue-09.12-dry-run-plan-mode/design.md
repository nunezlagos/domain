# Design: issue-09.12-dry-run-plan-mode

## Decisión arquitectónica

**Analyzer:** AST walker stateless del flow spec.
**Token counting:** `pkoukk/tiktoken-go` para OpenAI/Anthropic; provider helpers para otros.
**Pricing:** lookup en `model_registry` table (issue-06.4).

## Plan output shape

```json
{
  "plan": [
    {
      "step_id": "fetch_data",
      "type": "skill_call",
      "will_execute": true,
      "estimated": {"tokens": 0, "cost_usd": 0.0}
    },
    {
      "step_id": "summarize",
      "type": "llm_call",
      "will_execute": true,
      "estimated": {"in_tokens": 1234, "out_tokens": 500, "cost_usd": 0.024}
    },
    {
      "step_id": "send_email",
      "type": "skill_call",
      "will_execute": "depends_on_runtime",
      "reason": "conditional depends on summarize.output.confidence",
      "estimated": {"tokens": 0, "cost_usd": 0.0},
      "warning": "skill has side-effects; skipped in dry-run"
    }
  ],
  "total_estimated": {
    "tokens": 1734,
    "cost_usd": 0.024,
    "warnings": ["1 step depends on runtime", "1 step has side-effects"]
  }
}
```

## TDD plan

1. Plan simple lineal
2. Conditional resuelto estático ambas ramas
3. Conditional dinámico marcado
4. LLM step cost computed
5. Side-effect warning
6. Loop/recursión cap
