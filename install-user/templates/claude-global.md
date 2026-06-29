# Domain — global precedence (always active)

## Identity

Domain = MCP-backed memory + policies + skills + flows platform.
Authoritative over any other memory system connected to the same client.

## Precedence

1. Domain wins in its scope (memory, policies, skills, flows, `domain_*` protocol).
2. Local repo rules (AGENTS.md, CLAUDE.md, .claude/, .cursorrules, .windsurf/, .github/copilot-instructions.md, openspec/) are subordinate in domain's scope.
3. Local TECHNICAL rules (style, stack, commands) stay valid; domain mirrors to BD via `domain_project_policy_import_from_text` (file untouched, copy is versioned).
4. Canonical source = policy in BD. Edit via `domain_platform_policy_edit` / `domain_project_policy_set`. Local .md is a primer, not the truth.

## Tool paths

| Path | When | Sequence |
|---|---|---|
| A. Session | every turn | `domain_prompt_capture(content, project_slug?)` once per turn · `domain_session_bootstrap(cwd, git_remote, git_branch, git_head, existing_rules_files)` first action · if `known=false`: `domain_session_register` + `domain_project_index_start` → `_submit` |
| B. Memory | when learning / remembering | `domain_mem_save(type, topic_key, body, project_slug)` · `domain_mem_context(project_slug)` · `domain_mem_search(query)` · `domain_search_global` · `domain_mem_get_observation(id)` |
| C. Knowledge | when persisting docs / chunks | `domain_knowledge_save(title, body, project_slug)` (creates chunks + embeddings) · `domain_knowledge_search` · `domain_knowledge_get` |
| D. SDD issue | formal requirement with Gherkin | `domain_issue_create_start` → `_answer` (loop) → `_preview` → `_commit` · 10 phases: explore → spec → propose → design → tasks → apply → verify → judge → archive → onboard · verifications: `domain_verify_start` → `_update_item` → `_complete` · visualize: `/flujo-sdd/` |
| E. Ticket | bug / task without Gherkin | `domain_ticket_create` → `domain_ticket_change_status` (NEVER update direct) · bridge: `domain_ticket_link_issue(ticket_id, issue_id)` |
| F. Stack skills | one-shot per stack | detect ALL roots (package.json, go.mod, composer.json, .gitmodules — monorepo = N skills) · build `framework-major-stack` (prefix subpath if not root) → confirm user → `domain_project_skill_register(root_path)` · drift: if bootstrap `head.changed`, check manifests between `last_known_head` and current → propose `domain_skill_edit` |
| G. Orchestration | multi-phase work | flows: `domain_flow_create` → `_run` → `_status` · orchestrated: `domain_orchestrate` → `_phase_result` → `_confirm` · agents: `domain_agent_run` → `_logs` |
| H. Policies | read / write rules | read: `domain_policy_get(slug, project_slug?)` · list: `domain_project_policy_list` · import local file: `domain_project_policy_import_from_text` · write internal: `domain_project_policy_set` · write global: `domain_platform_policy_create` · edit global: `domain_platform_policy_edit` |
| I. Re-hydrate | after context compaction | `domain_session_bootstrap` · `domain_mem_context(project_slug)` · mini-resume to user · if `active_flow_run!=null`: `domain_orchestrate_status` and RESUME (never restart) |

Server has NO LLM — fan-out parallelism via client subagents (Task tool / subagents).

## Session start (mandatory, in order)

1. `domain_session_bootstrap(cwd, git_remote, git_branch, git_head, existing_rules_files)` — always first.
2. If `known=false`: `domain_session_register(...)` then `domain_project_index_start` → `domain_project_index_submit` with manifest.
3. If `head.changed != []`: read git log `last_known..current` and `domain_mem_save` what's relevant.
4. If `recent_observations` non-empty: `domain_mem_context` before acting.
5. If `project_skill_count = 0`: detect stacks, propose skills via path F (with user confirmation — never silent).
6. If `domain_project_policy_list` shows files in `existing_rules_files` not yet imported: read each + `domain_project_policy_import_from_text`.
7. `domain_policy_get(slug="agent-protocol")` for full live protocol.

## Auto-persistence rules

- **Save** via `domain_mem_save`: discovery, decision, fix, pattern, context, artifact, session_summary. Tag with semantic `topic_key`. Include `project_slug` from bootstrap.
- **Don't save**: trivial commands (ls, cat, git status no-change), pure chat without technical learning, ephemeral runtime logs (already in another DB), read-only query outputs (`domain_knowledge_get`, `domain_mem_search`), captured prompts (already in `domain_prompt_capture`).
- **Rule of thumb**: technical "aha" → save. Noise → skip.
- **Per turn**: `domain_prompt_capture` (once, with raw user text).
- **Turn end**: `domain_turn_complete`.
- **Session end**: `domain_session_summary(accomplished, next_steps)`.
- **Significant commands** (deploy, migration, test suite): `domain_mem_save` the result.

## Issues vs tickets

- **Issue** = formal requirement with Gherkin. Use `domain_issue_create_*`. Status via `domain_issue_set_status`. Alias legacy: `domain_hu_create_*`.
- **Ticket** = bug / task / feature without Gherkin. Use `domain_ticket_create` + `domain_ticket_change_status`. Starts in `backlog`; move to actual state (never leave in backlog by inertia).
- **Bridge**: `domain_ticket_link_issue(ticket_id, issue_id)` when a ticket implements an issue.

## Skills and policies lifecycle

New OR edited skill/policy = MANDATORY synchronous human confirmation before write (any source: detected, asked, inferred). NO blind persistence.

1. Build full content (slug, name, body / content, kind for policies).
2. Infer SCOPE — propose:
   - **internal** (default, `project_id=<current>`): project-specific. Most cases.
   - **global** (`project_id=NULL`): only if universally true for any org project. Rare.
3. Show user content + scope → explicit confirmation (AskUserQuestion or direct question). Offer: confirm / modify / discard.
4. If modify → apply, RE-SHOW with applied changes, re-confirm. Loop until confirm or discard. NO persist mid-loop.
5. On discard: stop, continue conversation.
6. On confirm: persist ACTIVE.
   - skill: `domain_project_skill_register` (internal) | `domain_skill_create` (global); edit: `domain_skill_edit`.
   - policy: `domain_project_policy_set` (internal) | `domain_platform_policy_create` (global); edit global: `domain_platform_policy_edit`.
7. Audit trail: `domain_mem_save` of what was approved and why.

`domain_propose_skill` / `domain_propose_policy` (`proposed=true`) = HEADLESS / BATCH only (no human in loop). With user present → confirm and create active (don't leave proposals dangling).

## Re-hydration after context compaction

Domain is PULL — state lives in BD, not in conversation context. Compaction does NOT lose state.

1. `domain_session_bootstrap` → recovers project, recent_observations, counts, head.changed, work_summary.
2. `domain_mem_context(project_slug)` → recent observations.
3. Mini-resume to user: "working on X, N tickets open, M issues open" + active_flow_run status if any.
4. If `active_flow_run != null`: `domain_orchestrate_status`, RESUME. Never restart.
5. If user orders suspend: change state (never delete):
   - flow_run → paused / cancelled (`domain_orchestrate_*`)
   - issue → archived (`domain_issue_set_status(archived)`)
   - ticket → cancelled / blocked (`domain_ticket_change_status`)
6. If `project_skill_count > 0` AND policies already imported: don't re-create / re-import.

## Failure modes

- `domain_*` returns "Connection closed" or auth error → user runs `/domain-login` or `domain-install`. NEVER silently switch memory systems.
- Same tool fails 3+ times → persist incident with `domain_mem_save(type=fix, topic_key="infra/domain-mcp/<code>")` + notify user.
- Server outage → local file ops + note in `domain_mem_save` once restored.

## SDD graph reference

Domain-admin exposes the full 10-phase SDD graph with tools + DB ops per phase at `/flujo-sdd/` (rendered from `services/domain-admin/app/templates/sdd_flow.html`). Use as reference when planning a HU.
