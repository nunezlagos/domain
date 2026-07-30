package issuebuilder

import (
	"regexp"
	"strconv"
)

var reIssueOrdinal = regexp.MustCompile(`^issue-\d+\.(\d+)`)

// El ordinal se deriva del máximo ya emitido y no de la cantidad de issues: contar
// reusa un número apenas se borra uno, y dos changes distintos terminan compartiendo
// el mismo (DOMAINSERV-210)
func siguienteOrdinal(slugsDelReq []string) int {
	max := 0
	for _, s := range slugsDelReq {
		if o := ordinalDesdeSlug(s); o > max {
			max = o
		}
	}
	return max + 1
}

func ordinalDesdeSlug(slug string) int {
	m := reIssueOrdinal.FindStringSubmatch(slug)
	if len(m) < 2 {
		return 0
	}
	o, err := strconv.Atoi(m[1])
	if err != nil {
		return 0
	}
	return o
}
