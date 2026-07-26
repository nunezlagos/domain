package main

import (
	"fmt"
	"regexp"
	"sort"
	"strings"
	"testing"
)

// El detector está duplicado desde services/domain-mcp/internal/seeds/voseo_guard.go: son
// dos módulos Go distintos (nunezlagos/domain-install vs el del server) y no hay paquete
// compartido entre ellos. La copia vive en un _test.go a propósito — el installer no
// necesita detectar voseo en runtime, solo el repo necesita que no entre.
//
// Un drift entre las dos copias afloja un guard, no cambia comportamiento: si el server
// aprende una forma nueva, agregarla acá también.

// arImperativeVoseo matchea el imperativo voseo de verbos -ar sobre un token en minúscula
// (terminación -á tónica). El futuro -rá se excluye aparte porque colisiona.
var arImperativeVoseo = regexp.MustCompile(`^[a-záéíóúñ]{2,}á$`)

var wordSplitter = regexp.MustCompile(`[^\p{L}]+`)

// voseoDenylist: formas voseo (-é/-í/-ás/-és) que el patrón -á no cubre.
var voseoDenylist = map[string]bool{
	"corré": true, "esperá": true, "generá": true, "considerá": true, "mejorá": true,
	"comprá": true, "tené": true, "poné": true, "hacé": true, "volvé": true,
	"escribí": true, "persistí": true, "describí": true, "abrí": true, "subí": true,
	"decí": true, "vení": true, "seguí": true, "elegí": true, "repetí": true,
	"dudás": true, "encontrás": true, "tenés": true, "podés": true, "querés": true,
	"necesitás": true, "hacés": true, "ponés": true, "sabés": true, "debés": true,
	"leé": true, "creé": true, "usalo": true, "usala": true, "delegale": true,
	"pasale": true, "preferila": true, "citalo": true, "llamalo": true, "decilo": true,
	// El pronombre, no una forma verbal: "el criterio le corresponde a quien te delega, no
	// a vos" fue el caso que probó que esto no se arregla con un sed. Un token "vos" suelto
	// no aparece en español neutral técnico, así que no arriesga falsos positivos.
	"vos": true,
}

// Límite conocido del detector: cataloga formas VERBALES de voseo más el pronombre "vos".
// No detecta los clíticos compartidos con el tuteo ("te delega", "tu retorno"), porque un
// token "te"/"tu" suelto aparece en contextos legítimos y el guard pasaría a dar falsos
// positivos. Esas formas quedan para la lectura humana.

// voseoWhitelist: palabras terminadas en -á que NO son voseo.
var voseoWhitelist = map[string]bool{
	"está": true, "acá": true, "allá": true, "quizá": true, "ojalá": true,
	"sofá": true, "mamá": true, "papá": true,
}

func esVoseo(tokenLower string) bool {
	if voseoWhitelist[tokenLower] {
		return false
	}
	if voseoDenylist[tokenLower] {
		return true
	}
	// futuro (será, podrá, esperará): termina en -rá pero no es voseo
	if strings.HasSuffix(tokenLower, "rá") {
		return false
	}
	return arImperativeVoseo.MatchString(tokenLower)
}

// assertEspanolNeutral falla si text contiene voseo rioplatense fuera de la whitelist.
func assertEspanolNeutral(source, text string) error {
	visto := map[string]bool{}
	var hits []string
	for _, tok := range wordSplitter.Split(text, -1) {
		if tok == "" {
			continue
		}
		low := strings.ToLower(tok)
		if esVoseo(low) && !visto[low] {
			visto[low] = true
			hits = append(hits, tok)
		}
	}
	if len(hits) == 0 {
		return nil
	}
	sort.Strings(hits)
	return fmt.Errorf("voseo detectado en %s: %s", source, strings.Join(hits, ", "))
}

func TestAgentVoseo_AssertEspanolNeutral_ConVoseo_DevuelveError(t *testing.T) {
	casos := []string{
		"Delegale la búsqueda cuando el recall sea profundo",
		"Usalo para citar la regla vigente",
		"Si no sabés el slug, filtrá por name",
		"Reportá el resultado y no lo confundas con vacío",
		"el criterio le corresponde a quien te delega, no a vos",
	}
	for _, c := range casos {
		if err := assertEspanolNeutral("test", c); err == nil {
			t.Errorf("esperaba voseo detectado en %q", c)
		}
	}
}

func TestAgentVoseo_AssertEspanolNeutral_Neutral_DevuelveNil(t *testing.T) {
	ok := "Delegar cuando el recall sea profundo. Utilizar para citar la regla vigente. " +
		"Si no se conoce el slug, filtrar por name. El deploy será hoy y el resultado está acá. " +
		"Reportar \"cubrí 3 de 5 policies\" en la Nota; ya leí y edité la línea."
	if err := assertEspanolNeutral("test", ok); err != nil {
		t.Errorf("falso positivo: %v", err)
	}
}

// TestAgentVoseo_ElCatalogoRealHablaEspanolNeutral es el guard que faltaba. El voseo en
// templates/agents/ ya reincidió dos veces: la corrección del 2026-07-08 sobre 6 prompts
// del catálogo, y la de DOMAINSERV-137 sobre estos 9 archivos. Las dos fueron manuales y
// las dos se detectaron leyendo, no fallando. Sin esto hay una tercera.
//
// Cubre el frontmatter además del body: la primera regresión entró por el campo
// `description` de domain-memory.md, que es lo que el hilo principal lee para decidir si
// delega — la superficie más visible del agente.
func TestAgentVoseo_ElCatalogoRealHablaEspanolNeutral(t *testing.T) {
	cat, err := agentCatalog()
	if err != nil {
		t.Fatalf("agentCatalog: %v", err)
	}
	if len(cat) == 0 {
		t.Fatal("el catálogo vino vacío: el guard no estaría cubriendo nada")
	}

	for _, a := range cat {
		if err := assertEspanolNeutral(a.slug+".md", string(a.claude)); err != nil {
			t.Error(err)
		}
		if len(a.opencode) == 0 {
			continue
		}
		if err := assertEspanolNeutral(a.slug+varianteOpencode, string(a.opencode)); err != nil {
			t.Error(err)
		}
	}
}
