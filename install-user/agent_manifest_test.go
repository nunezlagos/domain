package main

import (
	"os"
	"path/filepath"
	"testing"
)

// DOMAINSERV-137: `conflictos` era un campo declarado y reportado que nadie llenaba, así que
// el installer pisaba cualquier edición sin avisar. Estos tests fijan los tres estados de
// procedencia que el manifest hace distinguibles.

func catalogoConTemplate(contenido string) []agentTemplate {
	return []agentTemplate{
		{slug: "con-variante", claude: []byte(contenido),
			opencode: []byte("---\nmode: subagent\nmodel: anthropic/claude-haiku-4-5\n---\nopencode\n")},
	}
}

// El caso que motiva todo el ticket: si el usuario editó el agente, su edición sobrevive y el
// conflicto se reporta. Antes de esto la edición se perdía en silencio.
func TestInstalarAgentes_ArchivoEditadoPorElUsuario_MarcaConflictoYNoPisa(t *testing.T) {
	paths := pathsDePrueba(t)
	cat := catalogoDePrueba()
	if _, err := instalarAgentes(paths, cat); err != nil {
		t.Fatalf("primera corrida: %v", err)
	}

	destino := filepath.Join(paths.GlobalAgentsDir, "con-variante.md")
	editado := "---\nname: con-variante\nmodel: opus\n---\nlo edité yo\n"
	if err := os.WriteFile(destino, []byte(editado), 0o644); err != nil {
		t.Fatal(err)
	}

	res, err := instalarAgentes(paths, cat)
	if err != nil {
		t.Fatalf("segunda corrida: %v", err)
	}

	b, err := os.ReadFile(destino)
	if err != nil {
		t.Fatal(err)
	}
	if string(b) != editado {
		t.Errorf("la edición del usuario se pisó: %q", b)
	}
	if len(res.conflictos) != 1 || res.conflictos[0] != "con-variante" {
		t.Errorf("el conflicto debe reportarse, conflictos = %v", res.conflictos)
	}
}

// El contrapeso del test anterior: respetar TODA diferencia congelaría el catálogo y un fix
// de template no llegaría nunca a una máquina ya instalada. Si el archivo en disco es el que
// domain escribió, se actualiza sin preguntar.
func TestInstalarAgentes_TemplateNuevoSobreElDeDomain_ActualizaSinConflicto(t *testing.T) {
	paths := pathsDePrueba(t)
	viejo := "---\nname: con-variante\nmodel: haiku\n---\nv1\n"
	nuevo := "---\nname: con-variante\nmodel: sonnet\n---\nv2\n"

	if _, err := instalarAgentes(paths, catalogoConTemplate(viejo)); err != nil {
		t.Fatalf("primera corrida: %v", err)
	}
	res, err := instalarAgentes(paths, catalogoConTemplate(nuevo))
	if err != nil {
		t.Fatalf("segunda corrida: %v", err)
	}

	b, err := os.ReadFile(filepath.Join(paths.GlobalAgentsDir, "con-variante.md"))
	if err != nil {
		t.Fatal(err)
	}
	if string(b) != nuevo {
		t.Errorf("el template nuevo no se aplicó, el catálogo quedó congelado: %q", b)
	}
	if len(res.conflictos) != 0 {
		t.Errorf("actualizar lo que domain escribió no es un conflicto: %v", res.conflictos)
	}
}

// Upgrade de una máquina ya instalada: hay agentes en disco pero todavía no hay manifest. Sin
// adopción, las máquinas existentes verían un conflicto por agente justo en el upgrade que
// introduce la mejora.
func TestInstalarAgentes_SinManifestPrevio_AdoptaYActualiza(t *testing.T) {
	paths := pathsDePrueba(t)
	if err := os.MkdirAll(paths.GlobalAgentsDir, 0o755); err != nil {
		t.Fatal(err)
	}
	destino := filepath.Join(paths.GlobalAgentsDir, "con-variante.md")
	if err := os.WriteFile(destino, []byte("instalado antes del manifest\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	nuevo := "---\nname: con-variante\nmodel: sonnet\n---\nv2\n"
	res, err := instalarAgentes(paths, catalogoConTemplate(nuevo))
	if err != nil {
		t.Fatalf("instalarAgentes: %v", err)
	}

	b, err := os.ReadFile(destino)
	if err != nil {
		t.Fatal(err)
	}
	if string(b) != nuevo {
		t.Errorf("sin manifest se adopta y se actualiza, quedó: %q", b)
	}
	if len(res.conflictos) != 0 {
		t.Errorf("la primera corrida no debe reportar conflictos falsos: %v", res.conflictos)
	}
}

// Un conflicto es información, no una excepción: no puede impedir que el resto del catálogo
// se instale.
func TestInstalarAgentes_UnConflicto_NoBloqueaAlResto(t *testing.T) {
	paths := pathsDePrueba(t)
	cat := catalogoDePrueba()
	if _, err := instalarAgentes(paths, cat); err != nil {
		t.Fatalf("primera corrida: %v", err)
	}
	if err := os.WriteFile(filepath.Join(paths.GlobalAgentsDir, "con-variante.md"),
		[]byte("editado\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	res, err := instalarAgentes(paths, cat)
	if err != nil {
		t.Fatalf("segunda corrida: %v", err)
	}

	if len(res.conflictos) != 1 {
		t.Fatalf("conflictos = %v, se esperaba 1", res.conflictos)
	}
	var vioAlOtro bool
	for _, slug := range res.instalados {
		if slug == "sin-variante" {
			vioAlOtro = true
		}
	}
	if !vioAlOtro {
		t.Errorf("el agente en conflicto no debe bloquear al resto, instalados = %v", res.instalados)
	}
	for _, slug := range res.instalados {
		if slug == "con-variante" {
			t.Error("un agente en conflicto no se instaló: no debe contarse como instalado")
		}
	}
}

// El manifest es la primitiva que el sync de DOMAINSERV-141 va a reusar, así que su
// serialización tiene que sobrevivir un round-trip.
func TestManifiestoAgentes_GuardarYCargar_PreservaLosHashes(t *testing.T) {
	ruta := filepath.Join(t.TempDir(), "domain", "agents-manifest.json")
	original := manifiestoAgentes{"repo-scout": "abc123", "domain-memory": "def456"}

	if err := guardarManifiesto(ruta, original); err != nil {
		t.Fatalf("guardarManifiesto: %v", err)
	}
	vuelto := cargarManifiesto(ruta)

	if len(vuelto) != len(original) {
		t.Fatalf("cargarManifiesto devolvió %d entradas, se esperaban %d", len(vuelto), len(original))
	}
	for slug, hash := range original {
		if vuelto[slug] != hash {
			t.Errorf("%s: hash = %q, se esperaba %q", slug, vuelto[slug], hash)
		}
	}
}

// Un manifest ausente o corrupto no puede romper la instalación: se trata como vacío, que
// degrada a adopción, que es el comportamiento seguro.
func TestManifiestoAgentes_AusenteOCorrupto_DevuelveVacio(t *testing.T) {
	dir := t.TempDir()

	if m := cargarManifiesto(filepath.Join(dir, "no-existe.json")); len(m) != 0 {
		t.Errorf("un manifest ausente debe leerse como vacío: %v", m)
	}

	corrupto := filepath.Join(dir, "corrupto.json")
	if err := os.WriteFile(corrupto, []byte("{ esto no es json"), 0o644); err != nil {
		t.Fatal(err)
	}
	if m := cargarManifiesto(corrupto); len(m) != 0 {
		t.Errorf("un manifest corrupto debe leerse como vacío: %v", m)
	}
}

// Desinstalar deja el registro consistente: si el manifest sobreviviera con entradas de
// agentes ya borrados, la instalación siguiente los adoptaría contra un hash inexistente.
func TestDesinstalarAgentes_LimpiaLasEntradasDelManifiesto(t *testing.T) {
	paths := pathsDePrueba(t)
	cat := catalogoDePrueba()
	if _, err := instalarAgentes(paths, cat); err != nil {
		t.Fatalf("instalarAgentes: %v", err)
	}
	if m := cargarManifiesto(paths.AgentsManifest); len(m) == 0 {
		t.Fatal("la instalación debía sembrar el manifest")
	}

	desinstalarAgentes(paths, cat)

	if m := cargarManifiesto(paths.AgentsManifest); len(m) != 0 {
		t.Errorf("el manifest debe quedar sin las entradas del catálogo: %v", m)
	}
}
