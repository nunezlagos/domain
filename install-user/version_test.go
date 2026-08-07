package main

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// REQ-1 e REQ-3 de issue-57.1. La comparación es una función PURA a propósito: es la única
// pieza de este change que decide algo, y meterla en el python del hook la habría dejado
// solo testeable levantando el hook entero con un server simulado.

func TestVersionInfo_SinInyeccion_SeIdentificaComoDesarrollo(t *testing.T) {
	info := VersionInfo()

	if !strings.Contains(info, versionDeDesarrollo) {
		t.Fatalf("un build sin -X debe identificarse como desarrollo; devolvió %q", info)
	}
	// REQ-1: "NO reporta una versión de release que no le corresponde". Un default como
	// "0.0.0" o "1.0.0" cumpliría el formato semver y el hook lo compararía como si fuera
	// real: el cliente se creería al día o desactualizado sin base.
	if EsVersionDeRelease(Version) {
		t.Fatalf("el default %q pasa por versión de release: un build local estaría mintiendo", Version)
	}
}

func TestVersionInfo_ConInyeccionDeLdflags_ReportaVersionYCommit(t *testing.T) {
	bin := filepath.Join(t.TempDir(), "domain-install-test")
	build := exec.Command("go", "build",
		"-ldflags", "-X main.Version=1.2.0 -X main.Commit=abc1234",
		"-o", bin, ".")
	if out, err := build.CombinedOutput(); err != nil {
		t.Fatalf("build con -ldflags falló: %v\n%s", err, out)
	}

	out, err := exec.Command(bin, "--version").CombinedOutput()
	if err != nil {
		t.Fatalf("--version falló: %v\n%s", err, out)
	}
	texto := string(out)
	if !strings.Contains(texto, "1.2.0") {
		t.Errorf("--version no reporta la versión inyectada; devolvió %q", texto)
	}
	if !strings.Contains(texto, "abc1234") {
		t.Errorf("--version no reporta el commit inyectado; devolvió %q", texto)
	}
}

// Sabotaje (a) del design: quitar el -X del build deja este test en rojo. Sin él, el guard
// de versión no existe y el binario vuelve a no saber qué es sin que nada lo note.
func TestMakefile_Ldflags_InyectaLaVersionYElCommit(t *testing.T) {
	raw, err := os.ReadFile("Makefile")
	if err != nil {
		t.Fatal(err)
	}
	mk := string(raw)
	for _, simbolo := range []string{"-X main.Version=", "-X main.Commit="} {
		if !strings.Contains(mk, simbolo) {
			t.Errorf("el Makefile no inyecta %q: el binario compilado no sabría qué versión es (REQ-1)", simbolo)
		}
	}
}

// El workflow construye los 5 targets que la gente descarga. Si el Makefile inyecta y el
// workflow no, los binarios publicados —justamente los que hay que comparar— son los únicos
// que no saben su versión.
func TestReleaseWorkflow_Build_InyectaElTagComoVersion(t *testing.T) {
	raw, err := os.ReadFile(filepath.Join("..", ".github", "workflows", "release-installer.yml"))
	if err != nil {
		t.Fatal(err)
	}
	wf := string(raw)
	if !strings.Contains(wf, "-X main.Version=") {
		t.Fatal("release-installer.yml compila sin inyectar la versión: los binarios publicados no sabrían cuál son")
	}
	if !strings.Contains(wf, "github.ref_name") && !strings.Contains(wf, "GITHUB_REF_NAME") {
		t.Error("la versión inyectada no sale del tag: el número publicado y el del release podrían divergir")
	}
}

// El primer tag del proyecto (v0.3.0) hizo correr este workflow por primera vez desde que
// existe —run_number=1— y el job de release murió: había un `defaults: run:
// working-directory: install-user` a nivel WORKFLOW, que aplica a todos los `run:` de todos
// los jobs. Pero download-artifact es una ACTION y descarga en $GITHUB_WORKSPACE/artifacts,
// así que el `find artifacts` del flatten buscaba dentro de install-user/ y no encontraba nada.
//
// El working-directory tiene que quedar en el job que compila, no arriba. Este guard falla si
// alguien lo vuelve a subir.
func TestReleaseWorkflow_WorkingDirectory_NoEsGlobal(t *testing.T) {
	raw, err := os.ReadFile(filepath.Join("..", ".github", "workflows", "release-installer.yml"))
	if err != nil {
		t.Fatal(err)
	}

	for _, linea := range strings.Split(string(raw), "\n") {
		// a nivel workflow las claves van sin indentar; dentro de un job llevan espacios
		if linea == "defaults:" {
			t.Fatal("release-installer.yml tiene un `defaults:` a nivel WORKFLOW: el job de release heredaría install-user/ y su `find artifacts` no encontraría lo que download-artifact deja en el workspace")
		}
	}
}

// El repo adoptó flujo con develop: el trabajo nace en ramas, se integra a develop y de ahí
// pasa a main con su tag. Un workflow de CI que no escuche esas dos ramas mira donde el
// trabajo ya no está.
//
// Ya pasó una vez, y está documentado en el propio comentario de ci-mcp.yml (DOMAINSERV-192):
// al migrar de la rama `services` a main, los workflows se quedaron apuntando a la vieja y
// "el push a main solo disparaba CI install-user, que valida otro módulo: un check verde que
// no decía nada de domain-mcp". Este guard existe para que el próximo cambio de flujo no
// repita el error.
func TestWorkflowsDeCI_CubrenLasRamasDondeViveElTrabajo(t *testing.T) {
	ciWorkflows := []string{
		"ci-mcp.yml",
		"ci-install-user.yml",
		"ci-shell-guards.yml",
		"benchmarks-mcp.yml",
	}

	for _, wf := range ciWorkflows {
		raw, err := os.ReadFile(filepath.Join("..", ".github", "workflows", wf))
		if err != nil {
			t.Errorf("%s: %v", wf, err)
			continue
		}
		contenido := string(raw)

		linea := lineaDeBranches(contenido)
		if linea == "" {
			t.Errorf("%s: no declara `branches:` en su trigger de push", wf)
			continue
		}
		for _, rama := range []string{"main", "develop"} {
			if !strings.Contains(linea, rama) {
				t.Errorf("%s: su push no escucha %q (línea: %s) — el trabajo en esa rama no se verificaría", wf, rama, strings.TrimSpace(linea))
			}
		}
		// `services` se borró del remoto: dejarla en la lista describe un repo que ya no existe
		if strings.Contains(linea, "services]") || strings.Contains(linea, "[services") {
			t.Errorf("%s: sigue listando la rama `services`, que ya no existe en el remoto", wf)
		}
	}
}

func lineaDeBranches(contenido string) string {
	for _, l := range strings.Split(contenido, "\n") {
		if strings.Contains(l, "branches:") {
			return l
		}
	}
	return ""
}

func TestCompararVersiones_MenorMayorEIgual(t *testing.T) {
	casos := []struct {
		a, b     string
		esperado int
	}{
		{"1.2.0", "1.2.0", 0},
		{"1.1.0", "1.2.0", -1},
		{"1.2.0", "1.1.0", 1},
		{"1.2.0", "1.10.0", -1},
		{"1.9.0", "1.10.0", -1},
		{"2.0.0", "1.99.99", 1},
		{"1.2.3", "1.2", 1},
		{"1.2", "1.2.0", 0},
		// el prefijo v es como vienen los tags; comparar "v1.2.0" con "1.2.0" como strings
		// distintos daría un aviso permanente que nadie puede callar actualizando
		{"v1.2.0", "1.2.0", 0},
		{"v1.3.0", "v1.2.0", 1},
	}
	for _, c := range casos {
		if got := CompararVersiones(c.a, c.b); got != c.esperado {
			t.Errorf("CompararVersiones(%q, %q) = %d, esperaba %d", c.a, c.b, got, c.esperado)
		}
	}
}

// Formatos raros: la comparación se alimenta de un tag remoto y de un ldflag, o sea de dos
// entradas que nadie valida. Ante algo que no puede ordenar tiene que decir "no sé" y NO
// inventar un orden: un falso "estás viejo" entrena a ignorar el aviso.
func TestCompararVersiones_FormatosRaros_NoInventanUnOrden(t *testing.T) {
	raros := []struct{ a, b string }{
		{"", "1.2.0"},
		{"1.2.0", ""},
		{"dev", "1.2.0"},
		{"1.2.0", "dev"},
		{"no-es-una-version", "1.2.0"},
		{"1.2.0-rc1+meta", "1.2.0"},
	}
	for _, c := range raros {
		if got := CompararVersiones(c.a, c.b); got != versionesIncomparables {
			t.Errorf("CompararVersiones(%q, %q) = %d; con un formato que no puede ordenar debe devolver incomparable", c.a, c.b, got)
		}
	}
}

func TestAvisoDeActualizacion_ClienteViejo_TraeElComandoParaActualizar(t *testing.T) {
	aviso := AvisoDeActualizacion("1.1.0", "1.2.0", "")

	if aviso == "" {
		t.Fatal("un cliente por detrás del server debe producir aviso (REQ-3)")
	}
	if !strings.Contains(aviso, "1.2.0") {
		t.Errorf("el aviso no dice cuál es la versión nueva: %q", aviso)
	}
	if !strings.Contains(aviso, comandoDeActualizacion) {
		t.Errorf("el aviso no trae el comando para actualizar, que es lo único accionable: %q", aviso)
	}
}

func TestAvisoDeActualizacion_ClienteAlDia_NoDiceNada(t *testing.T) {
	if aviso := AvisoDeActualizacion("1.2.0", "1.2.0", ""); aviso != "" {
		t.Fatalf("un cliente al día no debe agregar ninguna línea (REQ-3); devolvió %q", aviso)
	}
}

// El server caído es el caso más frecuente de los tres, y el que no puede degradar a nada
// peor que el silencio: el hook ya arranca la sesión igual ante un bootstrap fallido.
func TestAvisoDeActualizacion_SinVersionDelServer_NoDiceNada(t *testing.T) {
	for _, serverVer := range []string{"", "?", "desconocida"} {
		if aviso := AvisoDeActualizacion("1.1.0", serverVer, ""); aviso != "" {
			t.Errorf("sin versión del server no hay nada que comparar; con %q devolvió %q", serverVer, aviso)
		}
	}
}

// Un cliente de desarrollo es el caso de quien está trabajando EN domain: avisarle que su
// build local está "viejo" contra el server es ruido garantizado en cada sesión.
func TestAvisoDeActualizacion_ClienteDeDesarrollo_NoDiceNada(t *testing.T) {
	if aviso := AvisoDeActualizacion(versionDeDesarrollo, "1.2.0", ""); aviso != "" {
		t.Fatalf("un build de desarrollo no se compara contra el server; devolvió %q", aviso)
	}
}

// REQ-6: por debajo del piso de compatibilidad el tono cambia. La sesión arranca igual —
// avisar no es romper— pero el mensaje deja de ser "hay algo nuevo".
func TestAvisoDeActualizacion_ClientePorDebajoDelMinimo_ElTonoEsExplicito(t *testing.T) {
	bajoMinimo := AvisoDeActualizacion("1.0.0", "1.4.0", "1.2.0")
	if bajoMinimo == "" {
		t.Fatal("un cliente por debajo del mínimo soportado debe avisar")
	}
	if !strings.Contains(strings.ToLower(bajoMinimo), "soportad") {
		t.Errorf("el aviso bajo el mínimo debe decir que la versión ya no está soportada, no solo que hay una nueva: %q", bajoMinimo)
	}

	viejoPeroSoportado := AvisoDeActualizacion("1.2.0", "1.4.0", "1.2.0")
	if viejoPeroSoportado == "" {
		t.Fatal("un cliente viejo pero soportado igual avisa, en tono informativo")
	}
	if strings.Contains(strings.ToLower(viejoPeroSoportado), "soportad") {
		t.Errorf("un cliente dentro del piso no debe recibir el mensaje de no-soportado: %q", viejoPeroSoportado)
	}
}
