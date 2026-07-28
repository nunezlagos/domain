package mcpserver

import (
	"os"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

// DOMAINSERV-190: la descripción de domain_skill_create y la note de su response decían
// que una skill global "solo es usable cuando se enlaza con domain_project_skill_register".
// Eso describe la regla de la migración 000160, que fue REVERTIDA por decisión del usuario
// (commit b26b365).
//
// La semántica vigente está en internal/service/skill/service.go, en el comentario de
// ApplicableSkillIDs: "Las globales (project_id IS NULL) aplican automaticamente a TODOS
// los proyectos... project_skills se usa SOLO para EXCLUIR (fila con is_enabled = FALSE)".
// Y domain_project_skill_unlink ya lo decía bien — o sea que las dos descripciones se
// contradecían entre sí DENTRO DEL MISMO ARCHIVO.
//
// El daño de una descripción así no es cosmético: es la que el agente lee ANTES de llamar.
// Quien le crea llama a register "por las dudas" y deja una fila project_skills inútil.
// Es el mismo modo de falla que un parámetro de scope descartado (DOMAINSERV-187) o un
// template que documenta una firma vieja (DOMAINSERV-182): algo que PROMETE un
// comportamiento que el código ya no tiene.

const skillToolsFuente = "project_skill_tools.go"

// frasesDeLaReglaRevertida son las que afirman que una global necesita enlace para servir.
// Si alguna reaparece, la descripción volvió a contradecir al resolver.
var frasesDeLaReglaRevertida = []string{
	"solo es usable cuando se enlaza",
	"No es usable hasta enlazarla",
	"no es usable hasta enlazarla",
}

func fuenteDeSkillTools(t *testing.T) string {
	t.Helper()
	b, err := os.ReadFile(skillToolsFuente)
	require.NoError(t, err, "sin la fuente el guard no verifica nada")
	return string(b)
}

// Ninguna descripción ni note puede afirmar la regla revertida: el resolver hace que las
// globales apliquen solas.
func TestSkillTools_NingunTextoAfirmaLaReglaRevertidaDeEnlace(t *testing.T) {
	fuente := fuenteDeSkillTools(t)

	for _, frase := range frasesDeLaReglaRevertida {
		require.NotContainsf(t, fuente, frase,
			"un texto de las tools de skills afirma %q, pero la migración 000160 fue revertida: "+
				"las globales aplican AUTOMÁTICAMENTE (ver ApplicableSkillIDs) y project_skills solo EXCLUYE. "+
				"Una descripción así hace que el agente cree filas project_skills inútiles",
			frase)
	}
}

// descripcionDeTool extrae el argumento de WithDescription de una tool. Acotar a ESE
// string es el punto: la primera versión de este guard buscaba la palabra en todo el
// bloque desde NewTool hasta el próximo, y ahí adentro entra el handler entero — el
// término aparecía en la `note` del response y el test pasaba sin que la descripción
// dijera nada. Falso verde, el mismo modo de falla que el guard de DOMAINSERV-182 al
// buscar "project_id" cuando la palabra ya estaba en el SELECT de salida.
func descripcionDeTool(t *testing.T, fuente, tool string) string {
	t.Helper()
	i := strings.Index(fuente, `mcp.NewTool("`+tool+`"`)
	require.NotEqualf(t, -1, i, "la tool %s no existe: revisar el guard antes que la tool", tool)

	resto := fuente[i:]
	j := strings.Index(resto, "mcp.WithDescription(")
	require.NotEqualf(t, -1, j, "la tool %s no declara WithDescription", tool)

	desc := resto[j+len("mcp.WithDescription("):]
	fin := strings.Index(desc, "\"),")
	require.NotEqualf(t, -1, fin, "no se pudo cerrar la descripción de %s", tool)
	return desc[:fin]
}

// La descripción de domain_skill_create tiene que decir la semántica vigente, no solo
// evitar la vieja: alguien podría borrar la frase equivocada y dejar el texto mudo, que
// para un agente que decide con esa descripción es casi igual de malo.
func TestSkillTools_SkillCreate_DeclaraQueLaGlobalAplicaSola(t *testing.T) {
	desc := descripcionDeTool(t, fuenteDeSkillTools(t), "domain_skill_create")

	require.Contains(t, strings.ToLower(desc), "automaticamente",
		"la descripción de domain_skill_create no dice que una skill global aplica automáticamente")
}

// El guard tiene que estar mirando la fuente real. Si el archivo se renombra o se parte,
// os.ReadFile falla y los otros dos tests no verifican nada — este lo hace explícito.
func TestSkillTools_LaFuenteDelGuardExiste(t *testing.T) {
	fuente := fuenteDeSkillTools(t)
	require.Contains(t, fuente, `mcp.NewTool("domain_project_skill_unlink"`,
		"la fuente no trae las tools esperadas: el guard quedó apuntando al archivo equivocado")
}
