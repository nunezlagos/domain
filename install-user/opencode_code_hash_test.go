package main

import (
	"os"
	"os/exec"
	"regexp"
	"strings"
	"testing"
)

// DOMAINSERV-247, defecto 1. El commit-gate del plugin exige un hash almacenado desde ad279c21,
// pero NO lo compara contra el working tree: una edición posterior a la corrida no invalida el
// marker. Falta portar domain_tests_code_hash (domain-hooks-lib.sh:63) a JS.
//
// EL RIESGO DE ESTE PORT, y por eso este test existe: replicarlo mal da un FALSO NEGATIVO
// PERMANENTE. El gate nunca aceptaría un marker legítimo, y un gate que deniega lo legítimo
// empuja al bypass — peor que el defecto que arregla.
//
// La única garantía posible es comparar la salida de la implementación JS contra la de la función
// BASH real, ejecutadas sobre el mismo repo. Si coinciden byte a byte, el port es correcto.

// codeHashJS corre la implementación del plugin.
func codeHashJS(t *testing.T, repo string) string {
	t.Helper()
	src, err := os.ReadFile("templates/opencode-sdd-gate.js")
	if err != nil {
		t.Fatal(err)
	}
	bloque := regexp.MustCompile(`(?s)function codeHashDelRepo\(.*?\n\}`).FindString(string(src))
	if bloque == "" {
		t.Fatal("no se encontró codeHashDelRepo en el plugin: el port del defecto 1 no está hecho")
	}
	script := "const { execSync, execFileSync } = require('child_process');\n" +
		"const { createHash } = require('crypto');\n" + bloque +
		"\nprocess.stdout.write(String(codeHashDelRepo(process.argv[1]) || ''))"

	out, err := exec.Command("node", "-e", script, repo).CombinedOutput()
	if err != nil {
		t.Fatalf("node falló: %v\n%s", err, out)
	}
	return strings.TrimSpace(string(out))
}

// codeHashBash corre la función original, que es la fuente de verdad.
func codeHashBash(t *testing.T, repo string) string {
	t.Helper()
	script := ". " + repo + "/install-user/hooks/domain-hooks-lib.sh && domain_tests_code_hash"
	cmd := exec.Command("bash", "-c", script)
	cmd.Dir = repo
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("bash falló: %v\n%s", err, out)
	}
	return strings.TrimSpace(string(out))
}

// EL TEST QUE HACE SEGURO EL PORT.
func TestCodeHash_LaImplementacionJSCoincideConLaDeBash(t *testing.T) {
	repo, err := exec.Command("git", "rev-parse", "--show-toplevel").Output()
	if err != nil {
		t.Skip("no estamos en un repo git")
	}
	raiz := strings.TrimSpace(string(repo))

	enBash := codeHashBash(t, raiz)
	enJS := codeHashJS(t, raiz)

	if enBash == "" {
		t.Fatal("la función bash devolvió vacío: no hay contra qué comparar")
	}
	if enJS != enBash {
		t.Errorf("el port NO coincide con la fuente de verdad.\n  bash: %s\n  js:   %s\n"+
			"Un hash distinto significa que el gate de OpenCode rechazaría markers legítimos: "+
			"falso negativo permanente que empuja al bypass, peor que el defecto que arregla.",
			enBash, enJS)
	}
}

// `node --check` valida SINTAXIS, no imports: un símbolo sin importar pasa el check y revienta en
// runtime. Pasó al portar codeHashDelRepo — usaba createHash sin importar crypto. Este test carga
// el plugin como MÓDULO real, que es lo único que ejercita los imports.
func TestPluginSddGate_SeImportaSinErrores(t *testing.T) {
	ruta, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	mod := "file://" + ruta + "/templates/opencode-sdd-gate.js"

	out, err := exec.Command("node", "--input-type=module", "-e",
		"await import(process.argv[1]); process.stdout.write('ok')", mod).CombinedOutput()
	if err != nil {
		t.Fatalf("el plugin no se puede importar — un símbolo sin importar pasa `node --check` y "+
			"revienta recién al cargarlo:\n%s", out)
	}
	if !strings.Contains(string(out), "ok") {
		t.Errorf("importación incompleta: %s", out)
	}
}

// INCIDENCIA ENCONTRADA AL PORTAR (DOMAINSERV-247): el hash de bash dependía del LOCALE. `sort`
// de GNU con es_CL.UTF-8 ignora la puntuación al comparar, así que el orden de la lista —y el
// hash— cambiaba con el LANG de quien corriera el hook. MEDIDO: 3f32e5d8... con LC_ALL=C contra
// 08391a55... con es_CL.UTF-8, en el mismo repo y el mismo instante.
//
// Es peor que una molestia: si el post-test escribía el marker con un locale y el pre-edit
// comparaba con otro, el gate denegaba el commit sin ninguna razón visible. El port a JS lo
// destapó porque .sort() de JS es binario y no coincidía con nada.
func TestCodeHash_NoDependeDelLocale(t *testing.T) {
	repo, err := exec.Command("git", "rev-parse", "--show-toplevel").Output()
	if err != nil {
		t.Skip("no estamos en un repo git")
	}
	raiz := strings.TrimSpace(string(repo))
	lib := raiz + "/install-user/hooks/domain-hooks-lib.sh"

	conLocale := func(locale string) string {
		cmd := exec.Command("bash", "-c", ". "+lib+" && domain_tests_code_hash")
		cmd.Dir = raiz
		cmd.Env = append(os.Environ(), "LC_ALL="+locale, "LANG="+locale)
		out, err := cmd.CombinedOutput()
		if err != nil {
			t.Fatalf("bash con LC_ALL=%s falló: %v\n%s", locale, err, out)
		}
		return strings.TrimSpace(string(out))
	}

	enC := conLocale("C")
	enES := conLocale("es_CL.UTF-8")

	if enC == "" {
		t.Fatal("la función devolvió vacío")
	}
	if enC != enES {
		t.Errorf("el hash cambia con el locale.\n  LC_ALL=C: %s\n  es_CL:    %s\n"+
			"Si el post-test escribe el marker con un locale y el pre-edit compara con otro, el "+
			"gate deniega el commit sin razón visible. Falta LC_ALL=C en el sort.", enC, enES)
	}
}

// El hash tiene que CAMBIAR ante una edición, o no sirve para detectar que el código se tocó
// después de la corrida — que es exactamente lo que el defecto 1 pide.
func TestCodeHash_CambiaAnteUnaEdicion(t *testing.T) {
	repo, err := exec.Command("git", "rev-parse", "--show-toplevel").Output()
	if err != nil {
		t.Skip("no estamos en un repo git")
	}
	raiz := strings.TrimSpace(string(repo))

	antes := codeHashJS(t, raiz)

	tmp := raiz + "/install-user/.codehash-probe.go.tmp"
	if err := os.WriteFile(tmp, []byte("// sonda temporal de DOMAINSERV-247\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	despues := codeHashJS(t, raiz)
	os.Remove(tmp)

	if antes == despues {
		t.Error("el hash NO cambió al agregar un archivo: no puede detectar una edición posterior " +
			"a la corrida de tests, que es todo el punto del defecto 1")
	}

	if restaurado := codeHashJS(t, raiz); restaurado != antes {
		t.Errorf("el hash no volvió a su valor tras quitar la sonda (%s != %s): sería inestable y "+
			"el gate denegaría al azar", restaurado, antes)
	}
}
