package main

import (
	"os"
	"path/filepath"
	"testing"
)

// DOMAINSERV-279. La detección de perfiles decide en qué directorios se escribe la config
// del usuario, así que un falso positivo no es cosmético: escribe en un path que no es un
// perfil. Y un falso negativo deja ese perfil sin hooks y sin el git-guard de
// permissions.deny/ask, en silencio.

// homeConPerfiles arma un home de prueba con los directorios y archivos indicados.
func homeConPerfiles(t *testing.T, dirs, files []string) string {
	t.Helper()
	home := t.TempDir()
	for _, d := range dirs {
		if err := os.MkdirAll(filepath.Join(home, d), 0o755); err != nil {
			t.Fatal(err)
		}
	}
	for _, f := range files {
		if err := os.WriteFile(filepath.Join(home, f), []byte("{}"), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	return home
}

func pathsDe(perfiles []claudeProfile) []string {
	out := make([]string, 0, len(perfiles))
	for _, p := range perfiles {
		out = append(out, p.Path)
	}
	return out
}

// El caso REAL medido en la máquina del usuario: junto a ~/.claude hay 13 entradas que
// matchean el glob ~/.claude*, y 10 son .claude.json.tmp.* de escrituras interrumpidas.
// Ofrecer eso como perfiles es ruido que vuelve inusable la confirmación.
func TestDetectClaudeProfiles_IgnoraArchivosYBasura(t *testing.T) {
	home := homeConPerfiles(t,
		[]string{".claude", ".claude-work"},
		[]string{
			".claude.json",
			".claude.json.backup-20260808T212928Z",
			".claude.json.tmp.107155.3bcf6ac7c43e",
			".claude.json.tmp.16898.6902fecfc955",
		},
	)

	got := pathsDe(detectClaudeProfiles(home, ""))

	quiero := map[string]bool{
		filepath.Join(home, ".claude"):      true,
		filepath.Join(home, ".claude-work"): true,
	}
	if len(got) != len(quiero) {
		t.Fatalf("detectó %d perfiles y esperaba %d: %v", len(got), len(quiero), got)
	}
	for _, p := range got {
		if !quiero[p] {
			t.Errorf("detectó %q, que no es un perfil: escribir la config ahí corrompe un archivo ajeno", p)
		}
	}
}

// Un directorio de backup ES un directorio, así que el filtro por tipo no alcanza: hay que
// descartarlo por nombre o el instalador escribiría en una copia de seguridad.
func TestDetectClaudeProfiles_DescartaDirectoriosDeBackup(t *testing.T) {
	home := homeConPerfiles(t, []string{
		".claude",
		".claude.backup-20260808T212928Z",
		".claude-work.bak",
		".claude.tmp.1234",
	}, nil)

	for _, p := range pathsDe(detectClaudeProfiles(home, "")) {
		base := filepath.Base(p)
		if base != ".claude" {
			t.Errorf("detectó %q como perfil: es una copia/temporal, no un config dir vivo", base)
		}
	}
}

// ~/.claude es el default de Claude Code (${CLAUDE_CONFIG_DIR:-$HOME/.claude}, medido en el
// binario 2.1.228). En una instalación limpia todavía no existe, y es justamente el que hay
// que crear: si la detección solo mira el disco, el caso más común queda sin configurar.
func TestDetectClaudeProfiles_IncluyeElDefaultAunqueNoExista(t *testing.T) {
	home := homeConPerfiles(t, nil, nil)

	got := detectClaudeProfiles(home, "")

	if len(got) != 1 {
		t.Fatalf("esperaba solo el default y detectó %d: %v", len(got), pathsDe(got))
	}
	if got[0].Path != filepath.Join(home, ".claude") {
		t.Errorf("el default debería ser ~/.claude y fue %q", got[0].Path)
	}
	if got[0].Exists {
		t.Error("marcó Exists=true un directorio que no existe")
	}
	if !got[0].Active {
		t.Error("el default tiene que quedar Active cuando CLAUDE_CONFIG_DIR no está seteada")
	}
}

// Con CLAUDE_CONFIG_DIR seteada, ese perfil es el activo — es el que la sesión de Claude
// está usando de verdad.
func TestDetectClaudeProfiles_MarcaActivoElDeLaEnvVar(t *testing.T) {
	home := homeConPerfiles(t, []string{".claude", ".claude-work"}, nil)
	activo := filepath.Join(home, ".claude-work")

	got := detectClaudeProfiles(home, activo)

	var vistos int
	for _, p := range got {
		if p.Active {
			vistos++
			if p.Path != activo {
				t.Errorf("marcó activo %q y el activo es %q", p.Path, activo)
			}
		}
	}
	if vistos != 1 {
		t.Errorf("esperaba exactamente 1 perfil activo y hubo %d", vistos)
	}
}

// CLAUDE_CONFIG_DIR puede apuntar FUERA del home o a un nombre que no empieza con .claude
// (la doc del binario menciona CLAUDE_CONFIG_DIR=/tmp). Ese perfil existe y hay que
// configurarlo igual, aunque el glob no lo encuentre.
func TestDetectClaudeProfiles_IncluyeLaEnvVarAunqueNoMatcheeElGlob(t *testing.T) {
	home := homeConPerfiles(t, []string{".claude"}, nil)
	externo := filepath.Join(t.TempDir(), "perfil-raro")
	if err := os.MkdirAll(externo, 0o755); err != nil {
		t.Fatal(err)
	}

	got := pathsDe(detectClaudeProfiles(home, externo))

	encontrado := false
	for _, p := range got {
		if p == externo {
			encontrado = true
		}
	}
	if !encontrado {
		t.Errorf("CLAUDE_CONFIG_DIR=%q quedó fuera de la detección: %v — ese es el perfil que "+
			"la sesión está usando, y quedaría sin hooks y sin git-guard", externo, got)
	}
}

// El orden importa para la confirmación: el activo primero, y el resto estable. Un orden
// que baila entre corridas hace que el usuario confirme una lista distinta cada vez.
func TestDetectClaudeProfiles_OrdenEstableConElActivoPrimero(t *testing.T) {
	home := homeConPerfiles(t, []string{".claude", ".claude-work", ".claude-personal"}, nil)
	activo := filepath.Join(home, ".claude-work")

	primera := pathsDe(detectClaudeProfiles(home, activo))
	segunda := pathsDe(detectClaudeProfiles(home, activo))

	if primera[0] != activo {
		t.Errorf("el activo debería ir primero y fue %q", primera[0])
	}
	if len(primera) != len(segunda) {
		t.Fatalf("dos corridas devolvieron cantidades distintas: %v vs %v", primera, segunda)
	}
	for i := range primera {
		if primera[i] != segunda[i] {
			t.Errorf("el orden no es estable en la posición %d: %q vs %q", i, primera[i], segunda[i])
		}
	}
}
