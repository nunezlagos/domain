package main

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// REQ-4 de issue-57.1, fase 2. Hasta acá bootstrap.sh instalaba Go y compilaba, y ese binario
// se declara 'dev' — o sea que TODO cliente instalado por el camino oficial quedaba fuera de
// la comparación de versiones y jamás recibía el aviso. Bajar el binario publicado es lo que
// hace que el trabajo de las fases 0 y 1 le llegue a alguien.
//
// La restricción dura del usuario: nada puede exigir estar logueado en Git. Bajar de Releases
// es anónimo, y estos tests lo verifican en vez de confiar en que así sea.

func leerBootstrap(t *testing.T) string {
	t.Helper()
	raw, err := os.ReadFile("bootstrap.sh")
	if err != nil {
		t.Fatal(err)
	}
	return string(raw)
}

func TestBootstrap_Descarga_NoUsaCredencialesNiGit(t *testing.T) {
	bs := leerBootstrap(t)

	// La descarga tiene que poder correrla alguien que solo clonó el repo, sin cuenta ni token.
	for _, prohibido := range []string{"Authorization:", "GITHUB_TOKEN", "curl -u ", "--user "} {
		if strings.Contains(bs, prohibido) {
			t.Errorf("bootstrap.sh usa %q: la descarga dejaría de ser anónima y rompería la restricción de REQ-4", prohibido)
		}
	}
	// git sigue siendo dependencia del script por otras razones, pero el binario NO puede venir
	// de un clone: eso es exactamente lo que la fase 2 elimina.
	if strings.Contains(bs, "git clone") {
		t.Error("bootstrap.sh clona el repo para obtener el binario: la fase 2 existe para no necesitarlo")
	}
}

func TestBootstrap_Descarga_VerificaElChecksumPublicado(t *testing.T) {
	bs := leerBootstrap(t)

	if !strings.Contains(bs, "SHA256SUMS") {
		t.Fatal("bootstrap.sh no verifica el binario contra SHA256SUMS.txt: una descarga truncada se ejecutaría igual")
	}
	// sha256sum es de coreutils y NO existe en macOS, donde el equivalente es `shasum -a 256`.
	// bootstrap.sh soporta darwin explícitamente; sin las dos, la verificación falla siempre en
	// mac y el script cae al fallback de compilar en silencio.
	//
	// Se exige la INVOCACIÓN completa de cada una y no que la palabra aparezca: un primer
	// intento de este test buscaba "shasum" suelto y sobrevivía a romper el `command -v`, o sea
	// que no discriminaba nada.
	for _, invocacion := range []string{"sha256sum -c", "shasum -a 256 -c"} {
		if !strings.Contains(bs, invocacion) {
			t.Errorf("falta la invocación %q: la verificación no funcionaría en una de las dos plataformas que el script soporta", invocacion)
		}
	}
	for _, deteccion := range []string{"command -v sha256sum", "command -v shasum"} {
		if !strings.Contains(bs, deteccion) {
			t.Errorf("falta %q: sin detectar cuál existe, la verificación falla siempre en la plataforma que no la tiene", deteccion)
		}
	}
}

// Los 5 assets publicados no cubren todas las arquitecturas (linux/386, riscv...), y un tag
// cuyo workflow falla no publica ninguno — pasó con v0.3.0. Sin fallback esos casos quedan sin
// instalador.
//
// Se EJECUTA el script con un curl que falla, en vez de buscar "go build" en el texto: la
// primera versión de este test hacía lo segundo y sobrevivía a un `if false && go build`, o sea
// que daba verde con el fallback desactivado.
func TestBootstrap_SinBinarioPublicado_CaeACompilar(t *testing.T) {
	work := t.TempDir()
	shim := filepath.Join(work, "bin")
	if err := os.MkdirAll(shim, 0o755); err != nil {
		t.Fatal(err)
	}
	// curl que siempre falla: simula una arquitectura sin asset publicado o un tag sin release
	if err := os.WriteFile(filepath.Join(shim, "curl"), []byte("#!/usr/bin/env bash\nexit 22\n"), 0o755); err != nil {
		t.Fatal(err)
	}
	// go que reporta la versión pero falla al compilar: alcanza para ver que el script LLEGÓ a
	// intentar compilar, sin dejar un binario real en el árbol de trabajo
	goFake := "#!/usr/bin/env bash\n[ \"$1\" = version ] && { echo 'go version go1.25.0 linux/amd64'; exit 0; }\nexit 1\n"
	if err := os.WriteFile(filepath.Join(shim, "go"), []byte(goFake), 0o755); err != nil {
		t.Fatal(err)
	}

	cmd := exec.Command("bash", "bootstrap.sh", "--version")
	cmd.Env = append(os.Environ(), "PATH="+shim+":"+os.Getenv("PATH"))
	out, _ := cmd.CombinedOutput() // falla a propósito en el build: lo que importa es que llegue

	texto := string(out)
	if !strings.Contains(texto, "Compilando domain-install") {
		t.Fatalf("con la descarga fallando, el script NO cayó a compilar: una plataforma sin binario publicado se quedaría sin instalador.\nsalida:\n%s", texto)
	}
	if !strings.Contains(texto, "no se pudo bajar o verificar") {
		t.Errorf("el fallback ocurrió sin avisar por qué: quien lo lea no sabría que hubo un intento de descarga fallido.\nsalida:\n%s", texto)
	}
}

func TestBootstrap_Descarga_ElAssetSaleDeLaPlataformaDetectada(t *testing.T) {
	bs := leerBootstrap(t)

	// El script ya resuelve OS y ARCH con uname (líneas 35-45) usando los mismos nombres que los
	// assets publicados. Componer el nombre a mano en vez de reusarlos abriría la puerta a que
	// diverjan.
	if !strings.Contains(bs, "domain-install-${OS}-${ARCH}") {
		t.Error("el nombre del asset no se compone con el OS y ARCH ya detectados: podrían divergir de los nombres publicados")
	}
	if !strings.Contains(bs, "releases/latest/download") {
		t.Error("no baja de /releases/latest/download: hardcodear una versión obligaría a tocar el script en cada release")
	}
}

// El end-to-end real: un shim de curl sirve un binario falso, y como bootstrap.sh termina en
// `exec ./domain-install "$@"`, si la salida es la del binario servido entonces se ejecutó el
// DESCARGADO y no uno compilado. No hace falta ninguna puerta de test en el script.
func TestBootstrap_ConBinarioPublicadoDisponible_EjecutaElDescargadoYNoCompila(t *testing.T) {
	work := t.TempDir()
	shim := filepath.Join(work, "bin")
	if err := os.MkdirAll(shim, 0o755); err != nil {
		t.Fatal(err)
	}

	// El shim distingue el asset del SHA256SUMS por el nombre pedido, y calcula el checksum de
	// verdad para que la verificación del script se ejercite en serio.
	curlFake := `#!/usr/bin/env bash
dest=""; url=""
while [ $# -gt 0 ]; do
  case "$1" in
    -o) dest="$2"; shift 2 ;;
    -*) shift ;;
    *) url="$1"; shift ;;
  esac
done
case "$url" in
  *SHA256SUMS*)
    printf '%s  domain-install-%s-%s\n' "$FAKE_SUM" "$FAKE_OS" "$FAKE_ARCH" > "$dest" ;;
  *domain-install-*)
    cp "$FAKE_BIN" "$dest" ;;
  *) exit 22 ;;
esac
`
	if err := os.WriteFile(filepath.Join(shim, "curl"), []byte(curlFake), 0o755); err != nil {
		t.Fatal(err)
	}

	fakeBin := filepath.Join(work, "fake-domain-install")
	if err := os.WriteFile(fakeBin, []byte("#!/usr/bin/env bash\necho BINARIO-DESCARGADO \"$@\"\n"), 0o755); err != nil {
		t.Fatal(err)
	}

	// go NO está en el PATH del shim: si el script intentara compilar, fallaría en vez de
	// producir un falso verde por haber compilado el binario real.
	suma, herramienta := checksumDe(t, fakeBin)
	if herramienta == "" {
		t.Skip("ni sha256sum ni shasum disponibles en este entorno")
	}

	cmd := exec.Command("bash", "bootstrap.sh", "--version")
	cmd.Dir = "."
	cmd.Env = append(os.Environ(),
		"PATH="+shim+":"+os.Getenv("PATH"),
		"FAKE_BIN="+fakeBin,
		"FAKE_SUM="+suma,
		"FAKE_OS="+osDetectado(),
		"FAKE_ARCH="+archDetectado(),
	)
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("bootstrap.sh falló con el binario disponible: %v\n%s", err, out)
	}
	if !strings.Contains(string(out), "BINARIO-DESCARGADO") {
		t.Fatalf("no se ejecutó el binario descargado; salida:\n%s", out)
	}
	if strings.Contains(string(out), "Compilando domain-install") {
		t.Errorf("compiló pese a tener el binario publicado disponible:\n%s", out)
	}
}

func checksumDe(t *testing.T, path string) (string, string) {
	t.Helper()
	if _, err := exec.LookPath("sha256sum"); err == nil {
		out, err := exec.Command("sha256sum", path).Output()
		if err != nil {
			t.Fatal(err)
		}
		return strings.Fields(string(out))[0], "sha256sum"
	}
	if _, err := exec.LookPath("shasum"); err == nil {
		out, err := exec.Command("shasum", "-a", "256", path).Output()
		if err != nil {
			t.Fatal(err)
		}
		return strings.Fields(string(out))[0], "shasum"
	}
	return "", ""
}

func osDetectado() string {
	out, _ := exec.Command("uname", "-s").Output()
	if strings.HasPrefix(strings.TrimSpace(string(out)), "Darwin") {
		return "darwin"
	}
	return "linux"
}

func archDetectado() string {
	out, _ := exec.Command("uname", "-m").Output()
	m := strings.TrimSpace(string(out))
	if m == "arm64" || m == "aarch64" {
		return "arm64"
	}
	return "amd64"
}
