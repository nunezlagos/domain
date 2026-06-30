package resend_test

import (
	"context"
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"

	"nunezlagos/domain/internal/mail/resend"
)

func TestResend_SendOK(t *testing.T) {
	var capturedBody map[string]any
	var capturedAuth string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		capturedAuth = r.Header.Get("Authorization")
		b, _ := io.ReadAll(r.Body)
		json.Unmarshal(b, &capturedBody)
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"id":"mock-id"}`))
	}))
	defer srv.Close()

	s := resend.New("re_xxx", "noreply@test.com", slog.Default())
	s.HTTPClient = srv.Client()
	s.BaseURL = srv.URL

	err := s.Send(context.Background(), "user@test.com", "Subject", "Body text")
	require.NoError(t, err)
	require.Equal(t, "Bearer re_xxx", capturedAuth)
	require.Equal(t, "noreply@test.com", capturedBody["from"])
	require.Equal(t, "user@test.com", capturedBody["to"])
	require.Equal(t, "Subject", capturedBody["subject"])
	require.Equal(t, "Body text", capturedBody["text"])
}

func TestResend_SendFails_4xx(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
		w.Write([]byte(`{"error":"unauthorized"}`))
	}))
	defer srv.Close()

	s := resend.New("re_bad", "noreply@test.com", slog.Default())
	s.HTTPClient = srv.Client()
	s.BaseURL = srv.URL

	err := s.Send(context.Background(), "user@test.com", "S", "B")
	require.Error(t, err)
	require.Contains(t, err.Error(), "resend: 401")
}

func TestResend_RetriesOn5xx(t *testing.T) {
	attempts := 0
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		attempts++
		if attempts <= 2 {
			w.WriteHeader(http.StatusServiceUnavailable)
			return
		}
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"id":"ok"}`))
	}))
	defer srv.Close()

	s := resend.New("re_xxx", "noreply@test.com", slog.Default())
	s.HTTPClient = srv.Client()
	s.BaseURL = srv.URL

	err := s.Send(context.Background(), "user@test.com", "S", "B")
	require.NoError(t, err)
	require.Equal(t, 3, attempts)
}

func TestResend_AllRetriesFail(t *testing.T) {
	attempts := 0
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		attempts++
		w.WriteHeader(http.StatusServiceUnavailable)
	}))
	defer srv.Close()

	s := resend.New("re_xxx", "noreply@test.com", slog.Default())
	s.HTTPClient = srv.Client()
	s.BaseURL = srv.URL

	err := s.Send(context.Background(), "user@test.com", "S", "B")
	require.Error(t, err)
	require.Equal(t, 3, attempts)
}

func TestResend_NoLeakAPIKey(t *testing.T) {
	var logBuf strings.Builder
	logger := slog.New(slog.NewTextHandler(&logBuf, &slog.HandlerOptions{Level: slog.LevelInfo}))

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"id":"mock-id"}`))
	}))
	defer srv.Close()

	s := resend.New("re_secret_key_123", "noreply@test.com", logger)
	s.HTTPClient = srv.Client()
	s.BaseURL = srv.URL

	err := s.Send(context.Background(), "user@test.com", "Hola", "Body")
	require.NoError(t, err)

	logOutput := logBuf.String()
	require.NotContains(t, logOutput, "re_secret_key_123")
}

func TestResend_CheckURL(t *testing.T) {
	require.Equal(t, "https://api.resend.com/emails", resend.DefaultBaseURL)
}

func TestResend_WithCustomLogger(t *testing.T) {
	var logBuf strings.Builder
	logger := slog.New(slog.NewTextHandler(&logBuf, &slog.HandlerOptions{Level: slog.LevelInfo}))
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"id":"m"}`))
	}))
	defer srv.Close()

	s := resend.New("re_xxx", "noreply@test.com", logger)
	s.HTTPClient = srv.Client()
	s.BaseURL = srv.URL

	err := s.Send(context.Background(), "a@b.com", "Hi", "Body")
	require.NoError(t, err)
	require.Contains(t, logBuf.String(), "email sent")
	require.Contains(t, logBuf.String(), "resend")
}
