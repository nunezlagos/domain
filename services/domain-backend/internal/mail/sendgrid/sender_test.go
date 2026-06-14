package sendgrid_test

import (
	"context"
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"testing"

	"nunezlagos/domain/internal/mail/sendgrid"

	"github.com/stretchr/testify/require"
)

func TestSendGrid_SendOK(t *testing.T) {
	var capturedBody map[string]any
	var capturedAuth string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		capturedAuth = r.Header.Get("Authorization")
		b, _ := io.ReadAll(r.Body)
		json.Unmarshal(b, &capturedBody)
		w.WriteHeader(http.StatusAccepted)
	}))
	defer srv.Close()

	s := sendgrid.New("SG.xxx", "noreply@test.com", slog.Default())
	s.HTTPClient = srv.Client()
	s.BaseURL = srv.URL

	err := s.Send(context.Background(), "user@test.com", "Subject", "Body")
	require.NoError(t, err)
	require.Equal(t, "Bearer SG.xxx", capturedAuth)

	personalizations := capturedBody["personalizations"].([]any)
	require.Len(t, personalizations, 1)
	to := personalizations[0].(map[string]any)["to"].([]any)
	require.Equal(t, "user@test.com", to[0].(map[string]any)["email"])
	require.Equal(t, "Subject", capturedBody["subject"].(map[string]any)["value"])
	require.Equal(t, "Body", capturedBody["content"].([]any)[0].(map[string]any)["value"])
}

func TestSendGrid_SendFails_4xx(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusBadRequest)
		w.Write([]byte(`{"errors":[{"message":"bad request"}]}`))
	}))
	defer srv.Close()

	s := sendgrid.New("SG.bad", "noreply@test.com", slog.Default())
	s.HTTPClient = srv.Client()
	s.BaseURL = srv.URL

	err := s.Send(context.Background(), "user@test.com", "S", "B")
	require.Error(t, err)
	require.Contains(t, err.Error(), "sendgrid: 400")
}

func TestSendGrid_CheckURL(t *testing.T) {
	require.Equal(t, "https://api.sendgrid.com/v3/mail/send", sendgrid.DefaultBaseURL)
}
