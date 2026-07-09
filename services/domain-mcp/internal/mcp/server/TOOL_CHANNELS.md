# Matriz de cobertura tool→canal (REQ-54 issue-54.6)

Mantenido sincronizado con `internal/mcp/server/tool_channels.go` — validado por `TestToolChannelsDocInSync`.
Total: 150 tools, cero huérfanas (invariante: TestAllToolsHaveChannel).

## hook (4)
Determinista por evento del cliente (SessionStart / UserPromptSubmit / Stop). No depende del modelo.

- `domain_mem_context`
- `domain_prompt_capture`
- `domain_session_bootstrap`
- `domain_turn_complete`

## first-response (5)
Protocolo obligatorio del primer mensaje de sesión (regla R2 + prompt first-response).

- `domain_policy_list`
- `domain_project_policy_list`
- `domain_project_skill_list`
- `domain_prompt_get`
- `domain_ticket_list`

## phase-contract (9)
Exigida por una fase del SDD vía required_tool_calls — el server RECHAZA el cierre sin ella.

- `domain_knowledge_save`
- `domain_openspec_apply`
- `domain_openspec_export`
- `domain_openspec_status`
- `domain_orchestrate_phase_result`
- `domain_verify_complete`
- `domain_verify_pending`
- `domain_verify_start`
- `domain_verify_update_item`

## phase-prep (0)
Su función la entrega el server en la preparación de contexto por fase (issue-54.2).


## policy-triggered (36)
Normada por policy/protocolo (domain.md, auto-persistencia, señal del auto-trigger 54.4).

- `domain_code_explore`
- `domain_code_observations`
- `domain_code_path`
- `domain_code_upload`
- `domain_context_snapshot`
- `domain_flow_status`
- `domain_health`
- `domain_knowledge_get`
- `domain_knowledge_search`
- `domain_known_error_set`
- `domain_mem_capture_passive`
- `domain_mem_code_links`
- `domain_mem_get_observation`
- `domain_mem_graph`
- `domain_mem_link`
- `domain_mem_link_code`
- `domain_mem_path`
- `domain_mem_related`
- `domain_mem_save`
- `domain_mem_search`
- `domain_mem_suggest_links`
- `domain_mem_suggest_topic_key`
- `domain_orchestrate`
- `domain_policy_get`
- `domain_project_index_start`
- `domain_project_index_status`
- `domain_project_index_submit`
- `domain_project_policy_import_from_text`
- `domain_project_repo_list`
- `domain_search_global`
- `domain_session_register`
- `domain_skill_execute`
- `domain_skill_get`
- `domain_skill_list`
- `domain_skill_search`
- `domain_ticket_create`

## user-intent (96)
Manual POR DISEÑO: solo cuando el humano lo pide. Automatizarla es regresión.

- `domain_agent_create`
- `domain_agent_get`
- `domain_agent_list`
- `domain_agent_run`
- `domain_agent_run_logs`
- `domain_client_create`
- `domain_client_delete`
- `domain_client_get`
- `domain_client_list`
- `domain_client_restore`
- `domain_client_set_status`
- `domain_client_update`
- `domain_code_build`
- `domain_code_graph`
- `domain_cron_create`
- `domain_cron_delete`
- `domain_cron_history`
- `domain_cron_list`
- `domain_cron_set_enabled`
- `domain_error_reset`
- `domain_flow_cancel`
- `domain_flow_create`
- `domain_flow_list`
- `domain_flow_run`
- `domain_hu_create_abandon`
- `domain_hu_create_answer`
- `domain_hu_create_commit`
- `domain_hu_create_preview`
- `domain_hu_create_start`
- `domain_hu_drafts_list`
- `domain_intake_approve`
- `domain_intake_get`
- `domain_intake_list_pending`
- `domain_intake_reject`
- `domain_intake_submit`
- `domain_issue_create_abandon`
- `domain_issue_create_answer`
- `domain_issue_create_commit`
- `domain_issue_create_preview`
- `domain_issue_create_start`
- `domain_issue_drafts_list`
- `domain_issue_list`
- `domain_issue_set_status`
- `domain_mem_delete`
- `domain_mem_infer_edges`
- `domain_mem_infer_edges_llm`
- `domain_mem_save_prompt`
- `domain_mem_stats`
- `domain_mem_unlink`
- `domain_orchestrate_confirm`
- `domain_platform_policy_create`
- `domain_platform_policy_edit`
- `domain_project_create`
- `domain_project_list`
- `domain_project_policy_delete`
- `domain_project_policy_set`
- `domain_project_repo_add`
- `domain_project_repo_delete`
- `domain_project_repo_set_default`
- `domain_project_skill_register`
- `domain_project_skill_unlink`
- `domain_project_update`
- `domain_prompt`
- `domain_prompt_captured_list`
- `domain_prompt_render`
- `domain_prompt_search`
- `domain_proposal_list`
- `domain_proposal_review`
- `domain_propose_policy`
- `domain_propose_skill`
- `domain_skill_create`
- `domain_skill_edit`
- `domain_sync_get_state`
- `domain_sync_list_conflicts`
- `domain_sync_mark_drift`
- `domain_sync_mark_resolved`
- `domain_sync_register_provider`
- `domain_sync_register_push`
- `domain_ticket_change_status`
- `domain_ticket_claim`
- `domain_ticket_comment_add`
- `domain_ticket_comment_list`
- `domain_ticket_delete`
- `domain_ticket_get`
- `domain_ticket_link_external`
- `domain_ticket_link_external_bulk`
- `domain_ticket_link_issue`
- `domain_ticket_reassign`
- `domain_ticket_release`
- `domain_ticket_status_history`
- `domain_ticket_update`
- `domain_timeline`
- `domain_usage_summary`
- `domain_workflow_recent`
- `domain_workflow_slowest`
- `domain_workflow_trace`

