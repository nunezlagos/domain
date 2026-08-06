package mcpserver

import (
	"os"
	"regexp"
	"strings"
	"testing"
)

// DOMAINSERV-223. MEDIDO en prod el 2026-08-06: dos filas activas con slug
// 'correr-migraciones-manuales' en el mismo proyecto, creadas con 3 minutos de diferencia.
//
// El origen es este handler: el INSERT INTO skills no tenía ON CONFLICT, así que cada
// re-registro con el mismo slug creaba una fila nueva. El INSERT en project_skills de la línea
// siguiente SÍ lo tenía — el patrón se conocía; al de skills le faltaba el constraint contra el
// cual hacer conflict, y la migración 000290 se lo da.
//
// Con el índice creado y SIN este upsert, el segundo registro dejaría de duplicar pero fallaría
// con "duplicate key value violates unique constraint", que para quien registra una skill dos
// veces es un error feo por un caso que tiene una respuesta obvia: actualizarla.
//
// Se verifica sobre el TEXTO del handler porque el comportamiento real exige una base con el
// índice aplicado, y eso vive en los tests de integración. Lo que este test sostiene es que el
// statement no vuelva a quedarse sin la cláusula.

func TestProjectSkillRegister_InsertDeSkill_TieneUpsertPorSlug(t *testing.T) {
	b, err := os.ReadFile("project_skill_tools.go")
	if err != nil {
		t.Fatalf("no se pudo leer el handler: %v", err)
	}
	src := string(b)

	re := regexp.MustCompile(`(?s)INSERT INTO skills.*?\x60`)
	stmt := re.FindString(src)
	if stmt == "" {
		t.Fatal("no se encontró el INSERT INTO skills en el handler")
	}

	if !strings.Contains(stmt, "ON CONFLICT") {
		t.Error("el INSERT INTO skills no tiene ON CONFLICT: con el índice único de la 000290 un " +
			"re-registro falla con unique violation en vez de actualizar, y sin el índice duplica " +
			"en silencio — que es el bug medido en prod")
	}
	if !strings.Contains(stmt, "DO UPDATE") {
		t.Error("el ON CONFLICT no actualiza: DO NOTHING dejaría la skill vieja intacta y el " +
			"usuario creería que su re-registro tomó efecto")
	}
}

// El conflicto tiene que resolverse por (project_id, slug), que es la clave del índice parcial de
// la 000290. Apuntar a otra columna haría que el ON CONFLICT no matchee y el INSERT falle igual.
func TestProjectSkillRegister_Upsert_ApuntaALaClaveDelIndiceDeLa290(t *testing.T) {
	b, err := os.ReadFile("project_skill_tools.go")
	if err != nil {
		t.Fatal(err)
	}
	stmt := regexp.MustCompile(`(?s)INSERT INTO skills.*?\x60`).FindString(string(b))

	if !strings.Contains(stmt, "project_id, slug") {
		t.Error("el ON CONFLICT no apunta a (project_id, slug): no coincide con la clave de " +
			"skills_slug_por_proyecto_uniq y Postgres rechaza el statement")
	}
	// el índice de la 000290 es PARCIAL: sin repetir su predicado, Postgres no lo puede usar
	// como árbitro del conflicto
	if !strings.Contains(stmt, "WHERE project_id IS NOT NULL AND deleted_at IS NULL") {
		t.Error("el ON CONFLICT no repite el predicado del índice parcial: un índice parcial solo " +
			"sirve de árbitro si la cláusula lo reproduce textualmente")
	}
}
