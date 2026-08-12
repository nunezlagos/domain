package main

import (
	"fmt"
	"path/filepath"
)

// profileResolveOpts son las entradas que deciden en qué perfiles se escribe.
type profileResolveOpts struct {
	// Explicit son los paths de --claude-config-dir (repetible). Si hay alguno, gana.
	Explicit []string
	Yes      bool
	TTY      bool
	// EnvConfigDir es CLAUDE_CONFIG_DIR, solo para poder avisar cuando --yes la ignora.
	EnvConfigDir string
	// Confirm muestra los perfiles y devuelve los que el usuario aprobó. Solo se invoca
	// con TTY y sin overrides.
	Confirm func([]claudeProfile) []claudeProfile
}

// profileSelection es el resultado: en qué perfiles escribir, por qué, y qué avisar.
type profileSelection struct {
	Profiles  []claudeProfile
	Reason    string
	Warnings  []string
	Cancelled bool
}

// resolveClaudeProfiles aplica la tabla de decisión de DOMAINSERV-279, en este orden:
//
//  1. --claude-config-dir      → esos, sin preguntar
//  2. --yes                    → ~/.claude, sin preguntar (avisando si ignora la env var)
//  3. TTY                      → se DETIENE y confirma
//  4. sin TTY (curl | bash)    → todos los detectados, sin preguntar
//
// El orden importa: el caso 4 es el camino canónico de instalación, donde stdin es el
// script. Preguntar ahí no es "menos cómodo", es un cuelgue o una lectura de basura.
func resolveClaudeProfiles(detected []claudeProfile, opts profileResolveOpts) profileSelection {
	if len(opts.Explicit) > 0 {
		return seleccionExplicita(detected, opts.Explicit)
	}
	if opts.Yes {
		return seleccionDelDefault(detected, opts.EnvConfigDir)
	}
	if opts.TTY && opts.Confirm != nil {
		elegidos := opts.Confirm(detected)
		return profileSelection{
			Profiles:  elegidos,
			Reason:    "confirmados por el usuario",
			Cancelled: len(elegidos) == 0,
		}
	}
	return profileSelection{
		Profiles: detected,
		Reason:   "sin TTY: se configuran todos los perfiles detectados",
	}
}

// seleccionExplicita respeta los paths del flag tal cual, incluso si la detección no los
// encontró: el usuario puede estar creando un perfil nuevo en esta misma corrida.
func seleccionExplicita(detected []claudeProfile, explicit []string) profileSelection {
	porPath := make(map[string]claudeProfile, len(detected))
	for _, p := range detected {
		porPath[p.Path] = p
	}
	elegidos := make([]claudeProfile, 0, len(explicit))
	for _, raw := range explicit {
		path := filepath.Clean(raw)
		if p, ok := porPath[path]; ok {
			elegidos = append(elegidos, p)
			continue
		}
		elegidos = append(elegidos, claudeProfile{Path: path})
	}
	return profileSelection{Profiles: elegidos, Reason: "--claude-config-dir"}
}

// seleccionDelDefault resuelve --yes al perfil default. Si CLAUDE_CONFIG_DIR apunta a otro
// lado la ignora —decisión del usuario— pero lo AVISA: ignorarla en silencio hace lo
// contrario de lo que espera quien corre --yes con la variable puesta.
func seleccionDelDefault(detected []claudeProfile, envConfigDir string) profileSelection {
	sel := profileSelection{Reason: "--yes: solo el perfil default"}
	for _, p := range detected {
		if filepath.Base(p.Path) == ".claude" {
			sel.Profiles = []claudeProfile{p}
			break
		}
	}
	if len(sel.Profiles) == 0 && len(detected) > 0 {
		sel.Profiles = []claudeProfile{detected[0]}
	}
	if envConfigDir != "" && len(sel.Profiles) > 0 && filepath.Clean(envConfigDir) != sel.Profiles[0].Path {
		sel.Warnings = append(sel.Warnings, fmt.Sprintf(
			"--yes ignora CLAUDE_CONFIG_DIR=%s y configura %s. Ese perfil queda SIN hooks y sin "+
				"el git-guard: para configurarlo corré --claude-config-dir %s",
			envConfigDir, sel.Profiles[0].Path, envConfigDir))
	}
	return sel
}
