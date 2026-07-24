// tool_channels.go — matriz de cobertura tool→canal (REQ-54 issue-54.6).
//
// Cada tool expuesta por el registry tiene EXACTAMENTE UN canal primario: el
// mecanismo que GARANTIZA (o norma) su invocación. La invariante "cero tools
// huérfanas" la congela TestAllToolsHaveChannel: una tool nueva sin canal
// rompe CI. Definición honesta de cobertura 100%: 100% ASIGNADAS, no 100%
// auto-invocadas — automatizar deletes/CRUD administrativo sería un bug.
//
// Este mapa es la FUENTE DE VERDAD de la matriz; el knowledge doc en BD se
// genera de acá (nunca editar el doc a mano).
package mcpserver

// ToolChannel es el canal primario de invocación de una tool.
type ToolChannel string

const (
	// ChannelHook: invocación DETERMINISTA por hook del cliente (SessionStart /
	// UserPromptSubmit / Stop). No depende del modelo.
	ChannelHook ToolChannel = "hook"
	// ChannelFirstResponse: protocolo obligatorio del primer mensaje de sesión
	// (regla R2 del SessionStart hook + prompt first-response).
	ChannelFirstResponse ToolChannel = "first-response"
	// ChannelPhaseContract: exigida por una fase del SDD vía required_tool_calls
	// (el server RECHAZA el cierre sin ella) o es el mecanismo del contrato.
	ChannelPhaseContract ToolChannel = "phase-contract"
	// ChannelPhasePrep: su función la entrega el server en la preparación de
	// contexto por fase (issue-54.2); el cliente la recibe sin pedirla.
	ChannelPhasePrep ToolChannel = "phase-prep"
	// ChannelPolicyTriggered: invocación normada por policy/protocolo (domain.md,
	// agent-protocol, auto-persistencia, señal del auto-trigger 54.4). El agente
	// la llama cuando la norma aplica.
	ChannelPolicyTriggered ToolChannel = "policy-triggered"
	// ChannelUserIntent: manual POR DISEÑO — solo cuando el humano lo pide
	// (CRUD administrativo, deletes, wizards conversacionales, confirmaciones).
	// Automatizarlas se considera regresión.
	ChannelUserIntent ToolChannel = "user-intent"
)

// toolChannel: la matriz completa. Mantener ORDENADA alfabéticamente.
var toolChannel = map[string]ToolChannel{
	"domain_agent_create":   ChannelUserIntent,
	"domain_agent_get":      ChannelUserIntent,
	"domain_agent_list":     ChannelUserIntent,
	"domain_agent_run":      ChannelUserIntent,
	"domain_agent_run_logs": ChannelUserIntent,

	"domain_attachment_confirm":     ChannelUserIntent,
	"domain_attachment_delete":      ChannelUserIntent,
	"domain_attachment_get_url":     ChannelUserIntent,
	"domain_attachment_init_upload": ChannelUserIntent,
	"domain_attachment_list":        ChannelUserIntent,

	"domain_client_create":     ChannelUserIntent,
	"domain_client_delete":     ChannelUserIntent,
	"domain_client_get":        ChannelUserIntent,
	"domain_client_list":       ChannelUserIntent,
	"domain_client_restore":    ChannelUserIntent,
	"domain_client_set_status": ChannelUserIntent,
	"domain_client_update":     ChannelUserIntent,

	// domain_code_* retiradas del manifest (DOMAINSERV-54): ocultas por default,
	// sin canal. Re-exponibles con DOMAIN_EXPOSE_CODE_TOOLS (ver code_graph_tools.go).

	"domain_context_snapshot": ChannelPolicyTriggered, // re-hidratación post-compactación

	"domain_cron_create":      ChannelUserIntent,
	"domain_cron_delete":      ChannelUserIntent,
	"domain_cron_history":     ChannelUserIntent,
	"domain_cron_list":        ChannelUserIntent,
	"domain_cron_set_enabled": ChannelUserIntent,

	"domain_error_reset": ChannelUserIntent,

	"domain_flow_cancel":         ChannelUserIntent,
	"domain_flow_create":         ChannelUserIntent,
	"domain_flow_grant_token":    ChannelHook, // post-orchestrate hook
	"domain_flow_list":           ChannelUserIntent,
	"domain_flow_run":            ChannelUserIntent,
	"domain_flow_status":         ChannelPolicyTriggered, // retomar flow activo (señal resume 54.4)
	"domain_flow_validate_token": ChannelHook,            // pre-edit hook

	"domain_health": ChannelPolicyTriggered, // failure modes: "cuando algo no funcione"

	// Wizards conversacionales: humano en el loop por definición.
	"domain_hu_create_abandon": ChannelUserIntent,
	"domain_hu_create_answer":  ChannelUserIntent,
	"domain_hu_create_commit":  ChannelUserIntent,
	"domain_hu_create_preview": ChannelUserIntent,
	"domain_hu_create_start":   ChannelUserIntent,
	"domain_hu_drafts_list":    ChannelUserIntent,

	"domain_intake_approve":      ChannelUserIntent,
	"domain_intake_get":          ChannelUserIntent,
	"domain_intake_list_pending": ChannelUserIntent,
	"domain_intake_reject":       ChannelUserIntent,
	"domain_intake_submit":       ChannelUserIntent,

	"domain_issue_create_abandon": ChannelUserIntent,
	"domain_issue_create_answer":  ChannelUserIntent,
	"domain_issue_create_commit":  ChannelUserIntent,
	"domain_issue_create_preview": ChannelUserIntent,
	"domain_issue_create_start":   ChannelUserIntent,
	"domain_issue_drafts_list":    ChannelUserIntent,
	"domain_issue_list":           ChannelUserIntent,
	"domain_issue_set_status":     ChannelUserIntent,

	"domain_knowledge_get":    ChannelPolicyTriggered,
	"domain_knowledge_save":   ChannelPhaseContract, // contrato de sdd-onboard
	"domain_knowledge_search": ChannelPolicyTriggered,

	"domain_known_error_set": ChannelPolicyTriggered, // failure modes

	"domain_mem_capture_passive":   ChannelPolicyTriggered,
	"domain_mem_context":           ChannelHook, // SessionStart lo pre-carga
	"domain_mem_delete":            ChannelUserIntent,
	"domain_mem_get_observation":   ChannelPolicyTriggered,
	"domain_mem_graph":             ChannelPolicyTriggered,
	"domain_mem_infer_edges":       ChannelUserIntent, // costoso, a pedido
	"domain_mem_infer_edges_llm":   ChannelUserIntent, // costoso (LLM), a pedido
	"domain_mem_link":              ChannelPolicyTriggered,
	"domain_mem_path":              ChannelPolicyTriggered,
	"domain_mem_related":           ChannelPolicyTriggered,
	"domain_mem_save":              ChannelPolicyTriggered, // auto-persistencia + D5 por fase
	"domain_mem_save_prompt":       ChannelUserIntent,
	"domain_mem_search":            ChannelPolicyTriggered,
	"domain_mem_stats":             ChannelUserIntent,
	"domain_mem_suggest_links":     ChannelPolicyTriggered,
	"domain_mem_suggest_topic_key": ChannelPolicyTriggered,
	"domain_mem_unlink":            ChannelUserIntent, // destructivo

	"domain_openspec_apply":  ChannelPhaseContract, // REQ-55.3 sync en propose/design/tasks
	"domain_openspec_export": ChannelPhaseContract, // REQ-55.3
	"domain_openspec_status": ChannelPhaseContract, // contrato de sdd-archive

	"domain_orchestrate":              ChannelPolicyTriggered, // señal determinista del hook (54.4) + policy
	"domain_orchestrate_confirm":      ChannelUserIntent,      // confirmación humana por definición
	"domain_orchestrate_phase_result": ChannelPhaseContract,   // ES el mecanismo del contrato

	"domain_platform_policy_create": ChannelUserIntent, // confirmación humana síncrona
	"domain_platform_policy_edit":   ChannelUserIntent,

	"domain_policy_get":  ChannelPolicyTriggered, // agent-protocol al inicio de sesión
	"domain_policy_list": ChannelFirstResponse,

	"domain_project_create":       ChannelUserIntent,
	"domain_project_index_start":  ChannelPolicyTriggered, // known=false (session start)
	"domain_project_index_status": ChannelPolicyTriggered,
	"domain_project_index_submit": ChannelPolicyTriggered,
	"domain_project_list":         ChannelUserIntent,
	"domain_project_update":       ChannelUserIntent,
	"domain_project_delete":       ChannelUserIntent,
	"domain_project_merge":        ChannelUserIntent,

	"domain_project_policy_delete":           ChannelUserIntent,
	"domain_project_policy_import_from_text": ChannelPolicyTriggered, // session start paso 7
	"domain_project_policy_list":             ChannelFirstResponse,
	"domain_project_policy_set":              ChannelUserIntent, // confirmación humana

	"domain_project_repo_add":         ChannelUserIntent,
	"domain_project_repo_delete":      ChannelUserIntent,
	"domain_project_repo_list":        ChannelPolicyTriggered, // session start paso 4 (disambiguación)
	"domain_project_repo_set_default": ChannelUserIntent,

	"domain_project_skill_list":     ChannelFirstResponse,
	"domain_project_skill_register": ChannelUserIntent, // confirmación humana
	"domain_project_skill_unlink":   ChannelUserIntent,

	"domain_prompt":               ChannelUserIntent, // router single-shot a pedido
	"domain_prompt_capture":       ChannelHook,       // UserPromptSubmit
	"domain_prompt_captured_list": ChannelUserIntent,
	"domain_prompt_heatmap":       ChannelUserIntent,    // análisis a pedido (DOMAINSERV-61)
	"domain_prompt_get":           ChannelFirstResponse, // first-response
	"domain_prompt_render":        ChannelUserIntent,
	"domain_prompt_search":        ChannelUserIntent,

	"domain_proposal_list":   ChannelUserIntent,
	"domain_proposal_review": ChannelUserIntent,
	"domain_propose_policy":  ChannelUserIntent, // headless/batch
	"domain_propose_skill":   ChannelUserIntent,

	"domain_search_global": ChannelPolicyTriggered,

	"domain_session_bootstrap": ChannelHook,            // SessionStart
	"domain_session_register":  ChannelPolicyTriggered, // known=false

	"domain_skill_create":  ChannelUserIntent, // confirmación humana
	"domain_skill_edit":    ChannelUserIntent,
	"domain_skill_execute": ChannelPolicyTriggered, // skills aplican por norma/threshold
	"domain_skill_get":     ChannelPolicyTriggered,
	"domain_skill_list":    ChannelPolicyTriggered,
	"domain_skill_search":  ChannelPolicyTriggered,

	"domain_sync_get_state":         ChannelUserIntent,
	"domain_sync_list_conflicts":    ChannelUserIntent,
	"domain_sync_mark_drift":        ChannelUserIntent,
	"domain_sync_mark_resolved":     ChannelUserIntent,
	"domain_sync_register_provider": ChannelUserIntent,
	"domain_sync_register_push":     ChannelUserIntent,

	"domain_ticket_change_status":      ChannelUserIntent,
	"domain_ticket_claim":              ChannelUserIntent,
	"domain_ticket_comment_add":        ChannelUserIntent,
	"domain_ticket_comment_list":       ChannelUserIntent,
	"domain_ticket_create":             ChannelPolicyTriggered, // señal "ticket" del capture (54.4)
	"domain_ticket_delete":             ChannelUserIntent,
	"domain_ticket_get":                ChannelUserIntent,
	"domain_ticket_link_external":      ChannelUserIntent,
	"domain_ticket_link_external_bulk": ChannelUserIntent,
	"domain_ticket_link_issue":         ChannelUserIntent,
	"domain_ticket_list":               ChannelFirstResponse,
	"domain_ticket_reassign":           ChannelUserIntent,
	"domain_ticket_release":            ChannelUserIntent,
	"domain_ticket_status_history":     ChannelUserIntent,
	"domain_ticket_update":             ChannelUserIntent,

	"domain_timeline": ChannelUserIntent,

	"domain_turn_complete": ChannelHook, // Stop

	"domain_usage_summary": ChannelUserIntent,

	"domain_verify_complete":    ChannelPhaseContract, // sdd-verify / sdd-review
	"domain_verify_pending":     ChannelPhaseContract,
	"domain_verify_start":       ChannelPhaseContract,
	"domain_verify_update_item": ChannelPhaseContract,

	// NOTA: domain_workflow_import/list/restore están definidas en
	// workflow_tools.go pero registerWorkflowTools NO tiene call-sites (deuda
	// issue-12.7: tools sin wire). Cuando se registren, clasificarlas acá —
	// TestAllToolsHaveChannel lo va a exigir.
	"domain_workflow_recent":  ChannelUserIntent,
	"domain_workflow_slowest": ChannelUserIntent,
	"domain_workflow_trace":   ChannelUserIntent,
}
