package skill

import (
	"os"
	"regexp"
	"strings"
	"testing"
)

// DOMAINSERV-248. MEDIDO en HEAD 6229220e: SkillGetBySlug (sql/query.sql:70) filtra SOLO por
// slug. No filtra por project_id, es `:one` y no tiene ORDER BY.
//
// Con la migración 000290 el slug pasó a ser único POR PROYECTO, así que dos proyectos con la
// misma skill es un caso legítimo y esperable — y esta query no los distingue. Un skill_get
// puede devolver la del proyecto ajeno, que es el punto 2 de la policy
// security-review-domain-specific.
//
// Además: Service.GetBySlug recibe un orgID y NO LO USA (service.go:460). El parámetro estaba;
// el scoping nunca se implementó. api/handler/skill_metrics.go:24 ya llama con uuid.Nil.
//
// Se verifica sobre la fuente sqlc y no sobre el generado: el .go se regenera y perdería el
// cambio; la fuente es donde vive la decisión.

func queryFuente(t *testing.T) string {
	t.Helper()
	b, err := os.ReadFile("sql/query.sql")
	if err != nil {
		t.Fatalf("no se pudo leer la fuente sqlc: %v", err)
	}
	src := string(b)
	i := strings.Index(src, "-- name: SkillGetBySlug")
	if i < 0 {
		t.Fatal("no se encontró SkillGetBySlug en la fuente")
	}
	resto := src[i:]
	if fin := strings.Index(resto[10:], "-- name:"); fin > 0 {
		return resto[:fin+10]
	}
	return resto
}

func TestSkillGetBySlug_FiltraPorProyecto(t *testing.T) {
	q := queryFuente(t)

	if !strings.Contains(q, "project_id") {
		t.Error("SkillGetBySlug no menciona project_id: un skill_get devuelve la skill de CUALQUIER " +
			"proyecto que comparta el slug. Con la 000290 el slug es único por proyecto, así que " +
			"la colisión entre proyectos es un caso legítimo y esta query no lo distingue " +
			"(cross-project leak, policy security-review-domain-specific punto 2)")
	}
}

// Sin scope explícito la query NO puede caer en una skill de proyecto: el default seguro es
// resolver solo las globales.
func TestSkillGetBySlug_SinScopeSoloResuelveGlobales(t *testing.T) {
	q := queryFuente(t)

	if !strings.Contains(q, "project_id IS NULL") {
		t.Error("la query no contempla el caso sin scope: sin una rama que restrinja a " +
			"project_id IS NULL, un caller que no sabe de qué proyecto habla recibe una skill de " +
			"proyecto arbitraria")
	}
}

// `:one` sin ORDER BY es no determinístico por definición. Aunque el índice de la 000290 haga
// imposible el duplicado dentro de un proyecto, la query puede matchear la global Y la del
// proyecto: hay que decidir cuál gana, no dejarlo al plan de ejecución.
func TestSkillGetBySlug_TieneOrdenDeterministico(t *testing.T) {
	q := queryFuente(t)

	if !regexp.MustCompile(`(?i)order\s+by`).MatchString(q) {
		t.Error("SkillGetBySlug es :one sin ORDER BY: cuando matchea la global y la del proyecto, " +
			"cuál devuelve queda a criterio del plan de ejecución")
	}
	if !regexp.MustCompile(`(?i)limit\s+1`).MatchString(q) {
		t.Error("falta LIMIT 1: con ORDER BY pero sin límite, sqlc :one falla si hay más de una fila")
	}
}

// La precedencia importa y es una decisión: la skill del proyecto gana sobre la global del mismo
// slug. Al revés, un proyecto no podría especializar una skill global.
func TestSkillGetBySlug_LaDelProyectoGanaSobreLaGlobal(t *testing.T) {
	q := queryFuente(t)

	if !regexp.MustCompile(`(?i)order\s+by\s+project_id\s+desc\s+nulls\s+last`).MatchString(q) {
		t.Error("el orden no prioriza la skill del proyecto sobre la global: con NULLS LAST y DESC " +
			"la del proyecto queda primera, que es lo que permite especializar una global")
	}
}
