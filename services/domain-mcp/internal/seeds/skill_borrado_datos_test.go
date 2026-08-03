// DOMAINSERV-222, segunda mitad: el guard de borrado mira comandos de Bash, así que una
// migración con DROP TABLE o DELETE sin WHERE NO pasa por él — se escribe con Write y la
// ejecuta el runner de golang-migrate. Ahí no hay red, y la única defensa es el
// procedimiento. Esta skill lo fija; el test verifica que siga estando y con el bump.
//
// Va GLOBAL y no project-scoped a propósito: el incidente que originó el guard ocurrió en
// ace-did-2025, no acá. Una skill de proyecto no cubre el proyecto donde el problema pasó.
package seeds

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestSkillsCatalog_BorradoDeDatos_FijaElProcedimiento(t *testing.T) {
	var contenido string
	for _, s := range SkillCatalog() {
		if s.Slug == "borrado-de-datos-contar-antes" {
			contenido = s.Content
			break
		}
	}
	require.NotEmpty(t, contenido,
		"la skill de borrado de datos debe estar en el catálogo seedeado: registrada solo por MCP no sobrevive un install limpio")

	// Cada fragmento es una regla que el procedimiento no puede perder. Si alguien
	// reescribe el body y se lleva una, el test lo dice en vez de degradarse en silencio.
	for _, frag := range []string{
		"count(*)",     // contar antes de borrar es EL principio
		"WHERE 1=1",    // la trampa: cumple la letra y borra todo
		"breaking:yes", // un borrado no se recupera con el down
		"DOMAINSERV-222",
	} {
		require.Contains(t, contenido, frag,
			"el procedimiento perdió una regla: %q", frag)
	}

	require.GreaterOrEqual(t, skillsSeedVersion, 10,
		"sumar una skill al catálogo exige bump a 10; sin él el re-seed se skippea y el síntoma es indistinguible del éxito")
}
