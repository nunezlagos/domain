package handler

import (
	"net/http/httptest"
	"os"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// DOMAINSERV-240 defecto 1: bitbucket devolvía `return true` sin verificar NADA, así que un
// webhook con ese source_type aceptaba cualquier payload de cualquiera. Un provider declarado
// sin verificación de firma es peor que un provider ausente: parece soportado.
func TestVerifyWebhookSignature_Bitbucket_NoSeAceptaSinVerificar(t *testing.T) {
	req := httptest.NewRequest("POST", "/api/v1/webhooks/x/receive", nil)

	assert.False(t, verifyWebhookSignature("bitbucket", []byte("secreto"), []byte("{}"), req),
		"bitbucket aceptaba cualquier payload sin mirar la firma")
}

// El default del switch también rechaza: un source_type que nadie implementó no se atiende.
func TestVerifyWebhookSignature_SourceTypeDesconocido_Rechaza(t *testing.T) {
	req := httptest.NewRequest("POST", "/api/v1/webhooks/x/receive", nil)

	assert.False(t, verifyWebhookSignature("gitea", []byte("s"), []byte("{}"), req))
	assert.False(t, verifyWebhookSignature("", []byte("s"), []byte("{}"), req))
}

// gitlab compara el token con hmac.Equal y no con ==. La comparación byte a byte de Go corta
// en el primer byte distinto, así que el tiempo de respuesta filtra cuánto prefijo del secreto
// acertó quien prueba: se puede reconstruir el secreto de a un byte.
func TestVerifyWebhookSignature_Gitlab_TokenCorrectoYIncorrecto(t *testing.T) {
	secreto := []byte("token-de-gitlab")

	req := httptest.NewRequest("POST", "/api/v1/webhooks/x/receive", nil)
	req.Header.Set("X-Gitlab-Token", "token-de-gitlab")
	assert.True(t, verifyWebhookSignature("gitlab", secreto, nil, req))

	malo := httptest.NewRequest("POST", "/api/v1/webhooks/x/receive", nil)
	malo.Header.Set("X-Gitlab-Token", "token-equivocado")
	assert.False(t, verifyWebhookSignature("gitlab", secreto, nil, malo))

	// un token vacío no puede pasar ni con secreto vacío: sería auth abierta
	vacio := httptest.NewRequest("POST", "/api/v1/webhooks/x/receive", nil)
	assert.False(t, verifyWebhookSignature("gitlab", []byte(""), nil, vacio))
}

// La comparación tiene que ser en tiempo constante. Se verifica sobre el código porque medir
// el tiempo en un test es inestable: un == sobre strings volvería mañana sin que nada falle.
func TestVerifyWebhookSignature_Gitlab_ComparaEnTiempoConstante(t *testing.T) {
	cuerpo := leerFuente(t, "webhook.go")
	switchDeFirma := entre(t, cuerpo, "func verifyWebhookSignature(", "\nfunc buildInputs")

	assert.Contains(t, switchDeFirma, "hmac.Equal([]byte(token), secret)",
		"la comparación del token de gitlab volvió a ser no-constante")
	assert.NotContains(t, switchDeFirma, "token == string(secret)",
		"== sobre strings corta en el primer byte distinto y filtra el prefijo correcto")
}

// DOMAINSERV-240 defecto 2: el oráculo de enumeración. Un slug inexistente daba 404 y una firma
// inválida daba 401, así que mirando el código de respuesta se podía enumerar qué webhooks tiene
// la instancia, sin credencial, contra un endpoint público. Las dos ramas ahora responden igual.
func TestReceiveWebhook_SlugInexistenteYFirmaInvalida_MismaRespuesta(t *testing.T) {
	cuerpo := leerFuente(t, "webhook.go")
	receive := entre(t, cuerpo, "func (a *API) receiveWebhook(", "\nfunc ")

	// las dos ramas pasan por el mismo helper: si alguien vuelve a poner un writeError con
	// otro status en una de las dos, esto falla
	assert.Equal(t, 2, strings.Count(receive, "responderNoAutorizado(w)"),
		"las ramas de slug inexistente y firma inválida tienen que responder por el mismo camino")
	assert.NotContains(t, receive, "http.StatusNotFound",
		"el 404 diferenciado es el oráculo de enumeración de DOMAINSERV-240")
}

// DOMAINSERV-240 defecto 3: RecordDelivery persistía el body de hasta 5MB ANTES de autenticar.
// En un endpoint público sin auth eso es amplificación de escritura: cualquiera llena la tabla
// mandando basura con firma inválida. La fila se conserva (saber que llegó algo y no cuadró es
// lo que se diagnostica), el payload no.
func TestReceiveWebhook_FirmaInvalida_NoPersisteElPayload(t *testing.T) {
	cuerpo := leerFuente(t, "webhook.go")
	receive := entre(t, cuerpo, "func (a *API) receiveWebhook(", "\nfunc ")
	ramaInvalida := entre(t, receive, "if !verifyWebhookSignature(", "writeError")

	require.Contains(t, ramaInvalida, "RecordDelivery",
		"la entrega con firma inválida sigue registrándose: es la señal de diagnóstico")
	assert.Contains(t, ramaInvalida, "hook.ID, nil,",
		"el payload se persistía antes de autenticar: 5MB de escritura para cualquiera")
}

// DOMAINSERV-240 defecto 4: `go a.runWebhookTarget(r.Context(), ...)` heredaba el contexto del
// request, que muere al responder, así que la goroutine arrancaba y se cancelaba sola. El fix es
// WithoutCancel (conserva los valores del ctx, como el request_id del logging) más un timeout
// propio para que no quede colgada.
func TestReceiveWebhook_LaGoroutineNoHeredaLaCancelacionDelRequest(t *testing.T) {
	cuerpo := leerFuente(t, "webhook.go")
	receive := entre(t, cuerpo, "func (a *API) receiveWebhook(", "\nfunc ")

	assert.Contains(t, receive, "context.WithoutCancel(r.Context())",
		"la goroutine volvió a heredar la cancelación del request")
	assert.NotContains(t, receive, "go a.runWebhookTarget(r.Context()",
		"ese es exactamente el bug: el ctx muere al responder y el dispatch nunca corre")
}

// El otro lado del defecto 4: NewWebhookDispatcher existía y su único llamador era su propio
// test, así que API.WebhookDispatcher quedaba siempre nil y toda entrega caía a la goroutine
// suelta — una por request, sin cota ni backpressure. StartWebhookDispatcher es el cableado.
func TestStartWebhookDispatcher_DejaElDispatcherCableadoYDevuelveElCierre(t *testing.T) {
	api := &API{}
	require.Nil(t, api.WebhookDispatcher, "arranca sin dispatcher")

	shutdown := api.StartWebhookDispatcher(WebhookDispatcherConfig{QueueSize: 4})

	require.NotNil(t, api.WebhookDispatcher,
		"sin esto toda entrega cae a la goroutine suelta de la rama de fallback")
	require.NotNil(t, shutdown, "sin cierre, un SIGTERM descarta las entregas ya aceptadas con 202")
	require.NoError(t, shutdown(t.Context()))
}

func leerFuente(t *testing.T, nombre string) string {
	t.Helper()
	b, err := os.ReadFile(nombre)
	require.NoError(t, err, "no pude leer %s", nombre)
	return string(b)
}

func entre(t *testing.T, texto, desde, hasta string) string {
	t.Helper()
	i := strings.Index(texto, desde)
	require.GreaterOrEqual(t, i, 0, "no encontré el marcador %q", desde)
	resto := texto[i+len(desde):]
	j := strings.Index(resto, hasta)
	require.Greater(t, j, 0, "no encontré el cierre %q", hasta)
	return resto[:j]
}
