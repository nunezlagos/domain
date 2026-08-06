//go:build integration

// DOMAINSERV-240 — el criterio de cierre del ticket: un test end-to-end que haga alta y
// después un POST real a /receive.
//
// Antes de esto la feature era INALCANZABLE, no incompleta: los 6 métodos del service
// existían y su único llamador era webhook_integration_test.go, así que con
// DOMAIN_MASTER_KEY puesta el endpoint dejaba de dar 503 webhooks_disabled y pasaba a dar
// 404 para CUALQUIER slug. Este test es el que falla si el camino de alta se rompe otra vez:
// no mockea el handler ni el service, levanta el router de verdad contra Postgres.
package webhook_test

import (
	"bytes"
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"nunezlagos/domain/internal/api/handler"
	"nunezlagos/domain/internal/service/webhook"
)

// routerDeRecepcion monta el mismo path que registra api.Router() para el endpoint público.
// Se monta a mano y no vía Router() porque este test verifica el camino de recepción, no la
// cadena de middlewares de /api (auth, CORS, rate limit): el endpoint de webhooks es público
// justamente porque no pasa por Bearer.
func routerDeRecepcion(t *testing.T, svc *webhook.Service) (http.Handler, func(context.Context) error) {
	t.Helper()
	api := &handler.API{WebhookService: svc}
	shutdown := api.StartWebhookDispatcher(handler.WebhookDispatcherConfig{QueueSize: 8})
	mux := http.NewServeMux()
	mux.Handle("/api/v1/webhooks/{slug}/receive", api.Router())
	return mux, shutdown
}

func postFirmado(t *testing.T, h http.Handler, slug string, body []byte, secret string) *httptest.ResponseRecorder {
	t.Helper()
	req := httptest.NewRequest(http.MethodPost, "/api/v1/webhooks/"+slug+"/receive", bytes.NewReader(body))
	if secret != "" {
		mac := hmac.New(sha256.New, []byte(secret))
		mac.Write(body)
		req.Header.Set("X-Domain-Signature", "sha256="+hex.EncodeToString(mac.Sum(nil)))
	}
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	return rec
}

// El camino completo: alta por el service (lo que ahora expone domain_webhook_create) y un
// POST firmado que el endpoint acepta. 202 y no 200 porque el dispatch es asincrónico: el
// handler responde StatusAccepted en cuanto encola.
func TestWebhookReceive_AltaYPostFirmado_Acepta(t *testing.T) {
	f, cleanup := setupWebhook(t)
	defer cleanup()

	const secret = "secreto-de-alta-e2e"
	f.create(t, "e2e-hook", secret)
	h, shutdown := routerDeRecepcion(t, f.svc)
	defer func() { _ = shutdown(context.Background()) }()

	rec := postFirmado(t, h, "e2e-hook", []byte(`{"event":"push"}`), secret)

	require.Equal(t, http.StatusAccepted, rec.Code,
		"el alta existe y la firma es válida: el endpoint tiene que aceptar. Un 404 acá significa "+
			"que el camino de alta volvió a no existir (DOMAINSERV-240)")
	var resp map[string]any
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &resp))
	assert.NotEmpty(t, resp, "la respuesta trae el webhook_id y el target_type")
}

// El oráculo de enumeración: un slug que no existe y una firma que no cuadra devuelven
// EXACTAMENTE lo mismo. Con 404 vs 401 se podía enumerar qué webhooks tiene la instancia sin
// credencial alguna, contra un endpoint público.
func TestWebhookReceive_SlugInexistenteYFirmaInvalida_RespuestaIndistinguible(t *testing.T) {
	f, cleanup := setupWebhook(t)
	defer cleanup()

	f.create(t, "existe", "secreto-real")
	h, shutdown := routerDeRecepcion(t, f.svc)
	defer func() { _ = shutdown(context.Background()) }()
	body := []byte(`{"event":"push"}`)

	inexistente := postFirmado(t, h, "no-existe", body, "cualquiera")
	firmaMala := postFirmado(t, h, "existe", body, "secreto-equivocado")

	assert.Equal(t, http.StatusUnauthorized, inexistente.Code)
	assert.Equal(t, http.StatusUnauthorized, firmaMala.Code)
	assert.Equal(t, inexistente.Body.String(), firmaMala.Body.String(),
		"el cuerpo también tiene que ser idéntico: si difiere, el oráculo sigue abierto por otra vía")
}

// Un webhook deshabilitado responde igual que uno inexistente: pausarlo no debe revelar que
// existe. ResolveBySlug ya devuelve ErrNotFound cuando enabled=false; esto lo fija.
func TestWebhookReceive_WebhookDeshabilitado_RespondeComoInexistente(t *testing.T) {
	f, cleanup := setupWebhook(t)
	defer cleanup()
	ctx := context.Background()

	const secret = "secreto-pausado"
	hook := f.create(t, "pausado", secret)
	// el SetEnabled tiene que COMMITEAR: el endpoint público lee por otro pool y una tx
	// abierta le sería invisible, así que el test pasaría por la razón equivocada
	f.enScopeCommit(t, func(scoped context.Context) error {
		return f.svc.SetEnabled(scoped, hook.ID, false)
	})
	h, shutdown := routerDeRecepcion(t, f.svc)
	defer func() { _ = shutdown(ctx) }()
	body := []byte(`{"event":"push"}`)

	pausado := postFirmado(t, h, "pausado", body, secret)
	inexistente := postFirmado(t, h, "no-existe", body, secret)

	assert.Equal(t, http.StatusUnauthorized, pausado.Code,
		"un webhook pausado no debe distinguirse de uno que no existe")
	assert.Equal(t, inexistente.Body.String(), pausado.Body.String())
}

// La firma inválida deja rastro para diagnóstico pero SIN el payload: persistir hasta 5MB
// antes de autenticar convertía un endpoint público en amplificación de escritura.
func TestWebhookReceive_FirmaInvalida_RegistraLaEntregaSinPayload(t *testing.T) {
	f, cleanup := setupWebhook(t)
	defer cleanup()
	ctx := context.Background()

	hook := f.create(t, "sin-payload", "secreto-real")
	h, shutdown := routerDeRecepcion(t, f.svc)
	defer func() { _ = shutdown(ctx) }()

	grande := bytes.Repeat([]byte("A"), 100_000)
	rec := postFirmado(t, h, "sin-payload", grande, "secreto-equivocado")
	require.Equal(t, http.StatusUnauthorized, rec.Code)

	// la lectura va en scope de proyecto: la escribió el camino público por PoolPublic, pero
	// el management sigue bajo RLS y ahí es donde importa el aislamiento
	scoped, cerrar := f.enScope(t)
	defer cerrar()
	deliveries, err := f.svc.Deliveries(scoped, hook.ID, 10)
	require.NoError(t, err)
	require.Len(t, deliveries, 1, "la fila se conserva: es la señal de que llegó algo y no cuadró")
	assert.Equal(t, "signature_invalid", deliveries[0].Status)
	assert.Empty(t, deliveries[0].Payload,
		"el payload NO se persiste antes de autenticar: 100KB de escritura para cualquiera sin credencial")
}

// bitbucket no se puede dar de alta: verifyWebhookSignature devolvía true sin mirar nada, así
// que un webhook con ese source_type aceptaba cualquier payload de cualquiera. La prevención
// va en el service y no solo en la tool MCP, porque el service es la fuente de verdad.
func TestWebhookCreate_Bitbucket_NoSeDaDeAlta(t *testing.T) {
	f, cleanup := setupWebhook(t)
	defer cleanup()

	ctx, cerrar := f.enScope(t)
	defer cerrar()

	_, err := f.svc.Create(ctx, webhook.CreateInput{
		OrganizationID: f.orgID, ProjectID: f.projectID, Slug: "bb-hook", Name: "BB",
		Secret: "s", SourceType: "bitbucket", TargetType: "flow",
		TargetID: uuid.New(), ActorID: f.user,
	})

	require.ErrorIs(t, err, webhook.ErrInvalidSourceType,
		"bitbucket no tiene verificación de firma implementada: dar de alta uno es auth abierta")
}
