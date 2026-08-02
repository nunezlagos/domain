package mcpserver

import (
	"context"
	"strings"
	"testing"

	"github.com/google/uuid"
	"github.com/mark3labs/mcp-go/mcp"

	"nunezlagos/domain/internal/auth/apikey"
)

// DOMAINSERV-185 parte A: las 3 tools de knowledge se registraban peladas, con wrap.Wrap y
// sin ningún wireup de transacción. Cuando entre la migración que pone RLS por project_id,
// "sin tx" NO da error: la policy compara project_id contra un GUC vacío, eso es falso, y
// domain_knowledge_search devuelve 0 filas SIEMPRE — indistinguible de "no hay nada", que
// es el modo de falla engañoso que el ticket describe.
//
// La ventana se acota a la LÍNEA del registro de cada tool. Buscar el wrapper en todo
// server.go daría verde por cualquiera de las ~40 tools que ya lo usan: el falso positivo
// por subcadena que este repo ya cometió cuatro veces (ver attachment_index_test.go).
func TestKnowledgeTools_SeRegistranConElWrapperDeProyecto(t *testing.T) {
	src := fuenteDe(t, "server.go")
	for _, tool := range []string{"domain_knowledge_save", "domain_knowledge_search", "domain_knowledge_get"} {
		t.Run(tool, func(t *testing.T) {
			for _, linea := range strings.Split(src, "\n") {
				// el nombre de la tool aparece además en la lista de channels y en el
				// mcp.NewTool de su schema: sin exigir wrap.Wrap el guard se ancla en la
				// primera de esas y opina sobre una línea que no es el registro
				if !strings.Contains(linea, `"`+tool+`"`) || !strings.Contains(linea, "wrap.Wrap(") {
					continue
				}
				if !strings.Contains(linea, "rlsProyecto(") {
					t.Errorf(
						"%s se registra SIN el wrapper de proyecto: sin tx no hay SET LOCAL "+
							"app.current_project_id, así que el RLS de knowledge_docs compara contra "+
							"un GUC vacío y devuelve 0 filas sin error.\nlínea: %s",
						tool, strings.TrimSpace(linea))
				}
				return
			}
			t.Fatalf("no encontré el registro de %s en server.go: si la tool se renombró o se "+
				"movió, este guard dejó de mirar lo que cree mirar", tool)
		})
	}
}

// El GUC es LOCAL a una tx: setearlo sin tx no falla en Postgres, se pierde con el
// statement. Ese "éxito" silencioso es el peor resultado posible acá, porque deja al
// handler corriendo con el scope vacío.
func TestSetProjectScope_SinTxEnElContexto_Falla(t *testing.T) {
	if err := setProjectScope(context.Background(), uuid.New()); err == nil {
		t.Error("setProjectScope devolvió nil sin tx en el contexto: el SET LOCAL no sobrevive " +
			"al statement, así que el GUC queda vacío y el llamador cree que hay scope")
	}
	// un projectID nulo escribe el GUC en cadena vacía, que bajo RLS compara falso contra
	// todo: mismo 0 filas, sin error
	if err := setProjectScope(context.Background(), uuid.Nil); err == nil {
		t.Error("setProjectScope aceptó uuid.Nil: el GUC quedaría vacío y el RLS devolvería 0 filas")
	}
}

// El wrapper tiene que FALLAR CERRADO: ante un scope irresoluble el handler NO se invoca.
// Si corriera igual, bajo RLS devolvería 0 filas sin error y el caller leería "no hay
// knowledge" cuando lo que hubo es un scope no seteado.
//
// Declarado explícitamente: el camino feliz —slug válido → GUC seteado— NO se cubre acá,
// porque exige base viva; lo cubre rls_sabotage_integration_test.go.
func TestWithProjectTxHandler_ScopeIrresoluble_NoInvocaElHandler(t *testing.T) {
	casos := map[string]*Deps{
		"sin principal":              {},
		"sin project_slug ni pool":   {Principal: &apikey.Principal{OrganizationID: uuid.NewString(), UserID: uuid.NewString()}},
		"organization_id no es uuid": {Principal: &apikey.Principal{OrganizationID: "no-soy-un-uuid", UserID: uuid.NewString()}},
	}
	for nombre, d := range casos {
		t.Run(nombre, func(t *testing.T) {
			invocado := false
			wrapped := withProjectTxHandler(d, func(context.Context, mcp.CallToolRequest) (*mcp.CallToolResult, error) {
				invocado = true
				return mcp.NewToolResultText("ok"), nil
			})
			res, err := wrapped(context.Background(), mcp.CallToolRequest{})
			if err != nil {
				t.Fatalf("el wrapper tiene que devolver un error de tool, no un error de transporte: %v", err)
			}
			if invocado {
				t.Error("el handler corrió con el GUC de proyecto sin setear: eso es 0 filas sin error")
			}
			if res == nil || !res.IsError {
				t.Errorf("el resultado no es un error de tool: %+v", res)
			}
		})
	}
}
