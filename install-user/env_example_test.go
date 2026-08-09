package main

import (
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"testing"
)

// DOMAINSERV-274: el cliente no tenía .env.example, así que las variables que entiende solo
// se descubrían leyendo el código. Un archivo de ejemplo arregla eso UNA vez; lo que lo
// mantiene cierto es este guard. Sin él, el ejemplo empieza a mentir en el primer commit que
// agregue una variable, y un ejemplo que miente es peor que no tenerlo: se lee como completo.
//
// La dirección que se verifica es la que importa: toda variable que el cliente LEE del
// usuario tiene que estar nombrada en el ejemplo. Al revés no se exige — el ejemplo
// documenta además cosas que el instalador escribe desde Go y el alias del parser.

// leídas del usuario = las que aparecen con el patrón de default de shell `${DOMAIN_X:-…}`
// o como `process.env.DOMAIN_X` en el plugin de OpenCode. Una var que el hook EXPORTA para
// su propio uso (DOMAIN_GUARD_CWD, DOMAIN_TURN_PID, …) no entra: no es configuración.
var reVarDeUsuario = regexp.MustCompile(`\$\{(DOMAIN_[A-Z0-9_]+):-`)

// documentadas en el ejemplo aunque no matcheen el patrón de arriba, cada una con su razón
var varsFueraDelPatron = map[string]string{
	"DOMAIN_USER_EMAIL":  "la persiste el instalador desde Go (envstore.go), no la lee ningún shell",
	"DOMAIN_MCP_API_KEY": "alias de DOMAIN_API_KEY que acepta el parser de domain-hooks-lib.sh",
	"DOMAIN_API_KEY":     "se resuelve por env o por archivo; el patrón de default no aplica",
}

func archivosQueLeenConfig(t *testing.T) []string {
	t.Helper()
	var out []string
	for _, patron := range []string{"hooks/*.sh", "templates/*.js", "bootstrap.sh"} {
		m, err := filepath.Glob(patron)
		if err != nil {
			t.Fatal(err)
		}
		out = append(out, m...)
	}
	if len(out) == 0 {
		t.Fatal("no se encontró ningún archivo que leer: el glob está roto y este guard " +
			"quedaría verde por vacío")
	}
	return out
}

func TestEnvExample_DocumentaTodaVariableQueElClienteLee(t *testing.T) {
	ejemplo, err := os.ReadFile("install.env.example")
	if err != nil {
		t.Fatalf("install.env.example no existe o no se puede leer: %v", err)
	}
	texto := string(ejemplo)

	encontradas := map[string][]string{}
	for _, f := range archivosQueLeenConfig(t) {
		b, err := os.ReadFile(f)
		if err != nil {
			t.Fatalf("%s: %v", f, err)
		}
		for _, m := range reVarDeUsuario.FindAllStringSubmatch(string(b), -1) {
			encontradas[m[1]] = append(encontradas[m[1]], f)
		}
	}
	if len(encontradas) == 0 {
		t.Fatal("no se detectó ninguna variable: el regex está roto y el guard no mide nada")
	}

	var faltan []string
	for v, donde := range encontradas {
		if !strings.Contains(texto, v) {
			faltan = append(faltan, v+" (leída en "+strings.Join(donde, ", ")+")")
		}
	}
	if len(faltan) > 0 {
		sort.Strings(faltan)
		t.Fatalf("el cliente lee variables que install.env.example no documenta:\n  %s\n\n"+
			"Agregalas al ejemplo. Un .env.example incompleto se lee como completo, así que "+
			"quien lo consulte va a concluir que esas variables no existen.",
			strings.Join(faltan, "\n  "))
	}
}

// La otra mitad: el ejemplo no puede nombrar variables que ya nadie lee. Una var muerta
// documentada manda a configurar algo que no tiene ningún efecto.
func TestEnvExample_NoDocumentaVariablesMuertas(t *testing.T) {
	ejemplo, err := os.ReadFile("install.env.example")
	if err != nil {
		t.Fatal(err)
	}

	vivas := map[string]bool{}
	for _, f := range archivosQueLeenConfig(t) {
		b, err := os.ReadFile(f)
		if err != nil {
			t.Fatal(err)
		}
		for _, m := range reVarDeUsuario.FindAllStringSubmatch(string(b), -1) {
			vivas[m[1]] = true
		}
	}
	// las que el ejemplo documenta y no salen del patrón de shell tienen su razón declarada
	for v := range varsFueraDelPatron {
		vivas[v] = true
	}

	reNombre := regexp.MustCompile(`DOMAIN_[A-Z0-9_]+`)
	var muertas []string
	vistas := map[string]bool{}
	for _, v := range reNombre.FindAllString(string(ejemplo), -1) {
		if vistas[v] || vivas[v] {
			continue
		}
		vistas[v] = true
		muertas = append(muertas, v)
	}
	if len(muertas) > 0 {
		sort.Strings(muertas)
		t.Fatalf("install.env.example documenta variables que ningún archivo del cliente lee:\n  %s\n\n"+
			"Sacalas, o agregá su razón a varsFueraDelPatron si las lee el código Go.",
			strings.Join(muertas, "\n  "))
	}
}
