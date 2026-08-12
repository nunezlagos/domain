package main

import (
	"os"
	"path/filepath"
	"sort"
	"strings"
)

// claudeProfile es un directorio de configuración de Claude Code. Normalmente hay uno
// (~/.claude), pero quien usa varias cuentas tiene uno por cuenta y los alterna con
// CLAUDE_CONFIG_DIR (DOMAINSERV-279).
type claudeProfile struct {
	Path   string
	Active bool
	Exists bool
}

// sufijosNoPerfil son marcas de copia o de escritura interrumpida. El filtro por tipo no
// alcanza: un directorio de backup también es un directorio, y escribir la config ahí es
// peor que no escribirla — queda una copia que parece válida.
var sufijosNoPerfil = []string{".backup", ".bak", ".tmp", ".old", ".orig", ".save"}

// esNombreDePerfil descarta backups y temporales. Se mira por CONTENCIÓN y no por sufijo
// final porque los nombres reales llevan datos pegados atrás: el caso medido en la máquina
// del usuario es `.claude.json.tmp.107155.3bcf6ac7c43e`, donde ".tmp" queda en el medio.
func esNombreDePerfil(base string) bool {
	if !strings.HasPrefix(base, ".claude") {
		return false
	}
	lower := strings.ToLower(base)
	for _, s := range sufijosNoPerfil {
		if strings.Contains(lower, s) {
			return false
		}
	}
	return true
}

// detectClaudeProfiles lista los config dirs de Claude Code del home. envConfigDir es el
// valor de CLAUDE_CONFIG_DIR (vacío si no está seteada).
//
// Tres reglas que vienen de mediciones, no de gusto:
//   - solo DIRECTORIOS, y descartando backups/temporales por nombre. En la máquina del
//     usuario el glob ~/.claude* devuelve 13 entradas y 11 no son perfiles.
//   - ~/.claude entra SIEMPRE, exista o no: es el default del binario
//     (${CLAUDE_CONFIG_DIR:-$HOME/.claude}) y en una instalación limpia es el que hay que
//     crear.
//   - el perfil de CLAUDE_CONFIG_DIR entra aunque el glob no lo encuentre: puede vivir
//     fuera del home o llamarse distinto, y es el que la sesión está usando de verdad.
//
// El activo va primero y el resto ordenado, para que la lista que se confirma no cambie
// de orden entre corridas.
func detectClaudeProfiles(home, envConfigDir string) []claudeProfile {
	def := filepath.Join(home, ".claude")
	activo := def
	if envConfigDir != "" {
		activo = filepath.Clean(envConfigDir)
	}

	encontrados := map[string]bool{def: true, activo: true}
	if entradas, err := os.ReadDir(home); err == nil {
		for _, e := range entradas {
			if e.IsDir() && esNombreDePerfil(e.Name()) {
				encontrados[filepath.Join(home, e.Name())] = true
			}
		}
	}

	paths := make([]string, 0, len(encontrados))
	for p := range encontrados {
		paths = append(paths, p)
	}
	sort.Strings(paths)

	perfiles := make([]claudeProfile, 0, len(paths))
	for _, p := range paths {
		info, err := os.Stat(p)
		perfiles = append(perfiles, claudeProfile{
			Path:   p,
			Active: p == activo,
			Exists: err == nil && info.IsDir(),
		})
	}
	sort.SliceStable(perfiles, func(i, j int) bool { return perfiles[i].Active && !perfiles[j].Active })
	return perfiles
}
