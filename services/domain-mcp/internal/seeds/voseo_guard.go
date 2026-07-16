package seeds

import (
	"fmt"
	"regexp"
	"sort"
	"strings"
)

// arImperativeVoseo matchea el imperativo voseo de verbos -ar sobre un token en
// minúscula (terminación -á tónica). El futuro -rá se excluye aparte porque colisiona.
var arImperativeVoseo = regexp.MustCompile(`^[a-záéíóúñ]{2,}á$`)

var wordSplitter = regexp.MustCompile(`[^\p{L}]+`)

// voseoDenylist: formas voseo (-é/-í/-ás/-és/-rá) que el patrón -á no cubre.
var voseoDenylist = map[string]bool{
	"corré": true, "esperá": true, "generá": true, "considerá": true, "mejorá": true,
	"comprá": true, "tené": true, "poné": true, "hacé": true, "volvé": true,
	"escribí": true, "persistí": true, "describí": true, "abrí": true, "subí": true,
	"decí": true, "vení": true, "seguí": true, "elegí": true, "repetí": true,
	"dudás": true, "encontrás": true, "tenés": true, "podés": true, "querés": true,
	"necesitás": true, "hacés": true, "ponés": true, "sabés": true, "debés": true,
}

// voseoWhitelist: palabras terminadas en -á que NO son voseo.
var voseoWhitelist = map[string]bool{
	"está": true, "acá": true, "allá": true, "quizá": true, "ojalá": true,
	"sofá": true, "mamá": true, "papá": true,
}

func isVoseo(tokenLower string) bool {
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

// assertNeutralSpanish falla si text contiene voseo rioplatense fuera de la whitelist.
// source identifica el origen (archivo o seeder) para el mensaje de error.
func assertNeutralSpanish(source, text string) error {
	seen := map[string]bool{}
	var hits []string
	for _, tok := range wordSplitter.Split(text, -1) {
		if tok == "" {
			continue
		}
		low := strings.ToLower(tok)
		if isVoseo(low) && !seen[low] {
			seen[low] = true
			hits = append(hits, tok)
		}
	}
	if len(hits) == 0 {
		return nil
	}
	sort.Strings(hits)
	return fmt.Errorf("voseo detectado en %s: %s", source, strings.Join(hits, ", "))
}
