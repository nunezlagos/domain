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
	"path/filepath"
	"strings"

	"github.com/google/uuid"
	mcpgo "github.com/mark3labs/mcp-go/server"

	"nunezlagos/domain/internal/audit"
	"nunezlagos/domain/internal/auth/apikey"
	"nunezlagos/domain/internal/cli/install"
	"nunezlagos/domain/internal/config"
	"nunezlagos/domain/internal/db"
	"nunezlagos/domain/internal/dispatch"
	"nunezlagos/domain/internal/llm"
	"nunezlagos/domain/internal/llm/anthropic"
	"nunezlagos/domain/internal/llm/google"
	"nunezlagos/domain/internal/llm/ollama"
	llmopenai "nunezlagos/domain/internal/llm/openai"
	llmratelimit "nunezlagos/domain/internal/llm/ratelimit"
	llmregistry "nunezlagos/domain/internal/llm/registry"
	llmretry "nunezlagos/domain/internal/llm/retry"
	mcpserver "nunezlagos/domain/internal/mcp/server"
	agentrunner "nunezlagos/domain/internal/runner/agent"
	flowrunner "nunezlagos/domain/internal/runner/flow"
	skillrunner "nunezlagos/domain/internal/runner/skill"
	agentsvc "nunezlagos/domain/internal/service/agent"
	capturedpromptsvc "nunezlagos/domain/internal/service/capturedprompt"
	clientsvc "nunezlagos/domain/internal/service/client"
	codegraphsvc "nunezlagos/domain/internal/service/codegraph"
	cronsvc "nunezlagos/domain/internal/service/cron"
	"nunezlagos/domain/internal/service/extsync"
	flowsvc "nunezlagos/domain/internal/service/flow"
	"nunezlagos/domain/internal/service/intake"
	issuesvc "nunezlagos/domain/internal/service/issue"
	"nunezlagos/domain/internal/service/issuebuilder"
	"nunezlagos/domain/internal/service/knowledge"
	"nunezlagos/domain/internal/service/observation"
	"nunezlagos/domain/internal/service/orchestrator"
	analysissvc "nunezlagos/domain/internal/service/orchestrator/analysis"
	"nunezlagos/domain/internal/service/orchestrator/phases"
	policysvc "nunezlagos/domain/internal/service/policy"
	projsvc "nunezlagos/domain/internal/service/project"
	projectpolicysvc "nunezlagos/domain/internal/service/projectpolicy"
	projectreposvc "nunezlagos/domain/internal/service/projectrepo"
	promptsvc "nunezlagos/domain/internal/service/prompt"
	"nunezlagos/domain/internal/service/promptrouter"
	requirementsvc "nunezlagos/domain/internal/service/requirement"
	searchsvc "nunezlagos/domain/internal/service/search"
	skillsvc "nunezlagos/domain/internal/service/skill"
	specsvc "nunezlagos/domain/internal/service/spec"
	tasksvc "nunezlagos/domain/internal/service/task"
	ticketsvc "nunezlagos/domain/internal/service/ticket"
	timelinesvc "nunezlagos/domain/internal/service/timeline"
	"nunezlagos/domain/internal/service/workflowimport"
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





	loadGlobalEnvFallback()

	cfg, err := config.Load()
	if err != nil {
		fmt.Fprintf(os.Stderr, "domain-mcp: config: %v\n", err)
		fmt.Fprintln(os.Stderr, "")
		fmt.Fprintln(os.Stderr, "Hint: run 'domain install' to generate ~/.config/domain/env,")
		fmt.Fprintln(os.Stderr, "or set DOMAIN_DATABASE_URL in the agent's MCP environment.")
		os.Exit(1)
	}
	apiKey := os.Getenv("DOMAIN_API_KEY")
	if apiKey == "" {
		apiKey = apiKeyFromCredentials()
	}
	if apiKey == "" {




		fmt.Fprintln(os.Stderr, "domain-mcp: DOMAIN_API_KEY is not set.")
		fmt.Fprintln(os.Stderr, "")
		fmt.Fprintln(os.Stderr, "To authenticate, run in your terminal:")
		fmt.Fprintln(os.Stderr, "  domain onboard")
		fmt.Fprintln(os.Stderr, "")
		fmt.Fprintln(os.Stderr, "Or, if opencode is already connected, type:")
		fmt.Fprintln(os.Stderr, "  /domain-login")
		fmt.Fprintln(os.Stderr, "")
		fmt.Fprintln(os.Stderr, "The wizard will create an API key, save it to")
		fmt.Fprintln(os.Stderr, "~/.config/domain/credentials.json, and reconfigure the MCP server.")
		os.Exit(1)
	}

	ctx := context.Background()
	pools, err := db.OpenProduction(ctx, cfg.DatabaseURL, cfg.DatabaseAuthURL)
	if err != nil {
		fmt.Fprintf(os.Stderr, "domain-mcp: pools: %v\n", err)
		os.Exit(1)
	}
	defer pools.Close()

	keys := &apikey.PGStore{Pool: pools.Auth, FieldEncKey: cfg.FieldEncKey}
	principal, err := keys.Resolve(ctx, apiKey)
	if err != nil {

		fmt.Fprintln(os.Stderr, "domain-mcp: API key is invalid or has been revoked.")
		fmt.Fprintln(os.Stderr, "")
		fmt.Fprintln(os.Stderr, "To re-authenticate, run in your terminal:")
		fmt.Fprintln(os.Stderr, "  domain onboard")
		fmt.Fprintln(os.Stderr, "")
		fmt.Fprintln(os.Stderr, "Or, if opencode is already connected, type:")
		fmt.Fprintln(os.Stderr, "  /domain-login")
		os.Exit(1)
	}

	recorder := &audit.PGRecorder{Pool: pools.Auth}



	clients := clientsvc.NewService(pools.App, recorder, nil)
	capturedPrompts := capturedpromptsvc.NewService(capturedpromptsvc.NewPgRepository(pools.App))
	projectRepos := projectreposvc.NewService(projectreposvc.NewPgRepository(pools.App))
	projectPolicies := projectpolicysvc.NewService(projectpolicysvc.NewPgRepository(pools.App))
	tickets := ticketsvc.NewService(ticketsvc.NewPgRepository(pools.App))
	projects := projsvc.NewService(pools.App, recorder, nil, nil).
		WithClientService(clients)
	observations := observation.NewService(pools.App, recorder, llm.NopEmbedder{}, nil, nil)
	observationEdges := observation.NewEdgeService(pools.App, llm.NopEmbedder{}, recorder)
	codeGraph := codegraphsvc.NewCodegraphService(pools.App)

	prompts := &promptsvc.Service{Pool: pools.App, Audit: recorder}
	timeline := &timelinesvc.Service{Pool: pools.App}
	search := &searchsvc.Service{Pool: pools.App}
	knowledgeSvc := &knowledge.Service{Pool: pools.App, Audit: recorder, Embedder: llm.NopEmbedder{}}
	skills := &skillsvc.Service{Pool: pools.App, Audit: recorder, Embedder: llm.NopEmbedder{}}
	agents := agentsvc.NewService(pools.App, recorder, nil)



	factory := llm.NewFactory()
	wrapLLM := func(p llm.Provider) llm.Provider {
		return llmratelimit.New(llmretry.New(p, llmretry.Config{}), 8)
	}
	if k := os.Getenv("DOMAIN_ANTHROPIC_KEY"); k != "" {
		factory.Register("anthropic", wrapLLM(anthropic.New(k)))
	}
	if k := os.Getenv("DOMAIN_OPENAI_KEY"); k != "" {
		factory.Register("openai", wrapLLM(llmopenai.New(k)))
	}
	if k := os.Getenv("DOMAIN_GOOGLE_KEY"); k != "" {
		factory.Register("google", wrapLLM(google.New(k)))
	}
	factory.Register("ollama", wrapLLM(ollama.New()))
	if def := os.Getenv("DOMAIN_LLM_PROVIDER"); def != "" {
		factory.SetDefault(def, def)
	}

	skillRunnerInst := skillrunner.New()

	modelRegistry := llmregistry.New()
	agentRunnerInst := &agentrunner.Runner{
		Pool: pools.App, Audit: recorder, Factory: factory,
		Agents: agents, Skills: skills,
		SkillRunner: skillRunnerInst, Models: modelRegistry,



		Env: cfg.Env,
	}

	flowService := &flowsvc.Service{Pool: pools.App, Audit: recorder}
	flowRunnerInst := &flowrunner.Runner{
		Pool: pools.App, Audit: recorder, Flows: flowService,
		Agents: agents, Skills: skills, Observations: observations,
		AgentRunner: agentRunnerInst, SkillRunner: skillRunnerInst,
		Signals: &flowsvc.SignalStore{Pool: pools.App},

	}






	orchPhases := phases.NewRegistry()
	orchPhases.MustRegister(phases.NewSDDExploreHandler())
	orchPhases.MustRegister(phases.NewSDDSpecHandler())
	orchPhases.MustRegister(phases.NewSDDProposeHandler())
	orchPhases.MustRegister(phases.NewSDDDesignHandler())
	orchPhases.MustRegister(phases.NewSDDTasksHandler())
	orchPhases.MustRegister(phases.NewSDDApplyHandler())
	orchPhases.MustRegister(phases.NewSDDVerifyHandler())
	orchPhases.MustRegister(phases.NewSDDJudgeHandler())
	orchPhases.MustRegister(phases.NewSDDReviewHandler())
	orchPhases.MustRegister(phases.NewSDDArchiveHandler())
	orchPhases.MustRegister(phases.NewSDDOnboardHandler())
	orchestratorSvc := orchestrator.New(pools.App, recorder, orchPhases, cfg.Env)

	orchestratorSvc.LLM = factory

	orchestratorSvc.Skills = skills



	analysisSvc := &analysissvc.Service{
		Pool:        pools.App,
		Audit:       recorder,
		LLM:         factory,
		Knowledge:   knowledgeSvc,
		Observation: observations,


		PromptLoader: func(ctx context.Context) (string, error) {
			p, err := prompts.GetActive(ctx, uuid.Nil, nil, "analysis")
			if err != nil {
				return "", err
			}
			return p.Body, nil
		},
	}

	issuebuilderSvc := &issuebuilder.Service{Pool: pools.App, Audit: recorder, DraftTTLHrs: 24}


	issuebuilderSvc.ReqSvc = &issuebuilder.RequirementServiceAdapter{
		Inner: &requirementsvc.Service{Pool: pools.App, Audit: recorder},
	}
	issuebuilderSvc.IssueSvc = &issuebuilder.IssueServiceAdapter{
		Inner: &issuesvc.Service{Pool: pools.App, Audit: recorder},
	}
	intakeSvc := &intake.Service{Pool: pools.App, Audit: recorder}
	extsyncSvc := &extsync.Service{Pool: pools.App}


	var classifier promptrouter.Classifier = promptrouter.HeuristicClassifier{}
	if anthrop, _ := factory.Get("anthropic"); anthrop != nil {
		classifier = &promptrouter.LLMClassifier{
			Provider: anthrop, Model: "claude-haiku-4-5-20251001",
			Fallback: promptrouter.HeuristicClassifier{},


			PromptLoader: func(ctx context.Context) (string, error) {
				p, err := prompts.GetActive(ctx, uuid.Nil, nil, "triage")
				if err != nil {
					return "", err
				}
				return p.Body, nil
			},
		}
	}
	promptRouterSvc := &promptrouter.Router{
		IntakeService:       intakeSvc,
		IssueBuilderService: issuebuilderSvc,
		Classifier:          classifier,



		Orchestrator:    orchestratorSvc,
		AnalysisService: &analysisAdapter{inner: analysisSvc},
	}
	workflowImportSvc := &workflowimport.Service{Pool: pools.App}




	mcpDispatcherAdapters := &dispatch.Adapters{
		FlowRunner:  flowRunnerInst,
		AgentRunner: agentRunnerInst,
		SkillRunner: skillRunnerInst,
		Agents:      agents,
		Skills:      skills,
	}
	mcpDispatcher := &dispatch.Dispatcher{
		RunFlow:  mcpDispatcherAdapters.RunFlowForDispatcher(),
		RunAgent: mcpDispatcherAdapters.RunAgentForDispatcher(),
		RunSkill: mcpDispatcherAdapters.RunSkillForDispatcher(),
		SourceValidator: func(s string) bool {
			return s == dispatch.SourceCron || s == dispatch.SourceWebhook ||
				s == dispatch.SourceMCP || s == dispatch.SourceManual
		},
	}

	srv := mcpserver.New(mcpserver.Deps{
		Observations:     observations,
		ObservationEdges: observationEdges,
		CodeGraph:        codeGraph,
		Projects:         projects,
		Prompts:      prompts,
		Timeline:     timeline,
		Search:       search,
		Knowledge:    knowledgeSvc,
		Skills:       skills,
		SkillExecution: &skillsvc.ExecutionService{
			Pool: pools.App, Skills: skills,
			Versions: &skillsvc.VersionStore{Pool: pools.App},
			Runner:   skillRunnerInst,
		},
		Crons:           &cronsvc.Service{Pool: pools.App, Audit: recorder},
		Clients:         clients,
		CapturedPrompts: capturedPrompts,
		ProjectRepos:    projectRepos,
		ProjectPolicies: projectPolicies,
		Tickets:         tickets,
		Policies:        &policysvc.Service{Pool: pools.App},
		Agents:          agents,
		AgentRunner:     agentRunnerInst,
		Flows:           flowService,
		FlowRunner:      flowRunnerInst,
		Orchestrator:    orchestratorSvc,
		Hubuilder:       issuebuilderSvc,
		IssueSvc:        &issuesvc.Service{Pool: pools.App, Audit: recorder},
		Spec:            &specsvc.Service{Pool: pools.App, Audit: recorder},
		Tasks:           &tasksvc.Service{Pool: pools.App, Audit: recorder},
		Intake:          intakeSvc,
		ExtSync:         extsyncSvc,
		PromptRouter:    promptRouterSvc,
		WorkflowImport:  workflowImportSvc,
		Pool:            pools.App,
		Principal:       principal,
		Dispatcher:      mcpDispatcher, // issue-35.1
		ServerName:      "domain-mcp",
		ServerVer:       Version,
	})

	fmt.Fprintf(os.Stderr, "domain-mcp %s ready (org=%s user=%s)\n",
		Version, principal.OrganizationID, principal.UserID)

	if err := mcpgo.ServeStdio(srv); err != nil {
		fmt.Fprintf(os.Stderr, "domain-mcp: serve: %v\n", err)
		os.Exit(1)
	}
}

// loadGlobalEnvFallback carga ~/.config/domain/env al environment del
// proceso SIN pisar variables ya seteadas. Lo escribe `domain install`
// (paso "Global MCP env"). Formato KEY=VALUE por línea, # comentarios.
func loadGlobalEnvFallback() {
	home, err := os.UserHomeDir()
	if err != nil {
		return
	}
	data, err := os.ReadFile(filepath.Join(home, ".config", "domain", "env"))
	if err != nil {
		return
	}
	for _, line := range strings.Split(string(data), "\n") {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		eq := strings.IndexByte(line, '=')
		if eq <= 0 {
			continue
		}
		key := strings.TrimSpace(line[:eq])
		val := strings.Trim(strings.TrimSpace(line[eq+1:]), `"'`)
		if os.Getenv(key) == "" {
			_ = os.Setenv(key, val)
		}
	}
}

// apiKeyFromCredentials lee la API key de ~/.config/domain/credentials.json
// (escrito por `domain install` / `domain onboard`). "" si no existe.
func apiKeyFromCredentials() string {
	data, err := os.ReadFile(install.CredentialsPath())
	if err != nil {
		return ""
	}
	creds, err := install.ParseCredentials(data)
	if err != nil {
		return ""
	}
	return creds.APIKey
}

// analysisAdapter puentea analysissvc.Input/Result → promptrouter.AnalysisInput/Result.
// Mantiene el analysis service desacoplado del promptrouter.
type analysisAdapter struct {
	inner *analysissvc.Service
}

func (a *analysisAdapter) RunAnalysis(ctx context.Context, in promptrouter.AnalysisInput) (*promptrouter.AnalysisResult, error) {
	result, err := a.inner.RunAnalysis(ctx, analysissvc.Input{
		OrganizationID: in.OrganizationID,
		UserID:         in.UserID,
		RawText:        in.RawText,
	})
	if err != nil {
		return nil, err
	}
	return &promptrouter.AnalysisResult{
		KnowledgeDocID: result.KnowledgeDocID,
		Title:          result.Title,
		Body:           result.Body,
	}, nil
}
