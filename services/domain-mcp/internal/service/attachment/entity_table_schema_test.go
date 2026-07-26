package attachment

import (
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

// DOMAINSERV-144: entityTable devolvía "tickets", una tabla que NUNCA existió —
// la real es project_tickets (000112). requireEntity interpola ese nombre con
// fmt.Sprintf, así que todo init_upload sobre un ticket moría con
// `42P01 relation "tickets" does not exist`. Por eso file_attachments tenía 0
// filas: no era falta de uso, era la feature rota para la entidad más usada.
//
// El test unitario de al lado no lo atrapó porque asserteaba el mismo valor
// equivocado: fijaba el bug en vez de detectarlo. Comparar el mapeo contra sí
// mismo nunca lo iba a encontrar — hace falta contrastarlo con el ESQUEMA.
//
// Este test cruza cada tabla del mapeo contra las migraciones del repo. No
// necesita base de datos, así que corre en la suite normal y no detrás de un
// build tag que nadie ejecuta.
func TestEntityTable_TodasLasTablasExistenEnLasMigraciones(t *testing.T) {
	declaradas := tablasDeclaradasEnMigraciones(t)

	for _, entityType := range []string{"user_story", "requirement", "hu_draft", "intake_payload", "ticket"} {
		tabla, ok := entityTable(entityType)
		require.True(t, ok, "entityTable(%q) debería resolver", entityType)
		require.Contains(t, declaradas, tabla,
			"entityTable(%q) apunta a %q, que ninguna migración crea ni renombra: "+
				"requireEntity la interpola y la query moriría con 42P01", entityType, tabla)
	}
}

var (
	reCreate = regexp.MustCompile(`(?i)CREATE\s+TABLE\s+(?:IF\s+NOT\s+EXISTS\s+)?(?:public\.)?"?([a-z_][a-z0-9_]*)"?`)
	reRename = regexp.MustCompile(`(?i)RENAME\s+TO\s+"?([a-z_][a-z0-9_]*)"?`)
)

// tablasDeclaradasEnMigraciones junta los nombres que aparecen en un CREATE
// TABLE o en un RENAME TO de los .up.sql. El RENAME importa: 000155 renombró
// captured_prompts a prompt_captured, así que mirar solo los CREATE dejaría
// fuera tablas que sí existen.
func tablasDeclaradasEnMigraciones(t *testing.T) map[string]bool {
	t.Helper()
	dir := filepath.Join("..", "..", "migrate", "migrations")
	entradas, err := os.ReadDir(dir)
	require.NoError(t, err, "no se pudo leer %s", dir)

	out := map[string]bool{}
	for _, e := range entradas {
		if e.IsDir() || !strings.HasSuffix(e.Name(), ".up.sql") {
			continue
		}
		sql, err := os.ReadFile(filepath.Join(dir, e.Name()))
		require.NoError(t, err)
		for _, re := range []*regexp.Regexp{reCreate, reRename} {
			for _, m := range re.FindAllStringSubmatch(string(sql), -1) {
				out[m[1]] = true
			}
		}
	}
	require.NotEmpty(t, out, "no se detectó ninguna tabla: el parser o la ruta están mal")
	return out
}
