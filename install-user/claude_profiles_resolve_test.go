package main

import (
	"path/filepath"
	"strings"
	"testing"
)

// DOMAINSERV-279. Esta es la tabla de decisión que pidió el usuario, ejecutable. Cada test
// fija UNA de las cuatro vías de invocación, porque mezclarlas fue lo que hizo falta
// aclarar tres veces en la conversación.

func perfilesDePrueba(t *testing.T) (home string, detectados []claudeProfile) {
	t.Helper()
	home = homeConPerfiles(t, []string{".claude", ".claude-work"}, nil)
	return home, detectClaudeProfiles(home, "")
}

// Vía 1: --claude-config-dir gana sobre todo y NO pregunta. Es el override explícito.
func TestResolveClaudeProfiles_FlagExplicitoNoPregunta(t *testing.T) {
	home, detectados := perfilesDePrueba(t)
	elegido := filepath.Join(home, ".claude-work")
	preguntado := false

	sel := resolveClaudeProfiles(detectados, profileResolveOpts{
		Explicit: []string{elegido},
		TTY:      true,
		Confirm: func([]claudeProfile) []claudeProfile {
			preguntado = true
			return nil
		},
	})

	if preguntado {
		t.Error("preguntó teniendo un override explícito: el flag existe para no preguntar")
	}
	if len(sel.Profiles) != 1 || sel.Profiles[0].Path != elegido {
		t.Fatalf("esperaba solo %q y devolvió %v", elegido, pathsDe(sel.Profiles))
	}
}

// Vía 2: --yes va a ~/.claude y NO pregunta.
func TestResolveClaudeProfiles_YesUsaElDefaultSinPreguntar(t *testing.T) {
	home, detectados := perfilesDePrueba(t)
	preguntado := false

	sel := resolveClaudeProfiles(detectados, profileResolveOpts{
		Yes: true,
		TTY: true,
		Confirm: func([]claudeProfile) []claudeProfile {
			preguntado = true
			return nil
		},
	})

	if preguntado {
		t.Error("--yes preguntó: el flag significa 'no me preguntes'")
	}
	if len(sel.Profiles) != 1 || sel.Profiles[0].Path != filepath.Join(home, ".claude") {
		t.Fatalf("--yes debía resolver a ~/.claude y dio %v", pathsDe(sel.Profiles))
	}
}

// Decisión del usuario: --yes ignora CLAUDE_CONFIG_DIR. Que la ignore está bien; que la
// ignore EN SILENCIO no, porque quien corre --yes en una terminal con la variable seteada
// espera lo contrario. El aviso es la mitad no negociable de esa decisión.
func TestResolveClaudeProfiles_YesAvisaSiIgnoraLaEnvVar(t *testing.T) {
	home, detectados := perfilesDePrueba(t)

	sel := resolveClaudeProfiles(detectados, profileResolveOpts{
		Yes:          true,
		EnvConfigDir: filepath.Join(home, ".claude-work"),
	})

	if len(sel.Warnings) == 0 {
		t.Fatal("no avisó que estaba ignorando CLAUDE_CONFIG_DIR")
	}
	junto := strings.Join(sel.Warnings, " ")
	if !strings.Contains(junto, ".claude-work") {
		t.Errorf("el aviso no nombra el perfil ignorado, así que no es accionable: %q", junto)
	}
}

// Vía 3: con TTY se DETIENE y confirma. Es el pedido textual del usuario.
func TestResolveClaudeProfiles_ConTTYConfirma(t *testing.T) {
	home, detectados := perfilesDePrueba(t)
	soloWork := filepath.Join(home, ".claude-work")
	ofrecidos := 0

	sel := resolveClaudeProfiles(detectados, profileResolveOpts{
		TTY: true,
		Confirm: func(p []claudeProfile) []claudeProfile {
			ofrecidos = len(p)
			for _, c := range p {
				if c.Path == soloWork {
					return []claudeProfile{c}
				}
			}
			return nil
		},
	})

	if ofrecidos != len(detectados) {
		t.Errorf("ofreció %d perfiles y detectó %d: la confirmación tiene que mostrar todos",
			ofrecidos, len(detectados))
	}
	if len(sel.Profiles) != 1 || sel.Profiles[0].Path != soloWork {
		t.Fatalf("no respetó la elección del usuario: %v", pathsDe(sel.Profiles))
	}
}

// Vía 4: sin TTY (el `install-curl.sh | sudo bash`, donde stdin ES el script) NO puede
// preguntar. Configura todos los detectados. Lo que nunca puede hacer es colgarse
// esperando un stdin que no va a llegar.
func TestResolveClaudeProfiles_SinTTYTomaTodosYNoPregunta(t *testing.T) {
	_, detectados := perfilesDePrueba(t)
	preguntado := false

	sel := resolveClaudeProfiles(detectados, profileResolveOpts{
		TTY: false,
		Confirm: func([]claudeProfile) []claudeProfile {
			preguntado = true
			return nil
		},
	})

	if preguntado {
		t.Fatal("preguntó sin TTY: en el pipe del curl eso cuelga o lee basura del script")
	}
	if len(sel.Profiles) != len(detectados) {
		t.Errorf("sin TTY debía tomar los %d detectados y tomó %d — un perfil sin configurar "+
			"es un perfil sin git-guard", len(detectados), len(sel.Profiles))
	}
}

// Si el usuario desmarca todo en la confirmación, eso es una cancelación explícita y hay
// que respetarla. Caer al default "por si acaso" sería escribir justo lo que dijo que no.
func TestResolveClaudeProfiles_ConfirmacionVaciaNoCaeAlDefault(t *testing.T) {
	_, detectados := perfilesDePrueba(t)

	sel := resolveClaudeProfiles(detectados, profileResolveOpts{
		TTY:     true,
		Confirm: func([]claudeProfile) []claudeProfile { return nil },
	})

	if len(sel.Profiles) != 0 {
		t.Errorf("el usuario desmarcó todo y aun así resolvió %v", pathsDe(sel.Profiles))
	}
	if !sel.Cancelled {
		t.Error("una selección vacía tiene que marcarse Cancelled para que el caller no la confunda con un error")
	}
}
