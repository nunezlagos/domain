package commands

import (
	"flag"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"

	clicli "nunezlagos/domain/internal/cli/client"
	"nunezlagos/domain/internal/cli/output"
)

// openspec implementa: domain openspec export|status|apply
//
// A DIFERENCIA del round-trip por MCP (donde el LLM escribe/lee los archivos),
// el CLI SÍ toca el filesystem local: export baja el árbol a disco y status/
// apply lo levantan de disco. La lógica de negocio (render/diff/persist) vive
// en el server (Engine), invocada vía /api/v1/openspec/*.
func openspec(args []string) int {
	if len(args) == 0 {
		fmt.Fprintln(os.Stderr, "uso: domain openspec <export|status|apply>")
		return 2
	}
	gf, args := parseGlobalFlags(args)
	c := newClient(gf)
	switch args[0] {
	case "export":
		return openspecExport(c, gf, args[1:])
	case "status":
		return openspecStatus(c, gf, args[1:])
	case "apply":
		return openspecApply(c, gf, args[1:])
	}
	fmt.Fprintf(os.Stderr, "acción desconocida: openspec %s\n", args[0])
	return 2
}

// openspecExport baja las specs activas del proyecto al repo. El server devuelve
// changes[].files = {path: contenido}; el CLI escribe cada path bajo --out.
func openspecExport(c *clicli.Client, gf *globalFlags, args []string) int {
	fs := flag.NewFlagSet("openspec export", flag.ContinueOnError)
	project := fs.String("project", "", "project slug (requerido)")
	scope := fs.String("scope", "active", "active | all")
	out := fs.String("out", ".", "directorio raíz donde escribir el árbol")
	_ = fs.Parse(args)
	if *project == "" {
		fmt.Fprintln(os.Stderr, "uso: domain openspec export --project <slug> [--scope active|all] [--out .]")
		return 2
	}
	q := map[string]string{"project_slug": *project, "scope": *scope}
	data, err := c.Do("GET", "/openspec/export", nil, q)
	if err != nil {
		return handleErr(err)
	}
	changes := changesFromData(data)
	written := 0
	for _, ch := range changes {
		files, _ := ch["files"].(map[string]any)
		for path, body := range files {
			content, _ := body.(string)
			dest := filepath.Join(*out, filepath.FromSlash(path))
			if err := os.MkdirAll(filepath.Dir(dest), 0o755); err != nil {
				fmt.Fprintf(os.Stderr, "error creando dir %s: %v\n", filepath.Dir(dest), err)
				return 1
			}
			if err := os.WriteFile(dest, []byte(content), 0o644); err != nil {
				fmt.Fprintf(os.Stderr, "error escribiendo %s: %v\n", dest, err)
				return 1
			}
			written++
		}
	}
	if gf.Quiet {
		return 0
	}
	fmt.Printf("%d change(s), %d archivo(s) escritos en %s\n", len(changes), written, *out)
	return 0
}

// openspecStatus muestra el drift repo↔DB. Lee el árbol local y lo manda al
// server para comparar hashes.
func openspecStatus(c *clicli.Client, gf *globalFlags, args []string) int {
	dir, _ := openspecDirFlag(args)
	files, err := collectOpenspecFiles(dir)
	if err != nil {
		fmt.Fprintf(os.Stderr, "error leyendo %s: %v\n", dir, err)
		return 1
	}
	if len(files) == 0 {
		fmt.Fprintf(os.Stderr, "no se encontraron archivos openspec en %s (¿corriste export?)\n", dir)
		return 1
	}
	data, err := c.Do("POST", "/openspec/status", map[string]any{"files": files}, nil)
	if err != nil {
		return handleErr(err)
	}
	if gf.Quiet {
		return 0
	}
	renderOpts(changesPayload(data), output.RenderOpts{Format: output.Format(gf.Format), NoHeaders: gf.NoHeaders})
	return 0
}

// openspecApply sube los .md editados a la DB. Lee el árbol local y lo persiste.
func openspecApply(c *clicli.Client, gf *globalFlags, args []string) int {
	dir, force := openspecDirFlag(args)
	files, err := collectOpenspecFiles(dir)
	if err != nil {
		fmt.Fprintf(os.Stderr, "error leyendo %s: %v\n", dir, err)
		return 1
	}
	if len(files) == 0 {
		fmt.Fprintf(os.Stderr, "no se encontraron archivos openspec en %s (¿corriste export?)\n", dir)
		return 1
	}
	body := map[string]any{"files": files, "force": force}
	data, err := c.Do("POST", "/openspec/apply", body, nil)
	if err != nil {
		return handleErr(err)
	}
	if gf.Quiet {
		return 0
	}
	renderOpts(changesPayload(data), output.RenderOpts{Format: output.Format(gf.Format), NoHeaders: gf.NoHeaders})
	return 0
}

// openspecDirFlag parsea el dir posicional (default ".") y el flag --force.
func openspecDirFlag(args []string) (string, bool) {
	dir := "."
	force := false
	for i := 0; i < len(args); i++ {
		switch args[i] {
		case "--force":
			force = true
		case "--dir":
			if i+1 < len(args) {
				dir = args[i+1]
				i++
			}
		default:
			if !strings.HasPrefix(args[i], "-") {
				dir = args[i]
			}
		}
	}
	return dir, force
}

// collectOpenspecFiles camina dir y junta los .md y .openspec.yaml como
// {path, content}, con path relativo a dir (matchea los paths que emite export).
func collectOpenspecFiles(dir string) ([]map[string]string, error) {
	var out []map[string]string
	err := filepath.WalkDir(dir, func(p string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			return nil
		}
		name := d.Name()
		if name != ".openspec.yaml" && !strings.HasSuffix(name, ".md") {
			return nil
		}
		rel, rerr := filepath.Rel(dir, p)
		if rerr != nil {
			return rerr
		}
		content, rerr := os.ReadFile(p)
		if rerr != nil {
			return rerr
		}
		out = append(out, map[string]string{
			"path":    filepath.ToSlash(rel),
			"content": string(content),
		})
		return nil
	})
	return out, err
}

// changesFromData extrae data.changes como []map[string]any.
func changesFromData(data any) []map[string]any {
	m, _ := data.(map[string]any)
	raw, _ := m["changes"].([]any)
	out := make([]map[string]any, 0, len(raw))
	for _, r := range raw {
		if c, ok := r.(map[string]any); ok {
			out = append(out, c)
		}
	}
	return out
}

// changesPayload devuelve el array changes (para render tabular) o el data crudo.
func changesPayload(data any) any {
	if m, ok := data.(map[string]any); ok {
		if ch, ok := m["changes"]; ok {
			return ch
		}
	}
	return data
}
