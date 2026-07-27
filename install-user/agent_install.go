package main

import (
	"bytes"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// resultadoAgentes describe qué pasó con cada agente. Las omisiones se reportan porque una
// ausencia silenciosa se lee como cobertura: es el modo de falla que dejó pasar la
// regresión de DOMAINSERV-135.
type resultadoAgentes struct {
	instalados       []string
	opencode         []string
	omitidosOpencode []string
	conflictos       []string
	guardsInstalados []string
	guardsFaltantes  []string
	dirCreado        bool
}

// prepararDirectorios crea los destinos y marca si el de agentes no existía: el watcher del
// harness solo cubre directorios que ya estaban al arrancar la sesión, así que crearlo ahora
// implica que esta sesión no descubre los agentes hasta reiniciar.
func prepararDirectorios(paths Paths, res *resultadoAgentes) error {
	if _, err := os.Stat(paths.GlobalAgentsDir); os.IsNotExist(err) {
		res.dirCreado = true
	}
	for _, dir := range []string{paths.GlobalAgentsDir, paths.AgentHooksDir, paths.OpencodeAgentsDir} {
		if dir == "" {
			continue
		}
		if err := os.MkdirAll(dir, 0o755); err != nil {
			return err
		}
	}
	return nil
}

// instalarAgentes escribe el catálogo en Claude Code y, para los agentes que tengan
// variante, también en OpenCode. Idempotente: reinstalar lo mismo no reporta conflicto.
func instalarAgentes(paths Paths, cat []agentTemplate) (resultadoAgentes, error) {
	var res resultadoAgentes

	if err := prepararDirectorios(paths, &res); err != nil {
		return res, err
	}
	manifiesto := cargarManifiesto(paths.AgentsManifest)

	for _, a := range cat {
		// un guard que no resuelve deja ABIERTO el tool que venía a acotar, así que el
		// agente no se instala en vez de instalarse sin protección
		tpl, guards, err := resolverGuards(paths, a)
		if err != nil {
			res.guardsFaltantes = append(res.guardsFaltantes, a.slug)
			continue
		}
		res.guardsInstalados = append(res.guardsInstalados, guards...)

		destino := filepath.Join(paths.GlobalAgentsDir, a.slug+".md")
		// un conflicto es información, no una excepción: se reporta y se sigue con el resto
		// del catálogo en vez de abortar la instalación entera
		if clasificar(destino, tpl, manifiesto[a.slug]) == delUsuario {
			res.conflictos = append(res.conflictos, a.slug)
			continue
		}
		if err := escribirTemplate(destino, tpl); err != nil {
			return res, err
		}
		manifiesto[a.slug] = sha256Hex(tpl)
		res.instalados = append(res.instalados, a.slug)

		if paths.OpencodeAgentsDir == "" {
			continue
		}
		if len(a.opencode) == 0 {
			res.omitidosOpencode = append(res.omitidosOpencode, a.slug)
			continue
		}
		if err := escribirTemplate(filepath.Join(paths.OpencodeAgentsDir, a.slug+".md"), a.opencode); err != nil {
			return res, err
		}
		res.opencode = append(res.opencode, a.slug)
	}

	// se persiste una vez al final, no por agente: si algo falla en el medio, el manifest no
	// queda declarando agentes que no se escribieron
	if err := guardarManifiesto(paths.AgentsManifest, manifiesto); err != nil {
		return res, err
	}
	return res, nil
}

// reportarAgentes deja a la vista qué se instaló y qué NO. Un agente ausente sin aviso se
// lee como cobertura, que es cómo la regresión de DOMAINSERV-135 pasó desapercibida.
func reportarAgentes(paths Paths, res resultadoAgentes) {
	ok(fmt.Sprintf("agentes: %d en %s (%s)", len(res.instalados), paths.GlobalAgentsDir,
		strings.Join(res.instalados, ", ")))
	if len(res.opencode) > 0 {
		ok(fmt.Sprintf("agentes opencode: %d (%s)", len(res.opencode), strings.Join(res.opencode, ", ")))
	}
	if len(res.omitidosOpencode) > 0 {
		warnL("omitidos de opencode por no tener variante .opencode.md: " +
			strings.Join(res.omitidosOpencode, ", "))
		warnL("  su frontmatter de Claude Code sería malformado en OpenCode; instalarlo igual es peor que omitirlo")
	}
	if len(res.guardsInstalados) > 0 {
		ok(fmt.Sprintf("guards: %d en %s (%s)", len(res.guardsInstalados), paths.AgentHooksDir,
			strings.Join(res.guardsInstalados, ", ")))
	}
	for _, slug := range res.guardsFaltantes {
		failL("NO instalado: " + slug + " declara un guard que no vino en el bundle")
		failL("  sin el guard, el tool que ese hook acota queda abierto: instalarlo igual sería peor")
	}
	for _, c := range res.conflictos {
		warnL("sin actualizar (difiere de lo instalado por domain, se respeta tu edición): " + c)
	}
	if res.dirCreado {
		warnL("se creó " + paths.GlobalAgentsDir + " por primera vez: reiniciá la sesión para que el harness lo descubra")
	}
}

// escribirTemplate no escribe si el contenido ya es el mismo, y borra el destino antes de
// crearlo: una instalación previa pudo dejar un symlink ahí, y WriteFile escribiría A
// TRAVÉS de él, pisando el archivo apuntado con el template del otro harness.
func escribirTemplate(destino string, contenido []byte) error {
	if actual, err := os.ReadFile(destino); err == nil && bytes.Equal(actual, contenido) {
		return nil
	}
	_ = os.Remove(destino)
	return os.WriteFile(destino, contenido, 0o644)
}

// escribirEjecutable es escribirTemplate para los guards: sin el bit de ejecución el hook
// no puede correr el script, y un hook que no corre no bloquea nada.
//
// Los guards quedan FUERA del esquema de procedencia a propósito: no son configuración que el
// usuario ajuste, son la restricción que acota un tool. Respetar una edición local acá dejaría
// vigente un guard debilitado sin que nada lo señale, así que el del bundle gana siempre.
func escribirEjecutable(destino string, contenido []byte) error {
	if actual, err := os.ReadFile(destino); err == nil && bytes.Equal(actual, contenido) {
		return os.Chmod(destino, 0o755)
	}
	_ = os.Remove(destino)
	return os.WriteFile(destino, contenido, 0o755)
}

// desinstalarAgentes remueve exactamente lo que instaló el catálogo. Un agente que el
// usuario haya puesto en el mismo directorio no se toca.
func desinstalarAgentes(paths Paths, cat []agentTemplate) {
	manifiesto := cargarManifiesto(paths.AgentsManifest)
	for _, a := range cat {
		_ = os.Remove(filepath.Join(paths.GlobalAgentsDir, a.slug+".md"))
		if paths.OpencodeAgentsDir != "" && len(a.opencode) > 0 {
			_ = os.Remove(filepath.Join(paths.OpencodeAgentsDir, a.slug+".md"))
		}
		// una entrada que sobreviva al archivo haría que la instalación siguiente compare
		// contra el hash de algo que ya no está
		delete(manifiesto, a.slug)
	}
	if len(manifiesto) == 0 {
		_ = os.Remove(paths.AgentsManifest)
	} else {
		_ = guardarManifiesto(paths.AgentsManifest, manifiesto)
	}
	// solo se van si quedaron vacíos, así que un agente ajeno los preserva
	_ = os.Remove(paths.GlobalAgentsDir)
	if paths.OpencodeAgentsDir != "" {
		_ = os.Remove(paths.OpencodeAgentsDir)
	}
}
