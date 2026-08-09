package main

import (
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
)

// DOMAINSERV-266: macOS no trae `sha256sum` — ahí la utilidad se llama `shasum -a 256`. Los
// hooks lo invocaban directo con `2>/dev/null`, así que en macOS el hash salía VACÍO y nada
// reportaba error. La cadena completa del daño, que es lo que hace grave a un fix de 3
// líneas: el post-test escribe el marker con tree_hash y code_hash vacíos → el pre-edit
// compara contra vacío → cae a su rama fail-closed (DOMAINSERV-95) → el commit-gate DENIEGA
// PARA SIEMPRE, con el doctor en verde.
//
// No hay runner macOS (se evaluó en DOMAINSERV-270 y se descartó por costo), así que la
// ausencia se simula con un PATH donde `sha256sum` no existe y `shasum` sí. Es la misma
// técnica que usa el resto de la suite para el `chown` que necesita root.

// pathSinSha256sum arma un directorio de bin con TODO lo que los hooks necesitan menos
// sha256sum, y devuelve el PATH resultante. shasum se emula con un wrapper sobre el
// sha256sum real: lo que se está probando es la RAMA de fallback, no la implementación del
// algoritmo.
func pathSinSha256sum(t *testing.T, conShasum bool) string {
	t.Helper()
	bin := t.TempDir()

	real, err := exec.LookPath("sha256sum")
	if err != nil {
		t.Skipf("esta máquina no tiene sha256sum, no se puede montar el escenario: %v", err)
	}

	// un sha256sum que NO existe: se logra no enlazándolo. El resto de las utilidades se
	// toman del PATH real para no romper los hooks por otra causa.
	for _, u := range []string{"bash", "sh", "git", "python3", "cut", "grep", "sed", "awk", "cat", "find", "mkdir", "rm", "date", "head", "tr", "sort", "printf", "env", "dirname", "basename", "stat", "wc", "tail", "mktemp", "chmod", "touch", "ls", "id", "uname"} {
		if p, err := exec.LookPath(u); err == nil {
			_ = os.Symlink(p, filepath.Join(bin, u))
		}
	}

	if conShasum {
		// shasum -a 256 con la misma salida que sha256sum: "<hex>  -"
		w := filepath.Join(bin, "shasum")
		cuerpo := "#!/bin/sh\n# emula `shasum -a 256` delegando en el sha256sum real\nexec " + real + "\n"
		if err := os.WriteFile(w, []byte(cuerpo), 0o755); err != nil {
			t.Fatal(err)
		}
	}
	return bin
}

// correrHookConPath es correrHookEnRepo pero con el PATH controlado.
func correrHookConPath(t *testing.T, script, home, dir, payload, path string) string {
	t.Helper()
	cmd := exec.Command(filepath.Join(path, "bash"), hookAbsoluto(t, script))
	cmd.Stdin = strings.NewReader(payload)
	cmd.Dir = dir
	cmd.Env = append(os.Environ(), "HOME="+home, "PATH="+path)
	out, _ := cmd.Output()
	return string(out)
}

// El caso de macOS: sin sha256sum pero CON shasum, el marker tiene que escribirse con su
// alcance igual que en Linux. Si el hash saliera vacío, el commit quedaría bloqueado.
func TestSha256Portable_SinSha256sumPeroConShasum_ElMarkerLlevaSuAlcance(t *testing.T) {
	path := pathSinSha256sum(t, true)
	home := t.TempDir()
	repo := repoGitDePrueba(t)

	correrHookConPath(t, "domain-post-test.sh", home, repo, payloadCorridaVerde(t), path)

	marker := filepath.Join(home, ".local", "state", "domain", "tests-ok-"+sesionGate)
	b, err := os.ReadFile(marker)
	if err != nil {
		t.Fatalf("sin sha256sum no se escribió el marker: en macOS el gate quedaría cerrado. %v", err)
	}
	campos := strings.Split(strings.SplitN(string(b), "\n", 2)[0], "\t")
	if len(campos) < 2 || strings.TrimSpace(campos[1]) == "" {
		t.Fatalf("el marker se escribió con el hash VACÍO: %q\n\n"+
			"Ese es exactamente el modo de falla de macOS — el pre-edit compara contra vacío, "+
			"cae a fail-closed y el commit-gate deniega para siempre, sin reportar nada.", string(b))
	}
}

// Contra-prueba: con sha256sum presente (Linux) el resultado tiene que ser el MISMO. Si
// difiriera, un marker escrito en una máquina no serviría en la otra.
func TestSha256Portable_ConYSinSha256sum_ElHashEsElMismo(t *testing.T) {
	lib := hookAbsoluto(t, "domain-hooks-lib.sh")
	entrada := "material de prueba para el hash\n"

	conSha := pathSinSha256sum(t, true) // tiene shasum, no sha256sum
	hashFallback := hashConLib(t, lib, entrada, conSha)

	hashNativo := hashConLib(t, lib, entrada, os.Getenv("PATH"))

	if hashFallback == "" {
		t.Fatal("la rama de fallback devolvió vacío: no hashea nada")
	}
	if hashFallback != hashNativo {
		t.Fatalf("el hash difiere según la utilidad usada:\n  con shasum:    %q\n  con sha256sum: %q\n\n"+
			"Tienen que coincidir, o un marker escrito en macOS no valdría en Linux y viceversa.",
			hashFallback, hashNativo)
	}
}

func hashConLib(t *testing.T, lib, entrada, path string) string {
	t.Helper()
	sh := filepath.Join(path, "bash")
	if _, err := os.Stat(sh); err != nil {
		sh = "bash"
	}
	cmd := exec.Command(sh, "-c", ". "+lib+" && domain_sha256")
	cmd.Stdin = strings.NewReader(entrada)
	cmd.Env = append(os.Environ(), "PATH="+path)
	out, _ := cmd.Output()
	return strings.TrimSpace(string(out))
}

// El guard sobre el guard: que no quede ningún `sha256sum` invocado directo en los hooks.
// Sin esto, el fix se pierde en el próximo hook que alguien escriba copiando el patrón viejo.
func TestSha256Portable_NingunHookInvocaSha256sumDirecto(t *testing.T) {
	hooks, err := filepath.Glob("hooks/*.sh")
	if err != nil || len(hooks) == 0 {
		t.Fatalf("no se encontró ningún hook: el guard quedaría verde por vacío (err=%v)", err)
	}

	var ofensores []string
	for _, h := range hooks {
		b, err := os.ReadFile(h)
		if err != nil {
			t.Fatal(err)
		}
		for i, linea := range strings.Split(string(b), "\n") {
			// la definición de domain_sha256 SÍ lo nombra: es la única que puede
			if strings.Contains(h, "domain-hooks-lib.sh") {
				continue
			}
			if strings.Contains(linea, "sha256sum") && !strings.HasPrefix(strings.TrimSpace(linea), "#") {
				ofensores = append(ofensores, h+":"+strconv.Itoa(i+1)+" → "+strings.TrimSpace(linea))
			}
		}
	}
	if len(ofensores) > 0 {
		t.Fatalf("hay hooks que invocan sha256sum directo en vez de domain_sha256:\n  %s\n\n"+
			"En macOS esa utilidad no existe y el hash sale vacío sin ningún error, lo que deja "+
			"el commit-gate cerrado permanentemente.", strings.Join(ofensores, "\n  "))
	}
}
