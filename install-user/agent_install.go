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
	dirCreado        bool
}

// instalarAgentes escribe el catálogo en Claude Code y, para los agentes que tengan
// variante, también en OpenCode. Idempotente: reinstalar lo mismo no reporta conflicto.
func instalarAgentes(paths Paths, cat []agentTemplate) (resultadoAgentes, error) {
	var res resultadoAgentes

	// el watcher del harness solo cubre directorios que existían al arrancar la sesión: si
	// lo creamos ahora, esta sesión no descubre los agentes hasta reiniciar
	if _, err := os.Stat(paths.GlobalAgentsDir); os.IsNotExist(err) {
		res.dirCreado = true
	}
	if err := os.MkdirAll(paths.GlobalAgentsDir, 0o755); err != nil {
		return res, err
	}
	if paths.OpencodeAgentsDir != "" {
		if err := os.MkdirAll(paths.OpencodeAgentsDir, 0o755); err != nil {
			return res, err
		}
	}

	for _, a := range cat {
		destino := filepath.Join(paths.GlobalAgentsDir, a.slug+".md")
		if err := escribirTemplate(destino, a.claude); err != nil {
			return res, err
		}
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

// desinstalarAgentes remueve exactamente lo que instaló el catálogo. Un agente que el
// usuario haya puesto en el mismo directorio no se toca.
func desinstalarAgentes(paths Paths, cat []agentTemplate) {
	for _, a := range cat {
		_ = os.Remove(filepath.Join(paths.GlobalAgentsDir, a.slug+".md"))
		if paths.OpencodeAgentsDir != "" && len(a.opencode) > 0 {
			_ = os.Remove(filepath.Join(paths.OpencodeAgentsDir, a.slug+".md"))
		}
	}
	// solo se van si quedaron vacíos, así que un agente ajeno los preserva
	_ = os.Remove(paths.GlobalAgentsDir)
	if paths.OpencodeAgentsDir != "" {
		_ = os.Remove(paths.OpencodeAgentsDir)
	}
}
