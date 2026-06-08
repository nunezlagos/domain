// Package commands — implementa los subcomandos del CLI Domain.
//
// Estructura: domain <resource> <action> [args] [flags]
// Resources: projects, observations, agents, flows, skills, search.
// Actions estándar: list, get, create, run, delete.
package commands

import (
	"flag"
	"fmt"
	"os"
	"strings"

	clicli "nunezlagos/domain/internal/cli/client"
	"nunezlagos/domain/internal/cli/output"
)

// Dispatch parsea args y ejecuta el subcomando.
// Args incluyen TODO desde domain[1:] (sin el binary path).
func Dispatch(args []string) int {
	if len(args) == 0 {
		printUsage()
		return 2
	}

	cmd := args[0]
	rest := args[1:]

	switch cmd {
	case "projects":
		return projects(rest)
	case "observations", "obs":
		return observations(rest)
	case "agents":
		return agents(rest)
	case "flows":
		return flows(rest)
	case "skills":
		return skills(rest)
	case "search":
		return search(rest)
	case "context":
		return contextCmd(rest)
	case "completion":
		return completion(rest)
	case "policies":
		return policies(rest)
	case "help", "-h", "--help":
		printUsage()
		return 0
	default:
		fmt.Fprintf(os.Stderr, "comando desconocido: %s\n\n", cmd)
		printUsage()
		return 2
	}
}

func printUsage() {
	fmt.Println(`domain CLI — interactúa con la API Domain

Uso:
  domain <resource> <action> [args] [flags]

Recursos:
  projects      ls | get <slug> | create <slug> <name>
  observations  ls --project <slug> | save --project <slug> <content>
  agents        ls | get <id_or_slug> | run <id> <input>
  flows         ls | run <id>
  skills        ls [--type X] [--tag Y]
  search        <query> [--limit N] [--type csv]
  context       [--project <slug>]
  policies      import-md <dir> | export-md [dir]
  audit         prune [--retention N] [--dry-run]
  completion    bash|zsh|fish

Flags globales:
  --format json|table   (default: table)

Env:
  DOMAIN_API_KEY      requerido
  DOMAIN_BASE_URL     default http://localhost:8000

Ejemplos:
  domain projects ls
  domain search "pgvector" --limit 5 --format json
  domain agents run my-agent "Revisá el PR"
  domain obs save --project demo "Decidimos usar X"`)
}

// --- helpers globales de flags ---

type globalFlags struct {
	Format string
}

func parseGlobalFlags(args []string) (*globalFlags, []string) {
	gf := &globalFlags{Format: "table"}
	var rest []string
	for i := 0; i < len(args); i++ {
		switch args[i] {
		case "--format":
			if i+1 < len(args) {
				gf.Format = args[i+1]
				i++
			}
		case "--json":
			gf.Format = "json"
		default:
			rest = append(rest, args[i])
		}
	}
	return gf, rest
}

func newClient() *clicli.Client {
	c, err := clicli.NewFromEnv()
	if err != nil {
		fmt.Fprintln(os.Stderr, "error:", err)
		os.Exit(1)
	}
	return c
}

func render(data any, format string) {
	_ = output.Render(os.Stdout, data, output.Format(format))
}

func handleErr(err error) int {
	if err != nil {
		fmt.Fprintln(os.Stderr, "error:", err)
		return 1
	}
	return 0
}

// === projects ===

func projects(args []string) int {
	if len(args) == 0 {
		fmt.Fprintln(os.Stderr, "uso: domain projects <ls|get|create>")
		return 2
	}
	gf, args := parseGlobalFlags(args)
	c := newClient()
	switch args[0] {
	case "ls", "list":
		data, err := c.Do("GET", "/projects", nil, nil)
		if err != nil {
			return handleErr(err)
		}
		render(data, gf.Format)
		return 0
	case "get":
		if len(args) < 2 {
			fmt.Fprintln(os.Stderr, "uso: domain projects get <slug>")
			return 2
		}
		data, err := c.Do("GET", "/projects/"+args[1], nil, nil)
		if err != nil {
			return handleErr(err)
		}
		render(data, gf.Format)
		return 0
	case "create":
		if len(args) < 3 {
			fmt.Fprintln(os.Stderr, "uso: domain projects create <slug> <name>")
			return 2
		}
		body := map[string]any{"slug": args[1], "name": strings.Join(args[2:], " ")}
		data, err := c.Do("POST", "/projects", body, nil)
		if err != nil {
			return handleErr(err)
		}
		render(data, gf.Format)
		return 0
	}
	fmt.Fprintf(os.Stderr, "acción desconocida: %s\n", args[0])
	return 2
}

// === observations ===

func observations(args []string) int {
	if len(args) == 0 {
		fmt.Fprintln(os.Stderr, "uso: domain obs <ls|save>")
		return 2
	}
	gf, args := parseGlobalFlags(args)
	c := newClient()

	fs := flag.NewFlagSet("obs", flag.ContinueOnError)
	project := fs.String("project", "", "project slug")
	limit := fs.Int("limit", 50, "list limit")
	switch args[0] {
	case "ls", "list":
		_ = fs.Parse(args[1:])
		if *project == "" {
			fmt.Fprintln(os.Stderr, "--project requerido")
			return 2
		}
		q := map[string]string{"project_slug": *project, "limit": fmt.Sprint(*limit)}
		data, err := c.Do("GET", "/observations", nil, q)
		if err != nil {
			return handleErr(err)
		}
		render(data, gf.Format)
		return 0
	case "save":
		_ = fs.Parse(args[1:])
		positional := fs.Args()
		if *project == "" || len(positional) == 0 {
			fmt.Fprintln(os.Stderr, "uso: domain obs save --project <slug> <content>")
			return 2
		}
		body := map[string]any{
			"project_slug": *project,
			"content":      strings.Join(positional, " "),
		}
		data, err := c.Do("POST", "/observations", body, nil)
		if err != nil {
			return handleErr(err)
		}
		render(data, gf.Format)
		return 0
	}
	fmt.Fprintf(os.Stderr, "acción desconocida: %s\n", args[0])
	return 2
}

// === agents ===

func agents(args []string) int {
	if len(args) == 0 {
		fmt.Fprintln(os.Stderr, "uso: domain agents <ls|get|run>")
		return 2
	}
	gf, args := parseGlobalFlags(args)
	c := newClient()
	switch args[0] {
	case "ls":
		data, err := c.Do("GET", "/agents", nil, nil)
		if err != nil {
			return handleErr(err)
		}
		render(data, gf.Format)
		return 0
	case "get":
		if len(args) < 2 {
			fmt.Fprintln(os.Stderr, "uso: domain agents get <id>")
			return 2
		}
		data, err := c.Do("GET", "/agents/"+args[1], nil, nil)
		if err != nil {
			return handleErr(err)
		}
		render(data, gf.Format)
		return 0
	case "run":
		if len(args) < 3 {
			fmt.Fprintln(os.Stderr, "uso: domain agents run <agent_id> <input>")
			return 2
		}
		body := map[string]any{"input": strings.Join(args[2:], " ")}
		data, err := c.Do("POST", "/agents/"+args[1]+"/run", body, nil)
		if err != nil {
			return handleErr(err)
		}
		render(data, gf.Format)
		return 0
	}
	fmt.Fprintf(os.Stderr, "acción desconocida: %s\n", args[0])
	return 2
}

// === flows ===

func flows(args []string) int {
	if len(args) == 0 {
		fmt.Fprintln(os.Stderr, "uso: domain flows <ls|run>")
		return 2
	}
	gf, args := parseGlobalFlags(args)
	c := newClient()
	switch args[0] {
	case "ls":
		data, err := c.Do("GET", "/flows", nil, nil)
		if err != nil {
			return handleErr(err)
		}
		render(data, gf.Format)
		return 0
	case "run":
		if len(args) < 2 {
			fmt.Fprintln(os.Stderr, "uso: domain flows run <id>")
			return 2
		}
		data, err := c.Do("POST", "/flows/"+args[1]+"/run", map[string]any{}, nil)
		if err != nil {
			return handleErr(err)
		}
		render(data, gf.Format)
		return 0
	}
	return 2
}

// === skills ===

func skills(args []string) int {
	gf, args := parseGlobalFlags(args)
	c := newClient()
	if len(args) == 0 || args[0] == "ls" {
		fs := flag.NewFlagSet("skills", flag.ContinueOnError)
		typ := fs.String("type", "", "filter by type")
		tag := fs.String("tag", "", "filter by tag")
		_ = fs.Parse(args[1:])
		q := map[string]string{}
		if *typ != "" {
			q["type"] = *typ
		}
		if *tag != "" {
			q["tag"] = *tag
		}
		data, err := c.Do("GET", "/skills", nil, q)
		if err != nil {
			return handleErr(err)
		}
		render(data, gf.Format)
		return 0
	}
	return 2
}

// === search ===

func search(args []string) int {
	gf, args := parseGlobalFlags(args)
	if len(args) == 0 {
		fmt.Fprintln(os.Stderr, "uso: domain search <query> [--limit N] [--type csv]")
		return 2
	}
	fs := flag.NewFlagSet("search", flag.ContinueOnError)
	limit := fs.Int("limit", 20, "result limit")
	typ := fs.String("type", "", "entity type csv")
	_ = fs.Parse(args[1:])
	q := map[string]string{"q": args[0], "limit": fmt.Sprint(*limit)}
	if *typ != "" {
		q["entity_type"] = *typ
	}
	c := newClient()
	data, err := c.Do("GET", "/search", nil, q)
	if err != nil {
		return handleErr(err)
	}
	render(data, gf.Format)
	return 0
}

// === context ===

func contextCmd(args []string) int {
	gf, args := parseGlobalFlags(args)
	fs := flag.NewFlagSet("context", flag.ContinueOnError)
	project := fs.String("project", "", "project slug")
	_ = fs.Parse(args)
	q := map[string]string{}
	if *project != "" {
		q["project_slug"] = *project
	}
	c := newClient()
	data, err := c.Do("GET", "/context", nil, q)
	if err != nil {
		return handleErr(err)
	}
	render(data, gf.Format)
	return 0
}
