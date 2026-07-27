package main

import (
	"fmt"
	"path/filepath"
	"sort"
)

// checkAgentCatalog compara el catálogo embebido contra lo instalado en disco. Devuelve
// cuántos chequeos CRÍTICOS fallaron.
//
// El criterio de qué es crítico sale de lo que el propio doctor le dice al usuario al final
// ("re-corré el install para reparar"): un agente ausente o desactualizado lo repara esa
// acción, así que es crítico. Un agente que el usuario editó NO lo repara — re-correr el
// install lo va a dejar igual, porque respetar esa edición es el comportamiento correcto —,
// así que se informa y no suma.
func checkAgentCatalog(paths Paths, cat []agentTemplate) int {
	step("Catálogo de agentes")

	manifiesto := cargarManifiesto(paths.AgentsManifest)
	var faltantes, desactualizados, editados []string

	for _, a := range cat {
		destino := filepath.Join(paths.GlobalAgentsDir, a.slug+".md")
		hashActual, err := fileSHA256(destino)
		if err != nil {
			faltantes = append(faltantes, a.slug)
			continue
		}
		if hashActual == sha256Hex(plantillaResuelta(paths, a)) {
			continue
		}
		if hashRegistrado, hay := manifiesto[a.slug]; hay && hashActual != hashRegistrado {
			editados = append(editados, a.slug)
			continue
		}
		desactualizados = append(desactualizados, a.slug)
	}

	return reportarCatalogo(len(cat), faltantes, desactualizados, editados)
}

func reportarCatalogo(total int, faltantes, desactualizados, editados []string) int {
	sort.Strings(faltantes)
	sort.Strings(desactualizados)
	sort.Strings(editados)

	if len(faltantes) == 0 && len(desactualizados) == 0 && len(editados) == 0 {
		ok(fmt.Sprintf("%d agentes instalados y al día", total))
		return 0
	}
	for _, slug := range faltantes {
		failL("falta el agente " + slug + " — re-corré el install")
	}
	for _, slug := range desactualizados {
		failL("el agente " + slug + " está desactualizado — re-corré el install")
	}
	for _, slug := range editados {
		warnL("el agente " + slug + " difiere de lo instalado por domain: se respeta tu edición")
	}
	return len(faltantes) + len(desactualizados)
}
