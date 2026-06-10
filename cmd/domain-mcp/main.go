// Package main es el entrypoint de `domain-mcp`: servidor MCP stdio.
//
// issue-12.1 mcp-core-stdio. Resuelve principal vía env var DOMAIN_API_KEY al
// boot y expone tools `domain_mem_save`, `domain_mem_search`,
// `domain_mem_context`, `domain_mem_get_observation` sobre stdin/stdout JSON-RPC.
//
// Variables de entorno:
//
//	DOMAIN_API_KEY            (requerido) — API key plaintext del user
//	DOMAIN_DATABASE_URL       (requerido) — DSN app_user pool
//	DOMAIN_DATABASE_AUTH_URL  (opcional)  — DSN app_admin pool; fallback al primero
//
// El proceso es one-shot por sesión MCP (un proceso por cliente conectado).
package main

import (
	"context"
	"fmt"
	"os"

	mcpgo "github.com/mark3labs/mcp-go/server"

	"nunezlagos/domain/internal/audit"
	"nunezlagos/domain/internal/auth/apikey"
	"nunezlagos/domain/internal/config"
	"nunezlagos/domain/internal/db"
	"nunezlagos/domain/internal/llm"
	"nunezlagos/domain/internal/llm/anthropic"
	"nunezlagos/domain/internal/llm/ollama"
	llmopenai "nunezlagos/domain/internal/llm/openai"
	llmregistry "nunezlagos/domain/internal/llm/registry"
	mcpserver "nunezlagos/domain/internal/mcp/server"
	agentrunner "nunezlagos/domain/internal/runner/agent"
	flowrunner "nunezlagos/domain/internal/runner/flow"
	skillrunner "nunezlagos/domain/internal/runner/skill"
	agentsvc "nunezlagos/domain/internal/service/agent"
	"nunezlagos/domain/internal/service/billing"
	"nunezlagos/domain/internal/service/extsync"
	"nunezlagos/domain/internal/service/promptrouter"
	"nunezlagos/domain/internal/service/workflowimport"
	flowsvc "nunezlagos/domain/internal/service/flow"
	"nunezlagos/domain/internal/service/issuebuilder"
	"nunezlagos/domain/internal/service/intake"
	"nunezlagos/domain/internal/service/knowledge"
	"nunezlagos/domain/internal/service/observation"
	"nunezlagos/domain/internal/service/orchestrator"
	"nunezlagos/domain/internal/service/orchestrator/phases"
	projsvc "nunezlagos/domain/internal/service/project"
	promptsvc "nunezlagos/domain/internal/service/prompt"
	searchsvc "nunezlagos/domain/internal/service/search"
	sesssvc "nunezlagos/domain/internal/service/session"
	skillsvc "nunezlagos/domain/internal/service/skill"
	timelinesvc "nunezlagos/domain/internal/service/timeline"
)

var (
	Version   = "0.0.0-dev"
	Commit    = "unknown"
	BuildTime = "unknown"
)

func main() {
	if len(os.Args) > 1 {
		switch os.Args[1] {
		case "version", "--version", "-v":
			fmt.Printf("domain-mcp %s\ncommit: %s\nbuilt: %s\n", Version, Commit, BuildTime)
			return
		case "healthcheck":
			os.Exit(0)
		}
	}

	cfg, err := config.Load()
	if err != nil {
		fmt.Fprintf(os.Stderr, "domain-mcp: config: %v\n", err)
		os.Exit(1)
	}
	apiKey := os.Getenv("DOMAIN_API_KEY")
	if apiKey == "" {
		fmt.Fprintln(os.Stderr, "domain-mcp: DOMAIN_API_KEY requerido")
		os.Exit(1)
	}

	ctx := context.Background()
	pools, err := db.OpenProduction(ctx, cfg.DatabaseURL, cfg.DatabaseAuthURL)
	if err != nil {
		fmt.Fprintf(os.Stderr, "domain-mcp: pools: %v\n", err)
		os.Exit(1)
	}
	defer pools.Close()

	keys := &apikey.PGStore{Pool: pools.Auth}
	principal, err := keys.Resolve(ctx, apiKey)
	if err != nil {
		fmt.Fprintln(os.Stderr, "domain-mcp: API key inválida o revocada")
		os.Exit(1)
	}

	recorder := &audit.PGRecorder{Pool: pools.Auth}
	projects := &projsvc.Service{Pool: pools.App, Audit: recorder}
	observations := &observation.Service{
		Pool: pools.App, Audit: recorder, Embedder: llm.NopEmbedder{},
	}
	sessions := &sesssvc.Service{Pool: pools.App, Audit: recorder}
	prompts := &promptsvc.Service{Pool: pools.App, Audit: recorder}
	timeline := &timelinesvc.Service{Pool: pools.App}
	search := &searchsvc.Service{Pool: pools.App}
	knowledgeSvc := &knowledge.Service{Pool: pools.App, Audit: recorder, Embedder: llm.NopEmbedder{}}
	skills := &skillsvc.Service{Pool: pools.App, Audit: recorder, Embedder: llm.NopEmbedder{}}
	agents := &agentsvc.Service{Pool: pools.App, Audit: recorder}
	billingSvc := &billing.Service{Pool: pools.App}

	// LLM factory: providers según env vars DOMAIN_*_KEY.
	factory := llm.NewFactory()
	if k := os.Getenv("DOMAIN_ANTHROPIC_KEY"); k != "" {
		factory.Register("anthropic", anthropic.New(k))
	}
	if k := os.Getenv("DOMAIN_OPENAI_KEY"); k != "" {
		factory.Register("openai", llmopenai.New(k))
	}
	op := ollama.New()
	if h := os.Getenv("DOMAIN_OLLAMA_HOST"); h != "" {
		op.BaseURL = h
	}
	factory.Register("ollama", op)
	if def := os.Getenv("DOMAIN_LLM_PROVIDER"); def != "" {
		factory.SetDefault(def, def)
	}

	skillRunnerInst := skillrunner.New()
	modelRegistry := &llmregistry.Registry{Pool: pools.App}
	agentRunnerInst := &agentrunner.Runner{
		Pool: pools.App, Audit: recorder, Factory: factory,
		Agents: agents, Skills: skills, Billing: billingSvc,
		SkillRunner: skillRunnerInst, Models: modelRegistry,
		// issue-08.10 enforcement híbrido: checkOrphanPolicy sólo bloquea
		// cuando Env="prod"; dev/staging permiten runs sin flow_run_id
		// para iteración libre.
		Env: cfg.Env,
	}

	flowService := &flowsvc.Service{Pool: pools.App, Audit: recorder}
	flowRunnerInst := &flowrunner.Runner{
		Pool: pools.App, Audit: recorder, Flows: flowService,
		Agents: agents, Skills: skills, Observations: observations,
		AgentRunner: agentRunnerInst, SkillRunner: skillRunnerInst,
	}

	// issue-08.10 sdd-pipeline-orchestrator. Registry con los 10 handlers
	// de fase SDD. El registry rechaza duplicados via MustRegister →
	// boot panic si alguien accidentalmente registra dos veces el mismo
	// slug. Los system_prompts NO viven acá: se obtienen vía
	// Repository.GetAgentTemplateSystemPrompt desde agent_templates en BD.
	orchPhases := phases.NewRegistry()
	orchPhases.MustRegister(phases.NewSDDExploreHandler())
	orchPhases.MustRegister(phases.NewSDDSpecHandler())
	orchPhases.MustRegister(phases.NewSDDProposeHandler())
	orchPhases.MustRegister(phases.NewSDDDesignHandler())
	orchPhases.MustRegister(phases.NewSDDTasksHandler())
	orchPhases.MustRegister(phases.NewSDDApplyHandler())
	orchPhases.MustRegister(phases.NewSDDVerifyHandler())
	orchPhases.MustRegister(phases.NewSDDJudgeHandler())
	orchPhases.MustRegister(phases.NewSDDArchiveHandler())
	orchPhases.MustRegister(phases.NewSDDOnboardHandler())
	orchestratorSvc := orchestrator.New(pools.App, recorder, orchPhases, cfg.Env)
	// issue-08.10 svc-005: LLM factory inyectado para Mode=Solo. El
	// orquestador llama provider.Complete por cada fase server-side
	// cuando se invoca con Mode=ModeSolo (CI/CD, batch jobs).
	orchestratorSvc.LLM = factory

	issuebuilderSvc := &issuebuilder.Service{Pool: pools.App, Audit: recorder, DraftTTLHrs: 24}
	intakeSvc := &intake.Service{Pool: pools.App, Audit: recorder}
	extsyncSvc := &extsync.Service{Pool: pools.App}

	// issue-12.7 prompt router + workflow override.
	var classifier promptrouter.Classifier = promptrouter.HeuristicClassifier{}
	if anthrop, _ := factory.Get("anthropic"); anthrop != nil {
		classifier = &promptrouter.LLMClassifier{
			Provider: anthrop, Model: "claude-haiku-4-5-20251001",
			Fallback: promptrouter.HeuristicClassifier{},
		}
	}
	promptRouterSvc := &promptrouter.Router{
		IntakeService:       intakeSvc,
		IssueBuilderService: issuebuilderSvc,
		Classifier:          classifier,
		// issue-08.10 mcp-006: con Orchestrator inyectado, los intents
		// feat/fix/refactor/hotfix/rfc/doc disparan el pipeline SDD
		// plug-and-play del orquestador en lugar del wizard legacy.
		Orchestrator: orchestratorSvc,
	}
	workflowImportSvc := &workflowimport.Service{Pool: pools.App}

	srv := mcpserver.New(mcpserver.Deps{
		Observations:   observations,
		Projects:       projects,
		Sessions:       sessions,
		Prompts:        prompts,
		Timeline:       timeline,
		Search:         search,
		Knowledge:      knowledgeSvc,
		Skills:         skills,
		Agents:         agents,
		AgentRunner:    agentRunnerInst,
		Flows:          flowService,
		FlowRunner:     flowRunnerInst,
		Orchestrator:   orchestratorSvc,
		Hubuilder:      issuebuilderSvc,
		Intake:         intakeSvc,
		ExtSync:        extsyncSvc,
		PromptRouter:   promptRouterSvc,
		WorkflowImport: workflowImportSvc,
		Pool:           pools.App,
		Principal:      principal,
		ServerName:     "domain-mcp",
		ServerVer:      Version,
	})

	fmt.Fprintf(os.Stderr, "domain-mcp %s ready (org=%s user=%s)\n",
		Version, principal.OrganizationID, principal.UserID)

	if err := mcpgo.ServeStdio(srv); err != nil {
		fmt.Fprintf(os.Stderr, "domain-mcp: serve: %v\n", err)
		os.Exit(1)
	}
}
