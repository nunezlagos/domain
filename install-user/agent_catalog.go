package main

import (
	"fmt"
	"path"
	"sort"
	"strings"
)

// varianteOpencode es el sufijo que marca la variante de un agente para OpenCode. No es un
// agente aparte: el MISMO agente necesita dos templates porque los esquemas de frontmatter
// son incompatibles (ver embed.go).
const varianteOpencode = ".opencode.md"

// agentTemplate es un agente del catálogo con sus templates por harness. opencode queda
// nil cuando el agente no tiene variante, y en ese caso NO se instala en OpenCode: darle
// el template de Claude Code sería un frontmatter malformado, peor que no instalarlo.
type agentTemplate struct {
	slug     string
	claude   []byte
	opencode []byte
	// guards son los scripts PreToolUse que el agente declare, por basename. Van con el
	// agente porque su ausencia no degrada una función: la ABRE. Un hook que apunta a un
	// script inexistente no bloquea nada, y el agente que se creía acotado deja de estarlo.
	guards map[string][]byte
}

// agentCatalog enumera templates/agents/ en orden estable. Antes el installer nombraba un
// archivo a mano (DOMAINSERV-137), así que los agentes nuevos del repo nunca llegaban al
// cliente y había que copiarlos a mano — que es cómo se produjo la regresión de
// DOMAINSERV-135.
func agentCatalog() ([]agentTemplate, error) {
	entries, err := agentsFS.ReadDir(agentsDir)
	if err != nil {
		return nil, fmt.Errorf("leer %s: %w", agentsDir, err)
	}

	variantes := map[string][]byte{}
	var bases []string
	for _, e := range entries {
		nombre := e.Name()
		if e.IsDir() || !strings.HasSuffix(nombre, ".md") {
			continue
		}
		contenido, err := agentsFS.ReadFile(path.Join(agentsDir, nombre))
		if err != nil {
			return nil, fmt.Errorf("leer %s: %w", nombre, err)
		}
		if slug, ok := recortarSufijo(nombre, varianteOpencode); ok {
			variantes[slug] = contenido
			continue
		}
		bases = append(bases, nombre)
	}

	cat := make([]agentTemplate, 0, len(bases))
	for _, nombre := range bases {
		contenido, err := agentsFS.ReadFile(path.Join(agentsDir, nombre))
		if err != nil {
			return nil, fmt.Errorf("leer %s: %w", nombre, err)
		}
		slug := strings.TrimSuffix(nombre, ".md")
		guards, err := guardsDelBundle(contenido)
		if err != nil {
			return nil, err
		}
		cat = append(cat, agentTemplate{
			slug: slug, claude: contenido, opencode: variantes[slug], guards: guards,
		})
	}
	sort.Slice(cat, func(i, j int) bool { return cat[i].slug < cat[j].slug })
	return cat, nil
}

func recortarSufijo(nombre, sufijo string) (string, bool) {
	if !strings.HasSuffix(nombre, sufijo) {
		return "", false
	}
	return strings.TrimSuffix(nombre, sufijo), true
}

// validarNombresUnicos corre ANTES de copiar: dos agentes con el mismo `name` en un
// directorio hacen que Claude Code cargue uno de los dos por orden de lectura del
// filesystem, sin precedencia documentada. Detectarlo después es detectarlo por el
// comportamiento raro de un agente que no es el que se editó.
func validarNombresUnicos(cat []agentTemplate) error {
	vistos := map[string]string{}
	for _, a := range cat {
		nombre := campoDelFrontmatter(a.claude, "name:")
		if nombre == "" {
			continue
		}
		if previo, dup := vistos[nombre]; dup {
			return fmt.Errorf("name duplicado %q en %s.md y %s.md: Claude Code cargaría uno de los dos por orden de lectura del filesystem", nombre, previo, a.slug)
		}
		vistos[nombre] = a.slug
	}
	return nil
}

// campoDelFrontmatter devuelve el valor de un campo de primer nivel del frontmatter, o ""
// si no está. No es un parser YAML: alcanza para name/model, que son escalares en una línea.
func campoDelFrontmatter(tpl []byte, prefijo string) string {
	s := string(tpl)
	if !strings.HasPrefix(s, "---\n") {
		return ""
	}
	fin := strings.Index(s[4:], "\n---")
	if fin < 0 {
		return ""
	}
	for _, linea := range strings.Split(s[4:4+fin], "\n") {
		if strings.HasPrefix(linea, prefijo) {
			return strings.TrimSpace(strings.TrimPrefix(linea, prefijo))
		}
	}
	return ""
}
