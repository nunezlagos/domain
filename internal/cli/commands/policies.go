package commands

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	clicli "nunezlagos/domain/internal/cli/client"
)

// policies implementa: domain policies import-md|export-md
func policies(args []string) int {
	if len(args) == 0 {
		fmt.Fprintln(os.Stderr, "uso: domain policies <import-md|export-md>")
		return 2
	}
	gf, args := parseGlobalFlags(args)
	c := newClient(gf)

	switch args[0] {
	case "import-md":
		return importPoliciesMD(c, gf, args[1:])
	case "export-md":
		return exportPoliciesMD(c, gf, args[1:])
	}
	fmt.Fprintf(os.Stderr, "acción desconocida: %s\n", args[0])
	return 2
}

// importPoliciesMD lee archivos .md de un directorio y los importa como policies.
// Formato: slug y kind se derivan del nombre: <kind>.<slug>.md
// Si el archivo incluye front matter "---\nslug: ...\nkind: ...\n---" se usa eso.
func importPoliciesMD(c *clicli.Client, gf *globalFlags, args []string) int {
	dir := "."
	dryRun := false
	for i := 0; i < len(args); i++ {
		switch args[i] {
		case "--dir":
			if i+1 < len(args) {
				dir = args[i+1]
				i++
			}
		case "--dry-run":
			dryRun = true
		default:
			// primer positional = dir
			if dir == "." {
				dir = args[i]
			}
		}
	}

	entries, err := os.ReadDir(dir)
	if err != nil {
		fmt.Fprintf(os.Stderr, "error leyendo directorio: %v\n", err)
		return 1
	}

	imported := 0
	for _, e := range entries {
		if e.IsDir() || !strings.HasSuffix(e.Name(), ".md") {
			continue
		}
		slug := strings.TrimSuffix(e.Name(), ".md")
		kind := "convention"

		// Detectar kind del nombre: si tiene formato <kind>.<slug>
		if parts := strings.SplitN(slug, ".", 2); len(parts) == 2 {
			kind = parts[0]
			slug = parts[1]
		}

		data, err := os.ReadFile(filepath.Join(dir, e.Name()))
		if err != nil {
			fmt.Fprintf(os.Stderr, "error leyendo %s: %v\n", e.Name(), err)
			continue
		}
		body := string(data)

		// Front matter opcional
		if strings.HasPrefix(body, "---") {
			if idx := strings.Index(body[3:], "\n---"); idx >= 0 {
				fm := body[3 : idx+3]
				body = strings.TrimSpace(body[idx+6:])
				for _, line := range strings.Split(fm, "\n") {
					line = strings.TrimSpace(line)
					if strings.HasPrefix(line, "slug:") {
						slug = strings.TrimSpace(line[5:])
					}
					if strings.HasPrefix(line, "kind:") {
						kind = strings.TrimSpace(line[5:])
					}
				}
			}
		}

		if dryRun {
			fmt.Printf("[dry-run] importar: kind=%s slug=%s file=%s (%d bytes)\n",
				kind, slug, e.Name(), len(body))
			continue
		}

		payload := map[string]any{
			"slug":    slug,
			"name":    policyNameFromSlug(slug),
			"kind":    kind,
			"body_md": body,
		}
		_, err = c.Do("POST", "/platform/policies", payload, nil)
		if err != nil {
			fmt.Fprintf(os.Stderr, "error importando %s: %v\n", e.Name(), err)
			continue
		}
		fmt.Printf("importado: %s (kind=%s slug=%s)\n", e.Name(), kind, slug)
		imported++
	}
	fmt.Printf("\n%d política(s) importada(s) desde %s\n", imported, dir)
	return 0
}

// exportPoliciesMD descarga todas las policies y las escribe como .md files.
func exportPoliciesMD(c *clicli.Client, gf *globalFlags, args []string) int {
	outDir := "."
	for i := 0; i < len(args); i++ {
		switch args[i] {
		case "--dir":
			if i+1 < len(args) {
				outDir = args[i+1]
				i++
			}
		default:
			if outDir == "." {
				outDir = args[i]
			}
		}
	}

	if err := os.MkdirAll(outDir, 0755); err != nil {
		fmt.Fprintf(os.Stderr, "error creando directorio: %v\n", err)
		return 1
	}

	raw, err := c.Do("GET", "/platform/policies", nil, nil)
	if err != nil {
		fmt.Fprintf(os.Stderr, "error obteniendo policies: %v\n", err)
		return 1
	}

	list, ok := raw.([]any)
	if !ok {
		// Puede venir como map con key "data"
		if m, ok := raw.(map[string]any); ok {
			if d, ok := m["data"]; ok {
				list, _ = d.([]any)
			}
		}
	}
	if list == nil {
		fmt.Fprintln(os.Stderr, "error: formato inesperado de respuesta")
		return 1
	}

	exported := 0
	for _, item := range list {
		p, ok := item.(map[string]any)
		if !ok {
			continue
		}
		slug, _ := p["slug"].(string)
		kind, _ := p["kind"].(string)
		bodyMD, _ := p["body_md"].(string)
		name, _ := p["name"].(string)
		if slug == "" {
			continue
		}
		filename := fmt.Sprintf("%s.%s.md", kind, slug)
		if kind == "" {
			filename = slug + ".md"
		}
		content := fmt.Sprintf("---\nslug: %s\nkind: %s\nname: %s\n---\n\n%s\n",
			slug, kind, name, bodyMD)
		if err := os.WriteFile(filepath.Join(outDir, filename), []byte(content), 0644); err != nil {
			fmt.Fprintf(os.Stderr, "error escribiendo %s: %v\n", filename, err)
			continue
		}
		fmt.Printf("exportado: %s\n", filename)
		exported++
	}
	fmt.Printf("\n%d política(s) exportada(s) a %s\n", exported, outDir)
	return 0
}

func policyNameFromSlug(slug string) string {
	name := strings.ReplaceAll(slug, "-", " ")
	name = strings.ReplaceAll(name, "_", " ")
	if len(name) > 0 {
		name = strings.ToUpper(name[:1]) + name[1:]
	}
	return name
}
