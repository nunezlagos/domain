package main

import (
	"crypto/sha256"
	"encoding/hex"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
)

// DOMAINSERV-239, modos 1, 3 y 4. Hasta acá los lifecycle hooks NO estaban embebidos: los
// instalaba install-curl.sh, o sea un script bash. Por eso doctor_hooks.go usaba
// `scriptOK := fileExists(hookPath)` — no tenía con qué comparar. No era descuido, era una
// restricción real.
//
// Embeberlos hace del hash una verdad del BINARIO y no del disco, que es lo que permite detectar
// un hook adulterado o divergente. La alternativa —un manifiesto que escriba el instalador— se
// descartó en el ticket: lo escribe quien instala, así que no detecta una instalación adulterada
// desde el vamos.

func hooksEsperados() []string {
	var out []string
	for _, spec := range claudeHooks {
		out = append(out, spec.Script)
	}
	return out
}

func TestHooksLifecycle_EstanEmbebidosEnElBinario(t *testing.T) {
	for _, script := range hooksEsperados() {
		b, err := hooksFS.ReadFile("hooks/" + script)
		if err != nil {
			t.Errorf("%s no está embebido: sin el contenido en el binario el doctor solo puede "+
				"comprobar que el archivo EXISTE, que es lo que DOMAINSERV-239 vino a cerrar (%v)",
				script, err)
			continue
		}
		if len(b) == 0 {
			t.Errorf("%s está embebido pero vacío", script)
		}
	}
}

// MODO 3: un binario viejo sobre hooks nuevos NO puede pisarlos en silencio. El archivo en juego
// es domain-pre-edit.sh, o sea el gate SDD.
func TestInstalarHook_ArchivoDivergente_NoLoPisa(t *testing.T) {
	dir := t.TempDir()
	destino := filepath.Join(dir, "domain-pre-edit.sh")
	previo := []byte("#!/usr/bin/env bash\n# version en disco, distinta de la embebida\n")
	if err := os.WriteFile(destino, previo, 0o755); err != nil {
		t.Fatal(err)
	}

	res := instalarHookEmbebido("domain-pre-edit.sh", dir)

	actual, err := os.ReadFile(destino)
	if err != nil {
		t.Fatal(err)
	}
	if string(actual) != string(previo) {
		t.Error("el hook divergente fue PISADO: un binario de un tag anterior degradaría el gate " +
			"SDD sin aviso y sin backup")
	}
	if res != hookDivergente {
		t.Errorf("la divergencia no se reporta (res=%v): pisar en silencio y callar son el mismo "+
			"defecto para quien opera", res)
	}
}

// El caso legítimo no puede romperse: si el hook no existe, se instala.
func TestInstalarHook_NoExiste_LoEscribe(t *testing.T) {
	dir := t.TempDir()

	res := instalarHookEmbebido("domain-pre-edit.sh", dir)

	if res != hookEscrito {
		t.Fatalf("un hook ausente no se instaló (res=%v): una instalación limpia quedaría sin gate", res)
	}
	b, err := os.ReadFile(filepath.Join(dir, "domain-pre-edit.sh"))
	if err != nil {
		t.Fatal(err)
	}
	esperado, _ := hooksFS.ReadFile("hooks/domain-pre-edit.sh")
	if string(b) != string(esperado) {
		t.Error("lo escrito no coincide con lo embebido")
	}
}

// Idempotencia: si ya está al día, no se reescribe ni se reporta como problema.
func TestInstalarHook_YaAlDia_NoHaceNada(t *testing.T) {
	dir := t.TempDir()
	contenido, _ := hooksFS.ReadFile("hooks/domain-stop.sh")
	if err := os.WriteFile(filepath.Join(dir, "domain-stop.sh"), contenido, 0o755); err != nil {
		t.Fatal(err)
	}

	if res := instalarHookEmbebido("domain-stop.sh", dir); res != hookAlDia {
		t.Errorf("un hook idéntico al embebido se reportó como %v en vez de al-día", res)
	}
}

// Un gate sin +x no corre, y el fallo sería mudo.
func TestInstalarHook_QuedaEjecutable(t *testing.T) {
	dir := t.TempDir()
	instalarHookEmbebido("domain-pre-edit.sh", dir)

	info, err := os.Stat(filepath.Join(dir, "domain-pre-edit.sh"))
	if err != nil {
		t.Fatal(err)
	}
	if info.Mode().Perm()&0o100 == 0 {
		t.Error("el hook quedó sin permiso de ejecución: el gate no correría")
	}
}

// domain-hooks-lib.sh NO está en claudeHooks porque no es un hook registrado: es la lib que
// TODOS cargan con `. "$LIB"`. Si el binario pasa a ser el dueño (modo 4) y solo itera
// claudeHooks, la lib no se instala y los hooks fallan en su primera línea útil — un fallo mudo
// que se ve como "el gate no anda" sin decir por qué.
func TestInstalarHooks_IncluyeLaLibCompartida(t *testing.T) {
	if _, err := hooksFS.ReadFile("hooks/domain-hooks-lib.sh"); err != nil {
		t.Fatalf("la lib compartida no está embebida: %v", err)
	}

	dir := t.TempDir()
	instalarHooksLifecycle(dir)

	if _, err := os.Stat(filepath.Join(dir, "domain-hooks-lib.sh")); err != nil {
		t.Error("instalarHooksLifecycle no instaló domain-hooks-lib.sh: los hooks la cargan con " +
			"`. \"$LIB\"` y sin ella fallan en su primera línea útil")
	}
}

// MODO 4: el dueño es el binario. Si install-curl.sh vuelve a copiar los hooks, los dos escriben
// el mismo archivo y gana el último — que es exactamente el problema que el modo 4 vino a evitar.
func TestInstallCurl_YaNoCopiaLosHooks(t *testing.T) {
	b, err := os.ReadFile("install-curl.sh")
	if err != nil {
		t.Fatalf("no se pudo leer install-curl.sh: %v", err)
	}

	var reclamos []string
	for i, linea := range strings.Split(string(b), "\n") {
		l := strings.TrimSpace(linea)
		if strings.HasPrefix(l, "#") || !strings.Contains(l, "hooks/domain-") {
			continue
		}
		// copiar o escribir un hook es reclamar la propiedad; nombrarlo en un mensaje no
		if strings.Contains(l, "install ") || strings.Contains(l, "cp ") || strings.Contains(l, "curl ") {
			reclamos = append(reclamos, strconv.Itoa(i+1)+": "+l)
		}
	}

	if len(reclamos) > 0 {
		t.Errorf("install-curl.sh volvió a instalar hooks en %v.\n"+
			"El dueño es el binario (go:embed hooks): si los dos escriben el mismo archivo gana el "+
			"último, y el hash del binario deja de ser la verdad contra la que el doctor compara.",
			reclamos)
	}
}

// La utilidad del hash: si no discriminara, todo el ticket sería decorativo.
func TestHashDeHook_DiscriminaContenido(t *testing.T) {
	h := func(b []byte) string { s := sha256.Sum256(b); return hex.EncodeToString(s[:]) }

	emb, err := hooksFS.ReadFile("hooks/domain-pre-edit.sh")
	if err != nil {
		t.Fatal(err)
	}
	if h(emb) == h(append(emb, '\n')) {
		t.Error("un byte de diferencia no cambia el hash")
	}
}
