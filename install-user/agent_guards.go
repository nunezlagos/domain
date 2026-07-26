package main

import (
	"fmt"
	"path"
	"path/filepath"
	"strings"
)

// guardsDir es donde viven los scripts PreToolUse del catálogo, dentro del binario.
const guardsDir = "templates/agents-hooks"

// guardsDeclarados devuelve el basename de cada script que el frontmatter referencia en un
// hook. Se queda con el basename porque la ruta del template es relativa y no sirve: se
// resuelve contra el cwd, así que en ~/.claude/agents/ apunta a la nada. La ruta real la
// escribe la instalación.
func guardsDeclarados(tpl []byte) []string {
	s := string(tpl)
	if !strings.HasPrefix(s, "---\n") {
		return nil
	}
	fin := strings.Index(s[4:], "\n---")
	if fin < 0 {
		return nil
	}
	var out []string
	for _, linea := range strings.Split(s[4:4+fin], "\n") {
		linea = strings.TrimSpace(linea)
		if !strings.HasPrefix(linea, "command:") {
			continue
		}
		ruta := strings.Trim(strings.TrimSpace(strings.TrimPrefix(linea, "command:")), `"'`)
		if ruta == "" {
			continue
		}
		out = append(out, path.Base(ruta))
	}
	return out
}

// guardsDelBundle carga los scripts que el agente declara. Un guard declarado que no está
// en el bundle NO es un error acá: lo detecta la instalación, que es la que puede decidir
// no instalar ese agente sin abortar el resto.
func guardsDelBundle(tpl []byte) (map[string][]byte, error) {
	declarados := guardsDeclarados(tpl)
	if len(declarados) == 0 {
		return nil, nil
	}
	out := map[string][]byte{}
	for _, nombre := range declarados {
		contenido, err := agentsFS.ReadFile(path.Join(guardsDir, nombre))
		if err != nil {
			continue
		}
		out[nombre] = contenido
	}
	return out, nil
}

// resolverGuards copia los guards del agente y devuelve su template con las referencias
// reescritas a ruta absoluta. Si falta alguno devuelve error: el llamador NO instala ese
// agente, porque un hook que no resuelve deja abierto justo el tool que venía a acotar.
func resolverGuards(paths Paths, a agentTemplate) ([]byte, []string, error) {
	declarados := guardsDeclarados(a.claude)
	if len(declarados) == 0 {
		return a.claude, nil, nil
	}
	if paths.AgentHooksDir == "" {
		return nil, nil, fmt.Errorf("%s declara guards pero no hay directorio de hooks", a.slug)
	}

	tpl := string(a.claude)
	var instalados []string
	for _, nombre := range declarados {
		contenido, ok := a.guards[nombre]
		if !ok || len(contenido) == 0 {
			return nil, nil, fmt.Errorf("%s declara el guard %q y no viene en el bundle", a.slug, nombre)
		}
		destino := filepath.Join(paths.AgentHooksDir, nombre)
		if err := escribirEjecutable(destino, contenido); err != nil {
			return nil, nil, err
		}
		tpl = reescribirReferencia(tpl, nombre, destino)
		instalados = append(instalados, nombre)
	}
	return []byte(tpl), instalados, nil
}

// reescribirReferencia reemplaza la ruta que el template declara por la absoluta del guard
// instalado, matcheando por basename para no depender de cómo la escribió el template.
func reescribirReferencia(tpl, nombre, destino string) string {
	var out []string
	for _, linea := range strings.Split(tpl, "\n") {
		recortada := strings.TrimSpace(linea)
		if strings.HasPrefix(recortada, "command:") && strings.Contains(recortada, nombre) {
			sangria := linea[:len(linea)-len(strings.TrimLeft(linea, " \t"))]
			out = append(out, fmt.Sprintf("%scommand: %q", sangria, destino))
			continue
		}
		out = append(out, linea)
	}
	return strings.Join(out, "\n")
}
