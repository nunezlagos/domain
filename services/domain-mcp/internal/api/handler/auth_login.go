package handler

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strings"

	"nunezlagos/domain/internal/auth/session"
)

type loginReq struct {
	Email    string `json:"email"`
	Password string `json:"password"`
}

type loginResp struct {
	TempToken    string         `json:"temp_token"`
	SessionToken string         `json:"session_token,omitempty"`
	ExpiresAt    string         `json:"expires_at,omitempty"`
	User         session.User   `json:"user"`
	Roles        []session.Role `json:"roles"`
}

func (a *API) authLogin(w http.ResponseWriter, r *http.Request) {
	if a.AuthSessionService == nil {
		writeJSON(w, http.StatusServiceUnavailable, map[string]string{"error": "session service no configurado"})
		return
	}
	var in loginReq
	if err := json.NewDecoder(r.Body).Decode(&in); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "json invalido"})
		return
	}
	in.Email = strings.TrimSpace(strings.ToLower(in.Email))
	if in.Email == "" || in.Password == "" {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "email y password requeridos"})
		return
	}
	res, err := a.AuthSessionService.LoginWithMeta(r.Context(), session.LoginInput{
		Email: in.Email, Password: in.Password,
		IP: realIP(r), UserAgent: r.Header.Get("User-Agent"),
	})
	if err != nil {
		writeJSON(w, http.StatusUnauthorized, map[string]string{"error": "credenciales invalidas"})
		return
	}
	resp := loginResp{
		TempToken: res.TempToken,
		User:      res.User,
		Roles:     res.Roles,
	}

	if len(res.Roles) == 1 {
		sel, selErr := a.AuthSessionService.SelectRole(
			r.Context(),
			res.TempToken, res.Roles[0].Slug,
			r.Header.Get("User-Agent"), realIP(r),
		)
		if selErr == nil {
			resp.SessionToken = sel.Token
			resp.ExpiresAt = sel.ExpiresAt.UTC().Format("2006-01-02T15:04:05Z")
		} else {
			fmt.Printf("WARN authLogin SelectRole failed: %v\n", selErr)
		}
	}
	writeJSON(w, http.StatusOK, resp)
}

func realIP(r *http.Request) string {
	if v := r.Header.Get("X-Forwarded-For"); v != "" {
		if idx := strings.Index(v, ","); idx > 0 {
			return strings.TrimSpace(v[:idx])
		}
		return strings.TrimSpace(v)
	}
	if v := r.Header.Get("X-Real-IP"); v != "" {
		return v
	}
	if r.RemoteAddr != "" {
		if idx := strings.LastIndex(r.RemoteAddr, ":"); idx > 0 {
			return r.RemoteAddr[:idx]
		}
	}
	return ""
}
