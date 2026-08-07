package main

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// REQ-3 de issue-57.1. El hook SessionStart es bash + python y no reimplementa la
// comparación: invoca al binario. Así la lógica vive en un solo lugar y se testea como
// función pura, y el hook queda con una responsabilidad que no puede equivocarse.

func compilarConVersion(t *testing.T, version string) string {
	t.Helper()
	bin := filepath.Join(t.TempDir(), "domain-install-test")
	build := exec.Command("go", "build", "-ldflags", "-X main.Version="+version, "-o", bin, ".")
	if out, err := build.CombinedOutput(); err != nil {
		t.Fatalf("build falló: %v\n%s", err, out)
	}
	return bin
}

func TestVersionCheck_ClienteViejo_ImprimeElAvisoConElComando(t *testing.T) {
	bin := compilarConVersion(t, "1.1.0")

	out, err := exec.Command(bin, "--version-check", "1.2.0").CombinedOutput()
	if err != nil {
		t.Fatalf("--version-check no puede fallar: el hook lo corre en cada arranque de sesión: %v\n%s", err, out)
	}
	texto := string(out)
	if !strings.Contains(texto, "1.2.0") || !strings.Contains(texto, comandoDeActualizacion) {
		t.Errorf("el aviso debe traer la versión nueva y el comando; devolvió %q", texto)
	}
}

func TestVersionCheck_ClienteAlDia_NoImprimeNada(t *testing.T) {
	bin := compilarConVersion(t, "1.2.0")

	out, err := exec.Command(bin, "--version-check", "1.2.0").CombinedOutput()
	if err != nil {
		t.Fatalf("--version-check falló: %v\n%s", err, out)
	}
	if strings.TrimSpace(string(out)) != "" {
		t.Errorf("un cliente al día no agrega ninguna línea al bloque; devolvió %q", out)
	}
}

// El server caído es el caso que no puede degradar a nada peor que el silencio: sin versión
// del server el hook igual invoca, y un exit distinto de 0 o un stderr ruidoso se colaría al
// arranque de la sesión.
func TestVersionCheck_SinVersionDelServer_SaleLimpioYSinAviso(t *testing.T) {
	bin := compilarConVersion(t, "1.1.0")

	out, err := exec.Command(bin, "--version-check", "").CombinedOutput()
	if err != nil {
		t.Fatalf("sin versión del server debe salir 0, no fallar: %v\n%s", err, out)
	}
	if strings.TrimSpace(string(out)) != "" {
		t.Errorf("sin nada que comparar no hay aviso; devolvió %q", out)
	}
}

// Sabotaje (b) del design: si alguien hace que el aviso bloquee —un exit != 0, un prompt, una
// espera— este test se pone en rojo. El hook corre en el arranque de CADA sesión.
func TestVersionCheck_NuncaBloquea_SalidaCeroEnTodosLosCasos(t *testing.T) {
	bin := compilarConVersion(t, "1.1.0")

	casos := [][]string{
		{"--version-check", "1.2.0"},
		{"--version-check", ""},
		{"--version-check", "no-es-una-version"},
		{"--version-check", "1.2.0", "1.2.0"},
		{"--version-check"},
	}
	for _, args := range casos {
		if _, err := exec.Command(bin, args...).CombinedOutput(); err != nil {
			t.Errorf("%v salió con error: el aviso NUNCA puede bloquear el arranque (REQ-3): %v", args, err)
		}
	}
}

// El hook tiene que invocar al binario, no reimplementar la comparación: dos copias de la
// regla divergen, y la del hook no tiene tests.
func TestHookSessionStart_InvocaElVersionCheckYLoPegaAlBloque(t *testing.T) {
	raw, err := os.ReadFile(filepath.Join("hooks", "domain-session-start.sh"))
	if err != nil {
		t.Fatal(err)
	}
	hook := string(raw)
	if !strings.Contains(hook, "--version-check") {
		t.Error("el hook no invoca --version-check: el aviso de REQ-3 no se produciría nunca")
	}
	if !strings.Contains(hook, "HOOK_VERSION_NOTICE") {
		t.Error("el hook calcula el aviso pero no lo expone al render: no llegaría al bloque de inicio")
	}
}

// El escenario de "el server no responde" del REQ-3, ejecutado de verdad: sin credenciales el
// hook cae a su rama de bootstrap fallido y aun así tiene que emitir su JSON y salir 0.
func TestHookSessionStart_SinCredenciales_ArrancaIgualYEmiteJSONValido(t *testing.T) {
	home := t.TempDir()
	cmd := exec.Command("bash", filepath.Join("hooks", "domain-session-start.sh"))
	cmd.Stdin = stringReader(`{"session_id":"s1","hook_event_name":"SessionStart","cwd":"` + home + `"}`)
	cmd.Env = append(os.Environ(), "HOME="+home, "DOMAIN_VPS_URL=", "DOMAIN_API_KEY=")

	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("el hook no puede fallar con el server inalcanzable: la sesión arranca igual (REQ-3): %v\n%s", err, out)
	}
	if !strings.Contains(string(out), "hookSpecificOutput") {
		t.Errorf("el hook debe emitir su JSON aun sin server; devolvió %q", out)
	}
}
