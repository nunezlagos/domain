package handler

import (
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/require"
)

// DOMAINSERV-240: collectHeaders copiaba r.Header ENTERO a la fila de webhook_deliveries. Para
// gitlab eso significa persistir el secreto en claro, porque ahí el secreto ES el header:
// verifyWebhookSignature compara `token == string(secret)` contra X-Gitlab-Token. O sea que el
// log de entregas guardaba la credencial que valida las entregas, sin cifrar, y la devolvía
// cualquiera que leyera las deliveries.
//
// Lo que se conserva es la CLAVE y lo que se pierde es el VALOR: saber que llegó un
// X-Hub-Signature es lo que sirve para diagnosticar una firma que no cuadra; su contenido no
// aporta nada al diagnóstico y es lo único que un atacante querría.

func TestCollectHeaders_RedactaLosSensiblesYConservaLaClave(t *testing.T) {
	req := httptest.NewRequest("POST", "/api/v1/webhooks/x/receive", nil)
	sensibles := []string{
		"X-Gitlab-Token", "X-Hub-Signature", "X-Hub-Signature-256", "X-Domain-Signature",
		"Authorization", "Proxy-Authorization", "Cookie", "Set-Cookie",
	}
	for _, h := range sensibles {
		req.Header.Set(h, "valor-secreto-"+h)
	}
	req.Header.Set("User-Agent", "GitHub-Hookshot/abc")
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Request-Id", "req-123")

	out := collectHeaders(req)

	for _, h := range sensibles {
		require.Contains(t, out, h, "la CLAVE %s tiene que sobrevivir: su presencia es lo que sirve para diagnosticar", h)
		require.Equal(t, "[redacted]", out[h], "el VALOR de %s se persistió en claro", h)
		require.NotContains(t, out[h], "valor-secreto", "el valor de %s se filtró", h)
	}
	require.Equal(t, "GitHub-Hookshot/abc", out["User-Agent"], "los headers benignos no se tocan")
	require.Equal(t, "application/json", out["Content-Type"])
	require.Equal(t, "req-123", out["X-Request-Id"])
}

// La comparación es case-insensitive y no es paranoia: Go canonicaliza los headers de una request
// real, pero RecordDelivery también se llama desde el dispatcher con mapas armados a mano
// (webhook.go:71 los pasa por webhookJob), y ahí la capitalización no está garantizada.
func TestCollectHeaders_RedactaSinImportarLaCapitalizacion(t *testing.T) {
	req := httptest.NewRequest("POST", "/api/v1/webhooks/x/receive", nil)
	// se escribe en el map directo para saltear la canonicalización de Header.Set
	req.Header["x-gitlab-token"] = []string{"secreto-en-minuscula"}
	req.Header["X-GITLAB-TOKEN"] = []string{"secreto-en-mayuscula"}

	out := collectHeaders(req)

	for k, v := range out {
		require.Equal(t, "[redacted]", v,
			"la clave %q escapó la denylist por su capitalización, y el secreto quedó en el log", k)
	}
}

// Un header presente pero con lista de valores vacía no debe aparecer: hoy el código lo salta, y
// si alguien lo cambia a out[k] = "" el diff se vería inocente.
func TestCollectHeaders_HeaderSinValores_NoAparece(t *testing.T) {
	req := httptest.NewRequest("POST", "/api/v1/webhooks/x/receive", nil)
	req.Header["X-Vacio"] = []string{}
	req.Header.Set("X-Lleno", "v")

	out := collectHeaders(req)

	require.NotContains(t, out, "X-Vacio")
	require.Equal(t, "v", out["X-Lleno"])
}

// La denylist tiene que estar toda en minúscula, o una entrada con mayúsculas nunca matchea
// contra strings.ToLower(k) y queda muerta sin que ningún test lo note.
func TestDenyHeaders_TodasLasClavesEnMinuscula(t *testing.T) {
	for k := range denyHeaders {
		require.Equal(t, k, lowerASCII(k),
			"la entrada %q de denyHeaders no está en minúscula: nunca va a matchear y es una "+
				"redacción que no ocurre", k)
	}
	// el que motivó el ticket, explícito para que un borrado accidental se vea
	require.True(t, denyHeaders["x-gitlab-token"],
		"x-gitlab-token salió de la denylist: para gitlab ese header ES el secreto")
}

func lowerASCII(s string) string {
	b := []byte(s)
	for i := range b {
		if b[i] >= 'A' && b[i] <= 'Z' {
			b[i] += 'a' - 'A'
		}
	}
	return string(b)
}
