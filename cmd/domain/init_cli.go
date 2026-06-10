// Subcomandos `domain init` y `domain workflow` (issue-12.7).
//
//   domain init [--root <path>] [--dry-run] [--no-stub]
//     Detecta archivos .md de instrucciones IA en el repo + los archiva
//     en BD + los reemplaza por stubs que apuntan al MCP de Domain.
//
//   domain workflow list [--root <path>]
//     Lista archivos importados de la BD con su status actual.
//
//   domain workflow restore <rel-path> [--root <path>]
//     Reescribe el archivo .md original desde el backup en BD.

package main

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"text/tabwriter"

	"github.com/jackc/pgx/v5/pgxpool"

	"nunezlagos/domain/internal/config"
	"nunezlagos/domain/internal/service/workflowimport"
)

func runInit(args []string) {
	root := "."
	dryRun := false
	noStub := false

	for i := 0; i < len(args); i++ {
		switch args[i] {
		case "--root":
			if i+1 < len(args) {
				root = args[i+1]
				i++
			}
		case "--dry-run":
			dryRun = true
		case "--no-stub":
			noStub = true
		case "-h", "--help":
			fmt.Println(`domain init — plug-and-play override de archivos .md de instrucciones IA

Uso:
  domain init [--root <path>] [--dry-run] [--no-stub]

Flags:
  --root <path>    Directorio raíz del proyecto a escanear (default ".")
  --dry-run        Solo detecta y reporta; NO backup ni reemplazo en disco
  --no-stub        Backup en BD pero NO sobrescribe los .md originales

Patterns detectados:
  - CLAUDE.md / claude.md (Claude Code)
  - .claude/**/*.md
  - .opencode/**/*.md
  - .cursor/**/*.md, .cursorrules
  - .windsurfrules, .windsurf/**/*.md
  - AGENTS.md, AGENT.md, CONVENTIONS.md (genéricos)
  - .aider.conf.yml

Después de "init", los originales viven en BD (tabla imported_workflow_files)
y los .md en disco quedan como stubs apuntando al MCP de Domain.

Para rollback: domain workflow restore <rel-path>`)
			return
		}
	}

	if dryRun {
		runInitDryRun(root)
		return
	}

	cfg, err := config.Load()
	if err != nil {
		fmt.Fprintf(os.Stderr, "config: %v\n", err)
		os.Exit(1)
	}
	ctx := context.Background()
	pool, err := pgxpool.New(ctx, cfg.DatabaseURL)
	if err != nil {
		fmt.Fprintf(os.Stderr, "db open: %v\n", err)
		os.Exit(1)
	}
	defer pool.Close()

	svc := &workflowimport.Service{Pool: pool}

	stub := workflowimport.DefaultStub
	writeStub := !noStub

	report, err := svc.Import(ctx, workflowimport.ImportInput{
		ProjectRoot:  root,
		StubTemplate: stub,
		WriteStub:    writeStub,
	})
	if err != nil {
		fmt.Fprintf(os.Stderr, "import: %v\n", err)
		os.Exit(1)
	}

	fmt.Printf("✓ Detectados:   %d archivos .md de IA\n", len(report.Detected))
	fmt.Printf("✓ Backup en BD: %d\n", report.BackedUp)
	if writeStub {
		fmt.Printf("✓ Reemplazados: %d (stub apuntando al MCP)\n", report.Replaced)
	}
	if report.Skipped > 0 {
		fmt.Printf("· Skipped:      %d (mismo content_hash en BD)\n", report.Skipped)
	}
	if len(report.Errors) > 0 {
		fmt.Println()
		fmt.Println("Errores parciales:")
		for _, e := range report.Errors {
			fmt.Println("  ✗ " + e)
		}
	}

	fmt.Println()
	fmt.Println("Archivos importados:")
	w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
	fmt.Fprintln(w, "TOOL\tPATH\tSIZE\tHASH")
	for _, f := range report.Detected {
		short := f.ContentHash
		if len(short) > 8 {
			short = short[:8]
		}
		fmt.Fprintf(w, "%s\t%s\t%d\t%s\n", f.SourceTool, f.RelPath, f.SizeBytes, short)
	}
	w.Flush()

	fmt.Println()
	fmt.Println("Próximos pasos:")
	fmt.Println("  1. Conectá tu agente IA al MCP de Domain:")
	fmt.Println("     domain setup claude-code --api-key sk_... --base-url http://localhost:8000")
	fmt.Println("  2. Verificá los archivos importados:")
	fmt.Println("     domain workflow list")
	fmt.Println("  3. Rollback de uno específico (si querés):")
	fmt.Println("     domain workflow restore <rel-path>")
}

func runInitDryRun(root string) {
	scanner := &workflowimport.Scanner{ProjectRoot: root}
	files, err := scanner.Detect(false)
	if err != nil {
		fmt.Fprintf(os.Stderr, "scan: %v\n", err)
		os.Exit(1)
	}
	if len(files) == 0 {
		fmt.Println("No se detectaron archivos .md de IA en " + root)
		return
	}
	fmt.Printf("DRY RUN — detectados %d archivos (no se modificó BD ni disco):\n\n", len(files))
	w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
	fmt.Fprintln(w, "TOOL\tPATH\tSIZE")
	for _, f := range files {
		fmt.Fprintf(w, "%s\t%s\t%d\n", f.SourceTool, f.RelPath, f.SizeBytes)
	}
	w.Flush()
}

func runWorkflow(args []string) {
	if len(args) == 0 {
		fmt.Println(`domain workflow — gestión de archivos .md importados (issue-12.7)

Uso:
  domain workflow list [--root <path>]
  domain workflow restore <rel-path> [--root <path>]`)
		return
	}
	switch args[0] {
	case "list":
		runWorkflowList(args[1:])
	case "restore":
		runWorkflowRestore(args[1:])
	default:
		fmt.Fprintf(os.Stderr, "subcomando workflow desconocido: %s\n", args[0])
		os.Exit(2)
	}
}

func runWorkflowList(args []string) {
	root := "."
	for i := 0; i < len(args); i++ {
		if args[i] == "--root" && i+1 < len(args) {
			root = args[i+1]
			i++
		}
	}
	_ = root // ProjectID lookup queda pending; por ahora lista TODOS los registros

	cfg, err := config.Load()
	if err != nil {
		fmt.Fprintf(os.Stderr, "config: %v\n", err)
		os.Exit(1)
	}
	ctx := context.Background()
	pool, err := pgxpool.New(ctx, cfg.DatabaseURL)
	if err != nil {
		fmt.Fprintf(os.Stderr, "db open: %v\n", err)
		os.Exit(1)
	}
	defer pool.Close()

	svc := &workflowimport.Service{Pool: pool}
	files, err := svc.List(ctx, nil)
	if err != nil {
		fmt.Fprintf(os.Stderr, "list: %v\n", err)
		os.Exit(1)
	}
	if len(files) == 0 {
		fmt.Println("No hay archivos importados. Corré 'domain init' primero.")
		return
	}
	w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
	fmt.Fprintln(w, "TOOL\tPATH\tSTATUS\tSIZE\tIMPORTED_AT")
	for _, f := range files {
		fmt.Fprintf(w, "%s\t%s\t%s\t%d\t%s\n",
			f.SourceTool, f.RelPath, f.Status, f.SizeBytes,
			f.CreatedAt.Format("2006-01-02 15:04"))
	}
	w.Flush()
}

func runWorkflowRestore(args []string) {
	if len(args) < 1 {
		fmt.Fprintln(os.Stderr, "uso: domain workflow restore <rel-path> [--root <path>]")
		os.Exit(2)
	}
	relPath := args[0]
	root := "."
	for i := 1; i < len(args); i++ {
		if args[i] == "--root" && i+1 < len(args) {
			root = args[i+1]
			i++
		}
	}

	cfg, err := config.Load()
	if err != nil {
		fmt.Fprintf(os.Stderr, "config: %v\n", err)
		os.Exit(1)
	}
	ctx := context.Background()
	pool, err := pgxpool.New(ctx, cfg.DatabaseURL)
	if err != nil {
		fmt.Fprintf(os.Stderr, "db open: %v\n", err)
		os.Exit(1)
	}
	defer pool.Close()

	svc := &workflowimport.Service{Pool: pool}
	if err := svc.Restore(ctx, nil, relPath, root); err != nil {
		fmt.Fprintf(os.Stderr, "restore: %v\n", err)
		os.Exit(1)
	}
	fmt.Printf("✓ Restaurado %s desde backup en BD\n", relPath)
}

// dummy helper para evitar warnings de imports no usados si runInit no se
// linkea (en caso edge).
var _ = json.Marshal
