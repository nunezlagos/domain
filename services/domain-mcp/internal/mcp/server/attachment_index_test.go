package mcpserver

import (
	"os"
	"strings"
	"testing"
)

// DOMAINSERV-207: domain_attachment_index escribía en knowledge_docs sin verificar que el
// adjunto existiera, y era la ÚNICA attachment tool registrada sin rls(). Las dos cosas se
// sostienen entre sí: la validación del handler no protege nada sin el SET LOCAL que pone
// withOrgTxHandler, porque el RLS FORCE de file_attachments no tiene org contra la que
// filtrar. Por eso los dos guards viven juntos.
//
// POR QUÉ DE FUENTE Y NO DE COMPORTAMIENTO: Deps.Attachments es un *attachmentsvc.Service
// concreto, no una interfaz, así que no se puede inyectar un doble sin refactorizar Deps —
// y eso es un cambio de diseño que este ticket no acordó. Lo que un test de fuente SÍ puede
// fijar es que el registro lleve rls() y que la validación esté antes de la escritura.
// Declarado explícitamente: el rechazo por pertenencia bajo RLS real NO está cubierto acá;
// exige una base viva con dos organizaciones.

func fuenteDe(t *testing.T, path string) string {
	t.Helper()
	b, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("leer %s: %v", path, err)
	}
	return string(b)
}

// La ventana se acota a la línea del registro. Buscar "rls(" en todo server.go daría verde
// por cualquiera de las otras ~40 tools que sí lo usan — el falso positivo por subcadena que
// este repo ya cometió cuatro veces (DOMAINSERV-190, 182, 205 y la verificación del 156).
func TestAttachmentIndex_SeRegistraConRLS(t *testing.T) {
	for _, linea := range strings.Split(fuenteDe(t, "server.go"), "\n") {
		if !strings.Contains(linea, `"domain_attachment_index"`) {
			continue
		}
		if !strings.Contains(linea, "rls(") {
			t.Errorf(
				"domain_attachment_index se registra SIN rls(), a diferencia de las otras cinco "+
					"attachment tools. Sin withOrgTxHandler no hay SET LOCAL app.current_org_id, "+
					"así que el RLS FORCE de file_attachments no filtra por organización y la "+
					"validación del adjunto en el handler no protege nada.\nlínea: %s",
				strings.TrimSpace(linea))
		}
		return
	}
	t.Fatal("no encontré el registro de domain_attachment_index en server.go: si la tool se " +
		"renombró o se movió, este guard dejó de mirar lo que cree mirar")
}

// El orden importa: validar DESPUÉS de escribir dejaría el doc creado igual.
func TestAttachmentIndex_ValidaElAdjuntoAntesDeEscribir(t *testing.T) {
	src := fuenteDe(t, "attachment_index.go")

	validacion := strings.Index(src, "d.Attachments.Get(ctx, attID)")
	if validacion == -1 {
		t.Fatal("attachment_index.go no resuelve el adjunto con d.Attachments.Get: el " +
			"attachment_id vuelve a ser un dato decorativo en metadata y la procedencia del " +
			"knowledge_doc, incomprobable")
	}
	escritura := strings.Index(src, "d.Knowledge.Save(")
	if escritura == -1 {
		t.Fatal("attachment_index.go no llama d.Knowledge.Save: revisar este guard")
	}
	if validacion > escritura {
		t.Error("la validación del adjunto ocurre DESPUÉS de d.Knowledge.Save: el doc queda " +
			"escrito igual, así que rechazar después no evita nada")
	}
}

// Distinguir "no existe" de "no es tuyo" le confirma a quien llama que ese UUID existe en
// otra organización. Es poco, y es gratis no darlo.
func TestAttachmentIndex_ElRechazoNoDistingueInexistenteDeAjeno(t *testing.T) {
	src := fuenteDe(t, "attachment_index.go")
	for _, filtracion := range []string{"otra organización", "otra organizacion", "no es tuyo", "de otra org"} {
		// se busca en los mensajes de error, no en los comentarios: el comentario SÍ explica
		// la distinción a propósito
		for _, linea := range strings.Split(src, "\n") {
			recortada := strings.TrimSpace(linea)
			if strings.HasPrefix(recortada, "//") {
				continue
			}
			if strings.Contains(recortada, "NewToolResultError") && strings.Contains(recortada, filtracion) {
				t.Errorf("un mensaje de error distingue el caso ajeno y filtra que el UUID "+
					"existe en otra organización: %s", recortada)
			}
		}
	}
}
