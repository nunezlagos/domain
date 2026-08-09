package main

import (
	"crypto/sha256"
	"encoding/hex"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// DOMAINSERV-271. bootstrap.sh baja el binario publicado y lo verifica contra SHA256SUMS.txt
// desde issue-57.1 fase 2; bootstrap.ps1 no tenía ese bloque y SIEMPRE instalaba Go y compilaba.
// Un binario compilado se declara 'dev' y queda fuera de la comparación de versiones por diseño
// (bootstrap.sh:107-109), así que en Windows el aviso de actualización no podía dispararse nunca.
//
// No hay runner Windows y no se va a pagar uno (ver el comentario del step cross-platform en
// ci-install-user.yml). Estos tests EJECUTAN el .ps1 con PowerShell Core sobre el runner Linux:
// el .ps1 no usa nada específico de Windows en el camino de descarga, y lo poco que usaba
// ($env:LOCALAPPDATA, $env:TEMP) se resolvió con fallback justamente para que esto sea posible.
//
// La descarga se intercepta shadowing `Invoke-WebRequest` con una FUNCIÓN definida en el scope
// que invoca al script: en PowerShell la resolución de comandos es alias > función > cmdlet, así
// que el script usa la del test sin ninguna puerta de test en el código de producción.

func leerBootstrapPS1(t *testing.T) string {
	t.Helper()
	raw, err := os.ReadFile("bootstrap.ps1")
	if err != nil {
		t.Fatal(err)
	}
	return string(raw)
}

// pwsh viene en la imagen de los runners ubuntu-* de GitHub. Si algún día dejara de venir, este
// guard tiene que ponerse ROJO, no saltearse: un skip en CI es un guard que nadie ejecuta nunca.
func pwshPath(t *testing.T) string {
	t.Helper()
	p, err := exec.LookPath("pwsh")
	if err != nil {
		if os.Getenv("CI") != "" {
			t.Fatal("pwsh no está en el PATH del runner: sin él NADA verifica bootstrap.ps1 y no hay runner Windows que lo cubra")
		}
		t.Skip("pwsh no instalado (local): instalalo para ejercitar bootstrap.ps1")
	}
	return p
}

// El .ps1 escribe domain-install.exe al lado suyo. Se corre sobre una COPIA para no ensuciar el
// árbol de trabajo, y la copia es el archivo real: no hay una segunda versión que pueda divergir.
func ps1EnSandbox(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	raw, err := os.ReadFile("bootstrap.ps1")
	if err != nil {
		t.Fatal(err)
	}
	dst := filepath.Join(dir, "bootstrap.ps1")
	if err := os.WriteFile(dst, raw, 0o644); err != nil {
		t.Fatal(err)
	}
	return dst
}

// driverPS1 arma el harness: define el shim de Invoke-WebRequest y después invoca el script real.
const driverPS1 = `
$ErrorActionPreference = "Continue"
function Invoke-WebRequest {
  param([Parameter(Mandatory=$true)][string]$Uri, [string]$OutFile, [switch]$UseBasicParsing, [int]$TimeoutSec)
  if ($Uri -like "*SHA256SUMS.txt") {
    if ($env:CASO -eq "sin-sums") { throw "Response status code does not indicate success: 404 (Not Found)." }
    Set-Content -LiteralPath $OutFile -Value ($env:SUM_SERVIDO + "  " + $env:ASSET_SERVIDO)
    return
  }
  if ($Uri -like "*domain-install-windows-*") {
    if ($env:CASO -eq "sin-asset") { throw "Response status code does not indicate success: 404 (Not Found)." }
    Copy-Item -LiteralPath $env:BIN_FALSO -Destination $OutFile -Force
    & chmod +x $OutFile   # solo para que el runner Linux pueda ejecutarlo; en Windows no aplica
    return
  }
  throw "URL inesperada: $Uri"
}
& $env:PS1_BAJO_TEST -DryRun
`

func correrPS1(t *testing.T, caso, sumServido, assetServido string, extraEnv ...string) string {
	t.Helper()
	pwsh := pwshPath(t)
	script := ps1EnSandbox(t)
	dir := filepath.Dir(script)

	// El "binario publicado" es un script con shebang: si aparece su salida, se ejecutó el
	// DESCARGADO. No hace falta ninguna marca dentro del .ps1 para saberlo.
	binFalso := filepath.Join(dir, "asset-servido")
	contenido := []byte("#!/usr/bin/env bash\necho BINARIO-DESCARGADO \"$@\"\n")
	if err := os.WriteFile(binFalso, contenido, 0o755); err != nil {
		t.Fatal(err)
	}
	suma := sha256.Sum256(contenido)
	if sumServido == "" {
		sumServido = hex.EncodeToString(suma[:]) // minúsculas, como las escribe sha256sum
	}
	if assetServido == "" {
		assetServido = "domain-install-windows-amd64.exe"
	}

	driver := filepath.Join(dir, "driver.ps1")
	if err := os.WriteFile(driver, []byte(driverPS1), 0o644); err != nil {
		t.Fatal(err)
	}

	// go que reporta versión pero falla al compilar: alcanza para ver que el script LLEGÓ a
	// intentar compilar, sin dejar un binario real ni bajar Go de verdad.
	shim := filepath.Join(dir, "shim")
	if err := os.MkdirAll(shim, 0o755); err != nil {
		t.Fatal(err)
	}
	goFalso := "#!/usr/bin/env bash\n[ \"$1\" = version ] && { echo 'go version go1.25.0 linux/amd64'; exit 0; }\nexit 1\n"
	if err := os.WriteFile(filepath.Join(shim, "go"), []byte(goFalso), 0o755); err != nil {
		t.Fatal(err)
	}

	cmd := exec.Command(pwsh, "-NoProfile", "-File", driver)
	cmd.Dir = dir
	cmd.Env = append(os.Environ(),
		"PATH="+shim+string(os.PathListSeparator)+os.Getenv("PATH"),
		"PS1_BAJO_TEST="+script,
		"BIN_FALSO="+binFalso,
		"SUM_SERVIDO="+sumServido,
		"ASSET_SERVIDO="+assetServido,
		"CASO="+caso,
	)
	cmd.Env = append(cmd.Env, extraEnv...)
	out, _ := cmd.CombinedOutput() // el fallback termina en build fallido a propósito
	return string(out)
}

func TestBootstrapPS1_ConBinarioPublicado_EjecutaElDescargadoYNoCompila(t *testing.T) {
	out := correrPS1(t, "ok", "", "")
	if !strings.Contains(out, "BINARIO-DESCARGADO") {
		t.Fatalf("no se ejecutó el binario descargado: Windows sigue compilando y su cliente sigue declarándose 'dev'.\nsalida:\n%s", out)
	}
	if strings.Contains(out, "Compilando domain-install") {
		t.Errorf("compiló pese a tener el binario publicado disponible:\n%s", out)
	}
}

// El test que le da sentido a bajar con verificación: un asset alterado NO se ejecuta.
func TestBootstrapPS1_ChecksumQueNoCuadra_NoEjecutaElBinario(t *testing.T) {
	out := correrPS1(t, "ok", strings.Repeat("0", 64), "")
	if strings.Contains(out, "BINARIO-DESCARGADO") {
		t.Fatalf("ejecutó un binario cuyo checksum NO coincide con SHA256SUMS.txt:\n%s", out)
	}
	if !strings.Contains(out, "Compilando domain-install") {
		t.Errorf("rechazó el binario pero tampoco cayó a compilar: el usuario se queda sin instalador.\nsalida:\n%s", out)
	}
	if !strings.Contains(out, "checksum distinto") {
		t.Errorf("el rechazo no dice que fue por checksum: se leería como un fallo de red.\nsalida:\n%s", out)
	}
}

// Get-FileHash devuelve MAYÚSCULAS y sha256sum escribe minúsculas. Si la comparación fuera
// sensible a mayúsculas fallaría SIEMPRE, y el síntoma sería "compila igual que antes".
func TestBootstrapPS1_ChecksumCorrecto_ComparaSinDistinguirMayusculas(t *testing.T) {
	binContenido := []byte("#!/usr/bin/env bash\necho BINARIO-DESCARGADO \"$@\"\n")
	suma := sha256.Sum256(binContenido)
	out := correrPS1(t, "ok", strings.ToUpper(hex.EncodeToString(suma[:])), "")
	if !strings.Contains(out, "BINARIO-DESCARGADO") {
		t.Fatalf("el mismo hash en mayúsculas fue rechazado: la comparación no es case-insensitive.\nsalida:\n%s", out)
	}
}

func TestBootstrapPS1_SinAssetPublicado_CaeACompilarYDiceporQue(t *testing.T) {
	out := correrPS1(t, "sin-asset", "", "")
	if !strings.Contains(out, "Compilando domain-install") {
		t.Fatalf("sin asset publicado el script no cayó a compilar: esa plataforma queda sin instalador.\nsalida:\n%s", out)
	}
	if !strings.Contains(out, "no se pudo bajar o verificar") {
		t.Errorf("el fallback ocurrió sin avisar por qué.\nsalida:\n%s", out)
	}
}

// El asset se lista con el nombre exacto; si SHA256SUMS.txt no lo tiene, no hay nada contra qué
// verificar y ejecutarlo igual sería peor que no bajarlo.
func TestBootstrapPS1_AssetNoListadoEnSums_NoEjecuta(t *testing.T) {
	out := correrPS1(t, "ok", "", "domain-install-linux-amd64")
	if strings.Contains(out, "BINARIO-DESCARGADO") {
		t.Fatalf("ejecutó un binario que SHA256SUMS.txt ni siquiera lista:\n%s", out)
	}
	if !strings.Contains(out, "no lista") {
		t.Errorf("no dice que el asset no está listado.\nsalida:\n%s", out)
	}
}

func TestBootstrapPS1_ForzarDesdeElCodigo_NiIntentaBajar(t *testing.T) {
	out := correrPS1(t, "ok", "", "", "DOMAIN_INSTALL_FROM_SOURCE=1")
	if strings.Contains(out, "Bajando el binario publicado") {
		t.Errorf("DOMAIN_INSTALL_FROM_SOURCE=1 no evitó la descarga: no hay forma de instalar desde el código.\nsalida:\n%s", out)
	}
	if !strings.Contains(out, "Compilando domain-install") {
		t.Errorf("con DOMAIN_INSTALL_FROM_SOURCE=1 no compiló.\nsalida:\n%s", out)
	}
}

// El .ps1 es un script: un error de sintaxis no aparece hasta que alguien lo corre en su máquina.
// El parser de PowerShell lo detecta sin ejecutar nada — es el `bash -n` del lado Windows.
func TestBootstrapPS1_Parsea(t *testing.T) {
	pwsh := pwshPath(t)
	abs, err := filepath.Abs("bootstrap.ps1")
	if err != nil {
		t.Fatal(err)
	}
	ps := `$e=$null;$t=$null;` +
		`[System.Management.Automation.Language.Parser]::ParseFile('` + abs + `',[ref]$t,[ref]$e)|Out-Null;` +
		`if($e.Count -gt 0){$e|ForEach-Object{$_.Message};exit 1}`
	out, err := exec.Command(pwsh, "-NoProfile", "-Command", ps).CombinedOutput()
	if err != nil {
		t.Fatalf("bootstrap.ps1 no parsea: %v\n%s", err, out)
	}
}

// Paridad textual con bootstrap.sh. Lo de arriba prueba el COMPORTAMIENTO; esto fija las
// invariantes que el comportamiento no distingue.
func TestBootstrapPS1_Descarga_NoUsaCredencialesNiGit(t *testing.T) {
	ps := leerBootstrapPS1(t)
	for _, prohibido := range []string{"Authorization", "GITHUB_TOKEN", "-Credential", "git clone"} {
		if strings.Contains(ps, prohibido) {
			t.Errorf("bootstrap.ps1 usa %q: la descarga dejaría de ser anónima y hay usuarios sin cuenta de GitHub", prohibido)
		}
	}
}

func TestBootstrapPS1_ParidadConBootstrapSh(t *testing.T) {
	ps := leerBootstrapPS1(t)
	for frag, porque := range map[string]string{
		"releases/latest/download":     "hardcodear una versión obligaría a tocar el script en cada release",
		"SHA256SUMS.txt":               "sin verificar, una descarga alterada o truncada se ejecutaría igual",
		"Get-FileHash":                 "es el equivalente de sha256sum y no exige ningún binario externo",
		"DOMAIN_INSTALL_FROM_SOURCE":   "sin escape no hay forma de instalar desde el código (bootstrap.sh:130)",
		"DOMAIN_REPO":                  "bootstrap.sh:116 permite apuntar a otro repo; sin esto no se puede probar contra un fork",
		"domain-install-windows-$arch": "el asset tiene que salir del arch ya detectado o diverge de lo que publica release-installer.yml",
	} {
		if !strings.Contains(ps, frag) {
			t.Errorf("falta %q en bootstrap.ps1: %s", frag, porque)
		}
	}
}

// release-installer.yml es quien publica el asset que el .ps1 baja. Si su matriz dejara de
// incluir windows/amd64, la descarga fallaría siempre y nadie se enteraría hasta instalar.
func TestBootstrapPS1_ElAssetQueBajaEsElQueLaReleasePublica(t *testing.T) {
	raw, err := os.ReadFile(filepath.Join("..", ".github", "workflows", "release-installer.yml"))
	if err != nil {
		t.Fatal(err)
	}
	wf := string(raw)
	if !strings.Contains(wf, "- os: windows") {
		t.Fatal("release-installer.yml ya no publica windows: bootstrap.ps1 bajaría un asset inexistente en cada instalación")
	}
	if strings.Contains(wf, `bootstrap.sh/ps1 puede entonces (futuro)`) {
		t.Error("el comentario de release-installer.yml sigue diciendo que bajar el binario es 'futuro': ya está implementado en .sh y en .ps1")
	}
}
