package phases

import (
	"os"
	"path/filepath"
	"regexp"
	"testing"
)

// DOMAINSERV-209: sdd_spec.go instruía "Construye la spec issue.md siguiendo
// .claude/rules/sdd.md" y ese archivo no existe en ninguna parte — ni en el repo ni en el
// home del usuario. El directorio .claude/rules/ tampoco.
//
// Una referencia rota en un prompt no es un no-op: el agente que la recibe busca el archivo,
// no lo encuentra, y sus salidas son inventar qué diría o seguir sin la regla que se le
// prometió. Es el mismo modo de falla que una descripción de tool que promete algo que el
// código no hace (DOMAINSERV-190): lo que el agente lee ANTES de actuar determina cómo actúa.
//
// La doctrina de este proyecto vive en policies de BD, así que los prompts deben nombrar la
// policy, no un archivo del cliente. Este guard verifica lo que puede verificar sin BD: que
// ninguna ruta de archivo citada en un prompt esté ausente del repo.

var rutaDotClaude = regexp.MustCompile(`\.claude/[A-Za-z0-9._/-]+`)

func TestPromptsDeFase_NoCitanRutasQueNoExisten(t *testing.T) {
	raizRepo := raizDelRepo(t)

	for _, h := range handlersConPlanPotencial() {
		out := buildParaGuard(t, h)
		for _, cruda := range rutaDotClaude.FindAllString(out.UserPrompt, -1) {
			ruta := sinPuntuacionFinal(cruda)
			absoluta := filepath.Join(raizRepo, ruta)
			if _, err := os.Stat(absoluta); err == nil {
				continue
			}
			t.Errorf(
				"el prompt de %s cita %q y ese archivo NO existe en el repo (%s). Un agente "+
					"que lo busque no lo encuentra, y su única salida es inventar qué decía o "+
					"ignorar la regla que se le prometió. La doctrina va en una policy de BD, "+
					"nombrada por slug, no en una ruta de archivo del cliente.",
				h.Slug(), ruta, absoluta)
		}
	}
}

// sinPuntuacionFinal saca el punto o la coma con que termina la oración que cita la ruta. El
// regex no puede excluirlos: un `.` es legítimo dentro de un nombre de archivo. Sin esto el
// guard compararía "settings.json." contra el disco y reportaría faltante un archivo que
// existe — lo detectó el sabotaje, que citó ".claude/rules/sdd.md." con el punto pegado.
func sinPuntuacionFinal(ruta string) string {
	for len(ruta) > 0 {
		ultimo := ruta[len(ruta)-1]
		if ultimo != '.' && ultimo != ',' && ultimo != ';' && ultimo != ':' && ultimo != ')' {
			break
		}
		ruta = ruta[:len(ruta)-1]
	}
	return ruta
}

// raizDelRepo sube desde el paquete hasta encontrar el .git. Es preferible a una ruta relativa
// fija: si el paquete se mueve, un ".." de más devuelve un directorio que existe y el guard
// pasa por la razón equivocada.
func raizDelRepo(t *testing.T) string {
	t.Helper()
	dir, err := os.Getwd()
	if err != nil {
		t.Fatalf("Getwd: %v", err)
	}
	for i := 0; i < 12; i++ {
		if _, err := os.Stat(filepath.Join(dir, ".git")); err == nil {
			return dir
		}
		padre := filepath.Dir(dir)
		if padre == dir {
			break
		}
		dir = padre
	}
	t.Fatal("no encontré la raíz del repo (.git) subiendo desde el paquete: sin ella este " +
		"guard no puede resolver las rutas citadas y no debe pasar en verde")
	return ""
}
