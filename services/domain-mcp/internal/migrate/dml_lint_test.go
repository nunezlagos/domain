package migrate

import (
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
)

// DOMAINSERV-25: lint de migraciones. Prohíbe DML destructivo (DELETE/UPDATE)
// sobre tablas de datos de usuario dentro de una migración de schema. Ese DML
// pertenece a un seeder idempotente o, si es inevitable, exige down reversible +
// backup (ver policy data-migration-methodology). El lint protege de regresiones.

var userTableDML = regexp.MustCompile(
	`(?i)(DELETE\s+FROM|UPDATE)\s+(projects|knowledge_observations|tickets|issues|sdd_requirements|issue_[a-z_]+)\b`)

// legacyDMLAllowlist: migraciones preexistentes con DML sobre tablas de usuario,
// YA aplicadas en prod (irreversibles). NO agregar entradas nuevas: una migración
// nueva que necesite tocar datos de usuario debe repensarse (seeder o backup+down).
var legacyDMLAllowlist = map[string]bool{
	"000166_backfill_project_id.up.sql": true,
}

func TestMigrations_NoDestructiveDMLOnUserTables(t *testing.T) {
	entries, err := os.ReadDir("migrations")
	if err != nil {
		t.Fatalf("no pude leer migrations/: %v", err)
	}
	for _, e := range entries {
		name := e.Name()
		if !strings.HasSuffix(name, ".up.sql") {
			continue
		}
		body, err := os.ReadFile(filepath.Join("migrations", name))
		if err != nil {
			t.Fatalf("no pude leer %s: %v", name, err)
		}
		// UPDATE aparece en "ON UPDATE CASCADE"/"FOR UPDATE"; el regex exige el
		// nombre de tabla de usuario inmediatamente después, así que no matchea.
		if m := userTableDML.FindString(string(body)); m != "" && !legacyDMLAllowlist[name] {
			t.Errorf("%s: DML destructivo sobre tabla de usuario (%q). "+
				"El DML de datos va en un seeder idempotente, no en una migración de schema. "+
				"Si es inevitable: down reversible + backup y agregar a legacyDMLAllowlist con justificación.", name, m)
		}
	}
}
