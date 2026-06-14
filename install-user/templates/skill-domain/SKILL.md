---
name: domain
description: Run every user turn through Domain MCP — classify intent, recover prior context from persistent memory, optionally start the SDD orchestrator pipeline, and persist non-obvious decisions/bugfixes. Use when the user sends ANY message (chat, idea, fix, feature, refactor, doc, rfc, analysis).
---

# Domain skill

This skill enforces the Domain MCP protocol on every user turn. Domain is a
remote MCP server (prefix `domain_*`) that centralizes:

- **Classification** of user intent (`domain_orchestrate`)
- **Persistent memory** across sessions (`domain_mem_save / mem_search / mem_context`)
- **Catalog** of projects, clients, knowledge, skills, agents, flows
- **SDD pipeline** for feature/fix work (multi-phase, validated)

## When to invoke

Always — at the start of every user turn — before responding or editing files.

## Procedure

### 1. Classify

Call `domain_orchestrate` with:

```
raw_text: <the user's full message>
project_slug: <inferred from cwd: prefer the basename of the git remote
              or the repo root directory name>
```

The response includes `intent` (one of `chat`, `idea`, `feature`, `fix`,
`hotfix`, `refactor`, `doc`, `rfc`, `analysis`) and, for actionable
intents, a `plan` with ordered steps.

### 2. Branch on intent

#### chat / idea
The response `reply` contains an inline protocol. Follow it:
- Call `domain_mem_search query=<keywords> project_slug=<slug>` to recover
  relevant prior context.
- Generate the answer **using that context** — do not invent history.
- If the chat surfaces something that should become work, propose escalating
  with `domain_orchestrate` (feature/fix) or `domain_intake_submit`.

#### feature / fix / hotfix / refactor / doc / rfc / analysis
- Execute each step of `plan` in order.
- Each step provides `system_prompt` + `user_prompt` + `suggested_saves`.
- After completing a step, report with
  `domain_orchestrate_phase_result(flow_run_step_id, output, memory_refs_saved)`.
- The response of `phase_result` tells you the next step (if any) or
  `requires_confirm=true` (then call `domain_orchestrate_confirm` after asking
  the user).

### 3. Work

Use the IDE's native tools (Edit, Read, Bash, etc.) to do code work. Domain
does not touch files — it tracks state and prompts.

### 4. Persist on close

Before the turn ends, call `domain_mem_save` for anything non-obvious:

```
title:    short searchable phrase
type:     decision | bugfix | pattern | config | discovery | learning | architecture
project:  <slug>
content:  |
  **What**: concrete one-liner
  **Why**: motivation (problem / user request / constraint)
  **Where**: paths or modules affected
  **Learned**: gotchas, edge cases (omit if nothing)
```

## Anti-patterns

- Do NOT respond from model memory without first calling `domain_mem_search`.
- Do NOT write notes to `~/notes/`, `TODO.md`, or local scratchpads —
  use `domain_mem_save` so the next session can find it.
- Do NOT read `.env` files for secrets that Domain could serve.
- Do NOT skip the classifier "because it looks like just a chat" — chat
  responses still get persistent context via the inline protocol in the
  reply.

## When this skill is NOT applicable

- The user's message is a literal `/help`, `/clear`, `/status` or other
  client-CLI command. Pass it through unchanged.
- The MCP server `domain` is unreachable (timeout / 5xx). Then fall back
  to local behavior and inform the user the protocol is degraded.
