package mcpserver

import (
	"context"
	"encoding/json"

	"github.com/google/uuid"
	"github.com/mark3labs/mcp-go/mcp"
	mcpgo "github.com/mark3labs/mcp-go/server"

	"nunezlagos/domain/internal/service/workflowimport"
)

type workflowImportService interface {
	Import(ctx context.Context, in workflowimport.ImportInput) (*workflowimport.ImportReport, error)
	List(ctx context.Context, projectID *uuid.UUID) ([]workflowimport.ImportedFile, error)
	Restore(ctx context.Context, projectID *uuid.UUID, relPath, projectRoot string) error
}

type workflowHandlers struct {
	workflow workflowImportService
}

func toolWorkflowImport() mcp.Tool {
	return mcp.NewTool("domain_workflow_import",
		mcp.WithDescription("Escanea el proyecto buscando archivos .md de instrucciones IA (CLAUDE.md, .claude/**, .opencode/**, .cursor/**, .windsurfrules, AGENTS.md, etc.) y los archiva en BD reemplazandolos por stubs que apuntan al MCP de Domain. Idempotente: skip si content_hash no cambio."),
		mcp.WithString("root",
			mcp.Description("Directorio raiz del proyecto (default '.')"),
		),
		mcp.WithBoolean("write_stub",
			mcp.Description("Si true (default), reemplaza el .md original con un stub. Si false, solo backup en BD."),
		),
		mcp.WithBoolean("dry_run",
			mcp.Description("Si true, solo detecta y reporta sin modificar BD ni disco."),
		),
	)
}

func toolWorkflowList() mcp.Tool {
	return mcp.NewTool("domain_workflow_list",
		mcp.WithDescription("Lista los archivos .md importados con su status (detected | backed_up | replaced | restored)."),
	)
}

func toolWorkflowRestore() mcp.Tool {
	return mcp.NewTool("domain_workflow_restore",
		mcp.WithDescription("Reescribe en disco el .md original desde el backup en BD. Usado para rollback selectivo."),
		mcp.WithString("rel_path",
			mcp.Description("Path relativo del archivo a restaurar (ej. 'CLAUDE.md')"),
			mcp.Required(),
		),
		mcp.WithString("root",
			mcp.Description("Directorio raiz del proyecto (default '.')"),
		),
	)
}

func (h *workflowHandlers) handleWorkflowImport(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	if h.workflow == nil {
		return mcp.NewToolResultError("workflow import service not configured"), nil
	}
	root := req.GetString("root", ".")
	writeStub := req.GetBool("write_stub", true)
	dryRun := req.GetBool("dry_run", false)

	if dryRun {
		scanner := &workflowimport.Scanner{ProjectRoot: root}
		files, err := scanner.Detect(false)
		if err != nil {
			return mcp.NewToolResultError("scan: " + err.Error()), nil
		}
		body, _ := json.MarshalIndent(map[string]any{
			"dry_run":  true,
			"detected": files,
			"count":    len(files),
		}, "", "  ")
		return &mcp.CallToolResult{Content: []mcp.Content{mcp.NewTextContent(string(body))}}, nil
	}

	rep, err := h.workflow.Import(ctx, workflowimport.ImportInput{
		ProjectRoot:  root,
		StubTemplate: workflowimport.DefaultStub,
		WriteStub:    writeStub,
	})
	if err != nil {
		return mcp.NewToolResultError("import: " + err.Error()), nil
	}
	body, _ := json.MarshalIndent(rep, "", "  ")
	return &mcp.CallToolResult{Content: []mcp.Content{mcp.NewTextContent(string(body))}}, nil
}

func (h *workflowHandlers) handleWorkflowList(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	if h.workflow == nil {
		return mcp.NewToolResultError("workflow import service not configured"), nil
	}
	files, err := h.workflow.List(ctx, nil)
	if err != nil {
		return mcp.NewToolResultError("list: " + err.Error()), nil
	}

	lite := make([]map[string]any, 0, len(files))
	for _, f := range files {
		lite = append(lite, map[string]any{
			"id":           f.ID,
			"source_tool":  f.SourceTool,
			"rel_path":     f.RelPath,
			"status":       f.Status,
			"size_bytes":   f.SizeBytes,
			"content_hash": f.ContentHash,
			"replaced_at":  f.ReplacedAt,
			"restored_at":  f.RestoredAt,
			"created_at":   f.CreatedAt,
		})
	}
	body, _ := json.MarshalIndent(lite, "", "  ")
	return &mcp.CallToolResult{Content: []mcp.Content{mcp.NewTextContent(string(body))}}, nil
}

func (h *workflowHandlers) handleWorkflowRestore(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	if h.workflow == nil {
		return mcp.NewToolResultError("workflow import service not configured"), nil
	}
	relPath, err := req.RequireString("rel_path")
	if err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}
	root := req.GetString("root", ".")
	if err := h.workflow.Restore(ctx, nil, relPath, root); err != nil {
		return mcp.NewToolResultError("restore: " + err.Error()), nil
	}
	body, _ := json.MarshalIndent(map[string]any{
		"restored": true, "rel_path": relPath,
	}, "", "  ")
	return &mcp.CallToolResult{Content: []mcp.Content{mcp.NewTextContent(string(body))}}, nil
}

func registerWorkflowTools(wrap *ResilientWrapper, deps Deps) []mcpgo.ServerTool {
	h := &workflowHandlers{workflow: deps.WorkflowImport}
	return []mcpgo.ServerTool{
		{Tool: toolWorkflowImport(), Handler: wrap.Wrap("domain_workflow_import", h.handleWorkflowImport)},
		{Tool: toolWorkflowList(), Handler: wrap.Wrap("domain_workflow_list", h.handleWorkflowList)},
		{Tool: toolWorkflowRestore(), Handler: wrap.Wrap("domain_workflow_restore", h.handleWorkflowRestore)},
	}
}
