package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"sort"
)

// manifiestoAgentes mapea slug -> sha256 del template que domain escribió la última vez. Es
// lo que hace distinguible "el usuario editó el agente" de "domain cambió el template": sin
// procedencia, las dos situaciones se ven idénticas en disco.
type manifiestoAgentes map[string]string

// procedencia son los tres estados que el manifest permite resolver.
type procedencia int

const (
	// alDia: el disco ya tiene el template vigente
	alDia procedencia = iota
	// deDomain: el archivo es el que domain escribió, así que actualizarlo es correcto
	deDomain
	// delUsuario: el archivo no coincide con nada que domain haya escrito
	delUsuario
)

// clasificar resuelve la procedencia del archivo en disco. hashRegistrado vacío significa que
// no hay registro previo: se ADOPTA el archivo como propio en vez de reportar conflicto,
// porque toda máquina instalada antes del manifest lo vería como ajeno y el upgrade que
// introduce esta mejora arrancaría marcando un conflicto por agente.
func clasificar(destino string, contenido []byte, hashRegistrado string) procedencia {
	hashActual, err := fileSHA256(destino)
	if err != nil {
		return deDomain
	}
	if hashActual == sha256Hex(contenido) {
		return alDia
	}
	if hashRegistrado == "" || hashActual == hashRegistrado {
		return deDomain
	}
	return delUsuario
}

// podarRetirados borra los agentes que domain instaló y ya no están en el catálogo. El
// manifest es lo que hace esto seguro: un agente que el usuario puso a mano nunca tuvo
// entrada, así que queda fuera del alcance sin necesidad de adivinar de quién es cada
// archivo. Devuelve los slugs removidos y los que se dejaron por estar editados.
func podarRetirados(paths Paths, cat []agentTemplate, manifiesto manifiestoAgentes) (removidos, editados []string) {
	enCatalogo := make(map[string]bool, len(cat))
	for _, a := range cat {
		enCatalogo[a.slug] = true
	}

	for slug, hashRegistrado := range manifiesto {
		if enCatalogo[slug] {
			continue
		}
		destino := filepath.Join(paths.GlobalAgentsDir, slug+".md")
		// una edición no se descarta por un cambio de catálogo: misma semántica que en la
		// instalación, se reporta y se deja
		if hashActual, err := fileSHA256(destino); err == nil && hashActual != hashRegistrado {
			editados = append(editados, slug)
			continue
		}
		_ = os.Remove(destino)
		if paths.OpencodeAgentsDir != "" {
			_ = os.Remove(filepath.Join(paths.OpencodeAgentsDir, slug+".md"))
		}
		delete(manifiesto, slug)
		removidos = append(removidos, slug)
	}
	sort.Strings(removidos)
	sort.Strings(editados)
	return removidos, editados
}

// cargarManifiesto devuelve el registro, o vacío si no existe o no se puede leer. Un manifest
// ilegible degrada a adopción, no a conflicto: preferimos actualizar el catálogo que dejar al
// usuario con agentes viejos por un archivo de estado corrupto.
func cargarManifiesto(ruta string) manifiestoAgentes {
	if ruta == "" {
		return manifiestoAgentes{}
	}
	b, err := os.ReadFile(ruta)
	if err != nil {
		return manifiestoAgentes{}
	}
	var m manifiestoAgentes
	if err := json.Unmarshal(b, &m); err != nil {
		return manifiestoAgentes{}
	}
	if m == nil {
		return manifiestoAgentes{}
	}
	return m
}

// guardarManifiesto persiste el registro. Se llama una vez al final de la instalación, no por
// agente, para no dejar el archivo a medias si algo falla en el medio.
func guardarManifiesto(ruta string, m manifiestoAgentes) error {
	if ruta == "" {
		return nil
	}
	if err := os.MkdirAll(filepath.Dir(ruta), 0o755); err != nil {
		return err
	}
	b, err := json.MarshalIndent(m, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(ruta, append(b, '\n'), 0o644)
}
