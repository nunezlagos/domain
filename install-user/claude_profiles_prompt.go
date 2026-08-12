package main

import (
	"bufio"
	"fmt"
	"os"
	"strconv"
	"strings"
)

// repeatableFlag junta las apariciones de un flag que se puede pasar varias veces
// (--claude-config-dir a.dir --claude-config-dir b.dir).
type repeatableFlag []string

func (r *repeatableFlag) String() string { return strings.Join(*r, ",") }

func (r *repeatableFlag) Set(v string) error {
	if strings.TrimSpace(v) == "" {
		return fmt.Errorf("valor vacío")
	}
	*r = append(*r, v)
	return nil
}

// confirmarPerfiles muestra los perfiles detectados y devuelve los que el usuario aprueba.
// Solo se invoca con TTY: quien llama ya resolvió eso (resolveClaudeProfiles).
//
// Enter sin escribir nada acepta TODOS los detectados, que es el caso normal y evita que
// el flujo habitual cueste tipeo. Para elegir un subconjunto se escriben los números.
func confirmarPerfiles(in *bufio.Reader, perfiles []claudeProfile) []claudeProfile {
	fmt.Printf("\n  Detecté %d perfil(es) de Claude:\n\n", len(perfiles))
	for i, p := range perfiles {
		marca := " "
		if p.Active {
			marca = "*"
		}
		estado := ""
		if !p.Exists {
			estado = "  (se va a crear)"
		}
		fmt.Printf("   %s %d) %s%s\n", marca, i+1, p.Path, estado)
	}
	fmt.Print("\n  * = perfil activo\n" +
		"  Enter = todos · números separados por coma (ej: 1,3) · 'n' = ninguno\n" +
		"  ¿Cuáles configuro? ")

	linea, err := in.ReadString('\n')
	if err != nil && strings.TrimSpace(linea) == "" {
		// stdin se cerró antes de responder: no se asume consentimiento sobre lo que
		// nadie confirmó. Es el mismo criterio fail-closed del resto del instalador.
		fmt.Println()
		warnL("no pude leer la respuesta: no se configura ningún perfil")
		return nil
	}
	return perfilesElegidos(strings.TrimSpace(linea), perfiles)
}

// perfilesElegidos traduce la respuesta a la lista de perfiles. Se separa de la lectura de
// stdin para poder testearla sin una terminal.
func perfilesElegidos(respuesta string, perfiles []claudeProfile) []claudeProfile {
	if respuesta == "" {
		return perfiles
	}
	if r := strings.ToLower(respuesta); r == "n" || r == "no" || r == "ninguno" {
		return nil
	}

	vistos := map[int]bool{}
	elegidos := make([]claudeProfile, 0, len(perfiles))
	for _, campo := range strings.Split(respuesta, ",") {
		n, err := strconv.Atoi(strings.TrimSpace(campo))
		if err != nil || n < 1 || n > len(perfiles) {
			warnL(fmt.Sprintf("ignoro %q: no es un número de la lista", strings.TrimSpace(campo)))
			continue
		}
		if vistos[n] {
			continue
		}
		vistos[n] = true
		elegidos = append(elegidos, perfiles[n-1])
	}
	return elegidos
}

// resolverPerfilesDeClaude arma la selección de perfiles para runInstall: detecta, resuelve
// según flags/TTY, imprime lo decidido y devuelve los perfiles a configurar.
func resolverPerfilesDeClaude(home string, explicit []string, yes bool) []claudeProfile {
	env := os.Getenv("CLAUDE_CONFIG_DIR")
	detectados := detectClaudeProfiles(home, env)

	sel := resolveClaudeProfiles(detectados, profileResolveOpts{
		Explicit:     explicit,
		Yes:          yes,
		TTY:          isTTY(),
		EnvConfigDir: env,
		Confirm: func(p []claudeProfile) []claudeProfile {
			return confirmarPerfiles(bufio.NewReader(os.Stdin), p)
		},
	})

	for _, w := range sel.Warnings {
		warnL(w)
	}
	if sel.Cancelled {
		info("no se seleccionó ningún perfil de Claude")
		return nil
	}
	// Sin TTY nadie confirmó nada, así que la lista se IMPRIME: es la única forma de que
	// quede registro de en qué directorios se escribió.
	if !isTTY() && len(explicit) == 0 && !yes && len(sel.Profiles) > 0 {
		info(fmt.Sprintf("sin TTY: configuro los %d perfil(es) detectado(s)", len(sel.Profiles)))
		for _, p := range sel.Profiles {
			info("  " + p.Path)
		}
	}
	return sel.Profiles
}
