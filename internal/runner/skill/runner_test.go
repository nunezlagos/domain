package skillrunner

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"

	skillsvc "nunezlagos/domain/internal/service/skill"
)

func TestExecute_Prompt_Renders(t *testing.T) {
	r := New()
	sk := &skillsvc.Skill{
		ID: uuid.New(), SkillType: skillsvc.TypePrompt,
		Content: "Hola {{name}}, hoy es {{day}}",
	}
	out, err := r.Execute(context.Background(), sk, map[string]any{
		"name": "Mario", "day": "viernes",
	})
	require.NoError(t, err)
	require.Equal(t, "Hola Mario, hoy es viernes", out)
}

func TestExecute_Prompt_MissingVar(t *testing.T) {
	r := New()
	sk := &skillsvc.Skill{
		ID: uuid.New(), SkillType: skillsvc.TypePrompt,
		Content: "Hola {{name}}",
	}
	_, err := r.Execute(context.Background(), sk, map[string]any{})
	require.Error(t, err)
	require.Contains(t, err.Error(), "name")
}

func TestExecute_API_HappyPath(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		require.Equal(t, "GET", r.Method)
		require.Equal(t, "Bearer abc", r.Header.Get("Authorization"))
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"ok":true}`))
	}))
	defer srv.Close()

	cfg := apiConfig{
		URL:     srv.URL + "/foo",
		Method:  "GET",
		Headers: map[string]string{"Authorization": "Bearer {{token}}"},
	}
	cfgJSON, _ := json.Marshal(cfg)

	r := New()
	sk := &skillsvc.Skill{
		ID: uuid.New(), SkillType: skillsvc.TypeAPI,
		Content: string(cfgJSON),
	}
	out, err := r.Execute(context.Background(), sk, map[string]any{"token": "abc"})
	require.NoError(t, err)
	require.Contains(t, out, `"status_code":200`)
	require.Contains(t, out, `\"ok\":true`)
}

func TestExecute_API_AllowlistHostRejected(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(200)
	}))
	defer srv.Close()

	cfg := apiConfig{URL: srv.URL, Method: "GET"}
	cfgJSON, _ := json.Marshal(cfg)

	r := New()
	r.AllowedHosts = map[string]bool{"api.example.com": true}
	sk := &skillsvc.Skill{
		ID: uuid.New(), SkillType: skillsvc.TypeAPI, Content: string(cfgJSON),
	}
	_, err := r.Execute(context.Background(), sk, nil)
	require.ErrorIs(t, err, ErrURLNotAllowed)
}

func TestExecute_API_InvalidURLScheme(t *testing.T) {
	cfg := apiConfig{URL: "file:///etc/passwd", Method: "GET"}
	cfgJSON, _ := json.Marshal(cfg)
	r := New()
	sk := &skillsvc.Skill{
		ID: uuid.New(), SkillType: skillsvc.TypeAPI, Content: string(cfgJSON),
	}
	_, err := r.Execute(context.Background(), sk, nil)
	require.Error(t, err)
	require.Contains(t, err.Error(), "http/https schemes")
}

func TestExecute_API_BadJSON(t *testing.T) {
	r := New()
	sk := &skillsvc.Skill{
		ID: uuid.New(), SkillType: skillsvc.TypeAPI,
		Content: "not a json",
	}
	_, err := r.Execute(context.Background(), sk, nil)
	require.ErrorIs(t, err, ErrInvalidConfig)
}

func TestExecute_CodeNotImplemented(t *testing.T) {
	r := New()
	sk := &skillsvc.Skill{
		ID: uuid.New(), SkillType: skillsvc.TypeCode,
		Content: "print('hola')",
	}
	_, err := r.Execute(context.Background(), sk, nil)
	require.ErrorIs(t, err, ErrNotImplemented)
	require.Contains(t, err.Error(), "HU-11.1")
}

func TestExecute_MCPToolNotImplemented(t *testing.T) {
	r := New()
	sk := &skillsvc.Skill{
		ID: uuid.New(), SkillType: skillsvc.TypeMCPTool, Content: "{}",
	}
	_, err := r.Execute(context.Background(), sk, nil)
	require.ErrorIs(t, err, ErrNotImplemented)
}

// Sabotaje: URL templated con var debe respetar allowlist DESPUÉS del render
// (no antes — sino el allowlist es trivialmente bypaseable).
func TestSabotage_API_AllowlistEvaluatedAfterTemplating(t *testing.T) {
	cfg := apiConfig{URL: "http://{{host}}/path", Method: "GET"}
	cfgJSON, _ := json.Marshal(cfg)

	r := New()
	r.AllowedHosts = map[string]bool{"trusted.example": true}
	sk := &skillsvc.Skill{
		ID: uuid.New(), SkillType: skillsvc.TypeAPI, Content: string(cfgJSON),
	}
	// Si el atacante pasa host="evil.example", debe ser rechazado tras render
	_, err := r.Execute(context.Background(), sk, map[string]any{"host": "evil.example"})
	require.ErrorIs(t, err, ErrURLNotAllowed)
}

// Sabotaje: response body > 1MB se trunca (LimitReader)
func TestSabotage_API_BodyCapped1MB(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte(strings.Repeat("x", 2<<20))) // 2MB
	}))
	defer srv.Close()
	cfg := apiConfig{URL: srv.URL, Method: "GET"}
	cfgJSON, _ := json.Marshal(cfg)
	r := New()
	sk := &skillsvc.Skill{
		ID: uuid.New(), SkillType: skillsvc.TypeAPI, Content: string(cfgJSON),
	}
	out, err := r.Execute(context.Background(), sk, nil)
	require.NoError(t, err)
	// body es 1MB chars, total JSON output debe ser ~1MB no 2MB
	require.Less(t, len(out), 2<<20)
}
