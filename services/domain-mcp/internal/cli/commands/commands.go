package commands

import (
	"flag"
	"fmt"
	"os"
	"strings"

	clicli "nunezlagos/domain/internal/cli/client"
	"nunezlagos/domain/internal/cli/output"
)

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
	case "skills", "skill":
		return skills(rest)
	case "search":
		return search(rest)
	case "context":
		return contextCmd(rest)
	case "completion":
		return completion(rest)
	case "policies":
		return policies(rest)
	case "config":
		return configCmd(rest)
	case "man":
		return manCmd()
	case "help", "-h", "--help":
		printUsage()
		return 0
	default:
		fmt.Fprintf(os.Stderr, "comando desconocido: %s\n", cmd)
		if s := suggest(cmd, knownCommands); s != "" {
			fmt.Fprintf(os.Stderr, "¿Quisiste decir %q?\n", s)
		}
		fmt.Fprintln(os.Stderr)
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
  skills        ls [--type X] [--tag Y] | metrics ... | suggest <run|list>
  search        <query> [--limit N] [--type csv]
  context       [--project <slug>]
  policies      import-md <dir> | export-md [dir]
  audit         prune [--retention N] [--dry-run]
  completion    bash|zsh|fish|powershell
  config        view (API key solo prefix)
  man           imprime man page (domain man > .../man1/domain.1)

Flags globales:
  --format json|table|yaml|csv   (default: table)
  --no-headers                   omitir cabeceras en table/csv
  --quiet, -q                    solo errores (exit code)
  --verbose, -v                  imprime requests HTTP a stderr
  --config <path>                archivo YAML con api_key/base_url

Env:
  DOMAIN_API_KEY      requerido
  DOMAIN_BASE_URL     default http://localhost:8000

Ejemplos:
  domain projects ls
  domain search "pgvector" --limit 5 --format json
  domain agents run my-agent "Revisá el PR"
  domain obs save --project demo "Decidimos usar X"`)
}

type globalFlags struct {
	Format    string
	NoHeaders bool
	Quiet     bool
	Config    string
	Verbose   bool
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
		case "--yaml":
			gf.Format = "yaml"
		case "--csv":
			gf.Format = "csv"
		case "--no-headers":
			gf.NoHeaders = true
		case "--quiet", "-q":
			gf.Quiet = true
		case "--verbose", "-v":
			gf.Verbose = true
		case "--config":
			if i+1 < len(args) {
				gf.Config = args[i+1]
				i++
			}
		default:
			rest = append(rest, args[i])
		}
	}
	return gf, rest
}

func newClient(gf *globalFlags) *clicli.Client {
	var c *clicli.Client
	var err error
	if gf != nil && gf.Config != "" {
		c, err = clicli.NewFromFile(gf.Config)
	} else {
		c, err = clicli.NewFromEnv()
	}
	if err != nil {
		fmt.Fprintln(os.Stderr, "error:", err)
		os.Exit(1)
	}
	if gf != nil {
		c.Verbose = gf.Verbose
	}
	return c
}

func renderOpts(data any, opts output.RenderOpts) {
	_ = output.RenderWithOpts(os.Stdout, data, opts)
}

func handleErr(err error) int {
	if err != nil {
		fmt.Fprintln(os.Stderr, "error:", err)
		return 1
	}
	return 0
}

func projects(args []string) int {
	if len(args) == 0 {
		fmt.Fprintln(os.Stderr, "uso: domain projects <ls|get|create>")
		return 2
	}
	gf, args := parseGlobalFlags(args)
	c := newClient(gf)
	switch args[0] {
	case "ls", "list":
		data, err := c.Do("GET", "/projects", nil, nil)
		if err != nil {
			return handleErr(err)
		}
		if gf.Quiet {
			return 0
		}
		renderOpts(data, output.RenderOpts{Format: output.Format(gf.Format), NoHeaders: gf.NoHeaders})
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
		if gf.Quiet {
			return 0
		}
		renderOpts(data, output.RenderOpts{Format: output.Format(gf.Format), NoHeaders: gf.NoHeaders})
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
		if gf.Quiet {
			return 0
		}
		renderOpts(data, output.RenderOpts{Format: output.Format(gf.Format), NoHeaders: gf.NoHeaders})
		return 0
	}
	fmt.Fprintf(os.Stderr, "acción desconocida: %s\n", args[0])
	return 2
}

func observations(args []string) int {
	if len(args) == 0 {
		fmt.Fprintln(os.Stderr, "uso: domain obs <ls|save>")
		return 2
	}
	gf, args := parseGlobalFlags(args)
	c := newClient(gf)

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
		if gf.Quiet {
			return 0
		}
		renderOpts(data, output.RenderOpts{Format: output.Format(gf.Format), NoHeaders: gf.NoHeaders})
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
		if gf.Quiet {
			return 0
		}
		renderOpts(data, output.RenderOpts{Format: output.Format(gf.Format), NoHeaders: gf.NoHeaders})
		return 0
	}
	fmt.Fprintf(os.Stderr, "acción desconocida: %s\n", args[0])
	return 2
}

func agents(args []string) int {
	if len(args) == 0 {
		fmt.Fprintln(os.Stderr, "uso: domain agents <ls|get|run>")
		return 2
	}
	gf, args := parseGlobalFlags(args)
	c := newClient(gf)
	switch args[0] {
	case "ls":
		data, err := c.Do("GET", "/agents", nil, nil)
		if err != nil {
			return handleErr(err)
		}
		if gf.Quiet {
			return 0
		}
		renderOpts(data, output.RenderOpts{Format: output.Format(gf.Format), NoHeaders: gf.NoHeaders})
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
		if gf.Quiet {
			return 0
		}
		renderOpts(data, output.RenderOpts{Format: output.Format(gf.Format), NoHeaders: gf.NoHeaders})
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
		if gf.Quiet {
			return 0
		}
		renderOpts(data, output.RenderOpts{Format: output.Format(gf.Format), NoHeaders: gf.NoHeaders})
		return 0
	}
	fmt.Fprintf(os.Stderr, "acción desconocida: %s\n", args[0])
	return 2
}

func flows(args []string) int {
	if len(args) == 0 {
		fmt.Fprintln(os.Stderr, "uso: domain flows <ls|run>")
		return 2
	}
	gf, args := parseGlobalFlags(args)
	c := newClient(gf)
	switch args[0] {
	case "ls":
		data, err := c.Do("GET", "/flows", nil, nil)
		if err != nil {
			return handleErr(err)
		}
		if gf.Quiet {
			return 0
		}
		renderOpts(data, output.RenderOpts{Format: output.Format(gf.Format), NoHeaders: gf.NoHeaders})
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
		if gf.Quiet {
			return 0
		}
		renderOpts(data, output.RenderOpts{Format: output.Format(gf.Format), NoHeaders: gf.NoHeaders})
		return 0
	}
	return 2
}

func skills(args []string) int {
	gf, args := parseGlobalFlags(args)
	c := newClient(gf)
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
		if gf.Quiet {
			return 0
		}
		renderOpts(data, output.RenderOpts{Format: output.Format(gf.Format), NoHeaders: gf.NoHeaders})
		return 0
	}
	if args[0] == "metrics" {
		return skillMetrics(gf, c, args[1:])
	}
	if args[0] == "suggest" {
		return skillSuggest(gf, c, args[1:])
	}
	if args[0] == "ab-test" {
		return skillABTest(gf, c, args[1:])
	}
	return 2
}

// skillABTest maneja `domain skill ab-test <start|results|stop>` (HU-52.4).
//
//   - start:   crea+arranca un experimento A/B (POST /skill-ab-tests).
//   - results: muestra los agregados de ambas variantes (GET /skill-ab-tests/<id>/results).
//   - stop:    cancela el experimento (POST /skill-ab-tests/<id>/stop).
func skillABTest(gf *globalFlags, c *clicli.Client, args []string) int {
	if len(args) == 0 {
		fmt.Fprintln(os.Stderr, "uso: domain skill ab-test <start|results|stop>")
		return 2
	}
	render := func(data any) int {
		if gf.Quiet {
			return 0
		}
		renderOpts(data, output.RenderOpts{Format: output.Format(gf.Format), NoHeaders: gf.NoHeaders})
		return 0
	}
	switch args[0] {
	case "start":
		fs := flag.NewFlagSet("ab-test start", flag.ContinueOnError)
		slug := fs.String("slug", "", "skill slug")
		versionA := fs.Int("version-a", 0, "version A (skill_versions)")
		versionB := fs.Int("version-b", 0, "version B (skill_versions)")
		split := fs.Float64("traffic-split-a", 0.50, "fraccion de trafico a la variante A (0..1)")
		minInv := fs.Int("min-invocations", 100, "minimo de invocaciones por variante")
		autoApply := fs.Bool("auto-apply", false, "pinear la version ganadora automaticamente")
		_ = fs.Parse(args[1:])
		if *slug == "" || *versionA == 0 || *versionB == 0 {
			fmt.Fprintln(os.Stderr, "uso: domain skill ab-test start --slug=X --version-a=N --version-b=M [--min-invocations=100]")
			return 2
		}
		body := map[string]any{
			"skill_slug":        *slug,
			"version_a":         *versionA,
			"version_b":         *versionB,
			"traffic_split_a":   *split,
			"min_invocations":   *minInv,
			"auto_apply_winner": *autoApply,
			"start_now":         true,
		}
		data, err := c.Do("POST", "/skill-ab-tests", body, nil)
		if err != nil {
			return handleErr(err)
		}
		return render(data)
	case "results":
		fs := flag.NewFlagSet("ab-test results", flag.ContinueOnError)
		id := fs.String("id", "", "ab test id")
		_ = fs.Parse(args[1:])
		if *id == "" {
			fmt.Fprintln(os.Stderr, "uso: domain skill ab-test results --id=...")
			return 2
		}
		data, err := c.Do("GET", "/skill-ab-tests/"+*id+"/results", nil, nil)
		if err != nil {
			return handleErr(err)
		}
		return render(data)
	case "stop":
		fs := flag.NewFlagSet("ab-test stop", flag.ContinueOnError)
		id := fs.String("id", "", "ab test id")
		_ = fs.Parse(args[1:])
		if *id == "" {
			fmt.Fprintln(os.Stderr, "uso: domain skill ab-test stop --id=...")
			return 2
		}
		data, err := c.Do("POST", "/skill-ab-tests/"+*id+"/stop", nil, nil)
		if err != nil {
			return handleErr(err)
		}
		return render(data)
	}
	fmt.Fprintf(os.Stderr, "accion desconocida: ab-test %s\n", args[0])
	return 2
}

// skillSuggest maneja `domain skill suggest <run|list>` (HU-52.3).
//
//   - run:  corre el batch del judge manualmente (POST /skill-suggestions/run).
//           SOLO persiste pending; jamas aplica (regla dura 6).
//   - list: lista sugerencias (default status=pending). Filtros skill/kind/status.
func skillSuggest(gf *globalFlags, c *clicli.Client, args []string) int {
	if len(args) == 0 {
		fmt.Fprintln(os.Stderr, "uso: domain skill suggest <run|list>")
		return 2
	}
	render := func(data any) int {
		if gf.Quiet {
			return 0
		}
		renderOpts(data, output.RenderOpts{Format: output.Format(gf.Format), NoHeaders: gf.NoHeaders})
		return 0
	}
	switch args[0] {
	case "run":
		data, err := c.Do("POST", "/skill-suggestions/run", nil, nil)
		if err != nil {
			return handleErr(err)
		}
		return render(data)
	case "list":
		fs := flag.NewFlagSet("suggest list", flag.ContinueOnError)
		status := fs.String("status", "pending", "estado (pending|approved|rejected|applied)")
		kind := fs.String("kind", "", "tipo (split|merge|refine|archive)")
		slug := fs.String("skill", "", "skill slug")
		limit := fs.Int("limit", 50, "limite")
		_ = fs.Parse(args[1:])
		q := map[string]string{"limit": fmt.Sprint(*limit)}
		if *status != "" {
			q["status"] = *status
		}
		if *kind != "" {
			q["kind"] = *kind
		}
		if *slug != "" {
			q["skill_slug"] = *slug
		}
		data, err := c.Do("GET", "/skill-suggestions", nil, q)
		if err != nil {
			return handleErr(err)
		}
		return render(data)
	}
	fmt.Fprintf(os.Stderr, "accion desconocida: suggest %s\n", args[0])
	return 2
}

// skillMetrics maneja `domain skill metrics <show|top-failed|slowest>` (HU-52.2).
func skillMetrics(gf *globalFlags, c *clicli.Client, args []string) int {
	if len(args) == 0 {
		fmt.Fprintln(os.Stderr, "uso: domain skill metrics <show|top-failed|slowest>")
		return 2
	}
	render := func(data any) int {
		if gf.Quiet {
			return 0
		}
		renderOpts(data, output.RenderOpts{Format: output.Format(gf.Format), NoHeaders: gf.NoHeaders})
		return 0
	}
	switch args[0] {
	case "show":
		fs := flag.NewFlagSet("metrics show", flag.ContinueOnError)
		slug := fs.String("slug", "", "skill slug")
		days := fs.Int("days", 7, "ventana en dias")
		_ = fs.Parse(args[1:])
		if *slug == "" {
			fmt.Fprintln(os.Stderr, "uso: domain skill metrics show --slug=X [--days=7]")
			return 2
		}
		q := map[string]string{"days": fmt.Sprint(*days)}
		data, err := c.Do("GET", "/skills/"+*slug+"/metrics", nil, q)
		if err != nil {
			return handleErr(err)
		}
		return render(data)
	case "top-failed":
		fs := flag.NewFlagSet("metrics top-failed", flag.ContinueOnError)
		days := fs.Int("days", 7, "ventana en dias")
		limit := fs.Int("limit", 10, "top N")
		_ = fs.Parse(args[1:])
		q := map[string]string{"days": fmt.Sprint(*days), "limit": fmt.Sprint(*limit)}
		data, err := c.Do("GET", "/skills/metrics/top-failed", nil, q)
		if err != nil {
			return handleErr(err)
		}
		return render(data)
	case "slowest":
		fs := flag.NewFlagSet("metrics slowest", flag.ContinueOnError)
		days := fs.Int("days", 7, "ventana en dias")
		limit := fs.Int("limit", 10, "top N")
		_ = fs.Parse(args[1:])
		q := map[string]string{"days": fmt.Sprint(*days), "limit": fmt.Sprint(*limit)}
		data, err := c.Do("GET", "/skills/metrics/slowest", nil, q)
		if err != nil {
			return handleErr(err)
		}
		return render(data)
	}
	fmt.Fprintf(os.Stderr, "accion desconocida: metrics %s\n", args[0])
	return 2
}

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
	c := newClient(gf)
	data, err := c.Do("GET", "/search", nil, q)
	if err != nil {
		return handleErr(err)
	}
	if gf.Quiet {
		return 0
	}
	renderOpts(data, output.RenderOpts{Format: output.Format(gf.Format), NoHeaders: gf.NoHeaders})
	return 0
}

func contextCmd(args []string) int {
	gf, args := parseGlobalFlags(args)
	fs := flag.NewFlagSet("context", flag.ContinueOnError)
	project := fs.String("project", "", "project slug")
	_ = fs.Parse(args)
	q := map[string]string{}
	if *project != "" {
		q["project_slug"] = *project
	}
	c := newClient(gf)
	data, err := c.Do("GET", "/context", nil, q)
	if err != nil {
		return handleErr(err)
	}
	if gf.Quiet {
		return 0
	}
	renderOpts(data, output.RenderOpts{Format: output.Format(gf.Format), NoHeaders: gf.NoHeaders})
	return 0
}
