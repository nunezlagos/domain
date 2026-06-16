package handler

import (
	"encoding/json"
	"errors"
	"net/http"
	"strings"

	"nunezlagos/domain/internal/auth/apikey"
	"nunezlagos/domain/internal/auth/session"
	"nunezlagos/domain/internal/store/txctx"
)

// REQ-72: endpoints REST de login web.
//
//   POST /api/v1/auth/login         {email,password} → {temp_token, user, roles[]}
//   POST /api/v1/auth/select-role   {temp_token, role_slug} → {session_token, expires_at}
//   GET  /api/v1/auth/me            (Bearer sess_*) → {user, active_role}
//   POST /api/v1/auth/logout        (Bearer sess_*) → {ok:true}
//
// El middleware acepta Bearer api key (domk_*) o session token (sess_*).
// El handler /me y /logout sólo aceptan session tokens.

type loginReq struct {
	Email    string `json:"email"`
	Password string `json:"password"`
}

type loginResp struct {
	TempToken string          `json:"temp_token"`
	User      session.User    `json:"user"`
	Roles     []session.Role  `json:"roles"`
}

func (a *API) authLogin(w http.ResponseWriter, r *http.Request) {
	if a.AuthSessionService == nil {
		writeJSON(w, http.StatusServiceUnavailable, map[string]string{"error": "session service no configurado"})
		return
	}
	var in loginReq
	if err := json.NewDecoder(r.Body).Decode(&in); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "json inválido"})
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
		// Defensa anti-enumeración: mismo mensaje para todas las fallas.
		writeJSON(w, http.StatusUnauthorized, map[string]string{"error": "credenciales inválidas"})
		return
	}
	writeJSON(w, http.StatusOK, loginResp{
		TempToken: res.TempToken,
		User:      res.User,
		Roles:     res.Roles,
	})
}

type selectRoleReq struct {
	TempToken string `json:"temp_token"`
	RoleSlug  string `json:"role_slug"`
}

type selectRoleResp struct {
	SessionToken string `json:"session_token"`
	ExpiresAt    string `json:"expires_at"`
	User         session.User `json:"user"`
	Role         session.Role `json:"role"`
}

func (a *API) authSelectRole(w http.ResponseWriter, r *http.Request) {
	if a.AuthSessionService == nil {
		writeJSON(w, http.StatusServiceUnavailable, map[string]string{"error": "session service no configurado"})
		return
	}
	var in selectRoleReq
	if err := json.NewDecoder(r.Body).Decode(&in); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "json inválido"})
		return
	}
	if in.TempToken == "" || in.RoleSlug == "" {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "temp_token y role_slug requeridos"})
		return
	}
	res, err := a.AuthSessionService.SelectRole(r.Context(), in.TempToken, in.RoleSlug,
		r.Header.Get("User-Agent"), realIP(r))
	if err != nil {
		switch {
		case errors.Is(err, session.ErrRoleNotGranted):
			writeJSON(w, http.StatusForbidden, map[string]string{"error": "no tenés ese rol"})
		default:
			writeJSON(w, http.StatusUnauthorized, map[string]string{"error": "temp_token inválido o expirado"})
		}
		return
	}
	writeJSON(w, http.StatusOK, selectRoleResp{
		SessionToken: res.Token,
		ExpiresAt:    res.ExpiresAt.UTC().Format("2006-01-02T15:04:05Z"),
		User:         res.User,
		Role:         res.Role,
	})
}

type meResp struct {
	User       session.User `json:"user"`
	ActiveRole session.Role `json:"active_role"`
}

func (a *API) authMe(w http.ResponseWriter, r *http.Request) {
	active, ok := SessionFromContext(r.Context())
	if !ok {
		writeJSON(w, http.StatusUnauthorized, map[string]string{"error": "endpoint requiere session token"})
		return
	}
	// Cargar email/name del user desde DB. Usar la tx del ctx para
	// que herede el SET LOCAL app.current_org_id del middleware (sino
	// RLS filtra el row porque a.Pool no tiene contexto multi-tenant).
	u := session.User{ID: active.UserID, OrganizationID: active.OrganizationID}
	if tx := txctx.TxFromContext(r.Context()); tx != nil {
		_ = tx.QueryRow(r.Context(),
			`SELECT email, name FROM users WHERE id = $1 AND deleted_at IS NULL`,
			active.UserID,
		).Scan(&u.Email, &u.Name)
	} else if a.Pool != nil {
		_ = a.Pool.QueryRow(r.Context(),
			`SELECT email, name FROM users WHERE id = $1 AND deleted_at IS NULL`,
			active.UserID,
		).Scan(&u.Email, &u.Name)
	}
	writeJSON(w, http.StatusOK, meResp{
		User:       u,
		ActiveRole: active.Role,
	})
}

// REQ-78 POST /api/v1/auth/refresh: extiende la sesión activa por otro
// SessionTTL (8h). Útil para que el dashboard no obligue a re-login si
// el usuario sigue activo. Devuelve el nuevo expires_at + datos del
// usuario y rol activo.
func (a *API) authRefresh(w http.ResponseWriter, r *http.Request) {
	if a.AuthSessionService == nil {
		writeJSON(w, http.StatusServiceUnavailable, map[string]string{"error": "session service no configurado"})
		return
	}
	header := r.Header.Get("Authorization")
	const prefix = "Bearer "
	if !strings.HasPrefix(header, prefix) {
		writeJSON(w, http.StatusUnauthorized, map[string]string{"error": "Bearer token requerido"})
		return
	}
	tok := strings.TrimSpace(strings.TrimPrefix(header, prefix))
	if !strings.HasPrefix(tok, session.TokenPrefix) {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "no es un session token"})
		return
	}
	active, expires, err := a.AuthSessionService.Refresh(r.Context(), tok)
	if err != nil {
		writeJSON(w, http.StatusUnauthorized, map[string]string{"error": "token inválido o expirado"})
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"expires_at": expires.UTC().Format("2006-01-02T15:04:05Z"),
		"user":       session.User{ID: active.UserID, OrganizationID: active.OrganizationID},
		"active_role": active.Role,
	})
}

func (a *API) authLogout(w http.ResponseWriter, r *http.Request) {
	if a.AuthSessionService == nil {
		writeJSON(w, http.StatusServiceUnavailable, map[string]string{"error": "session service no configurado"})
		return
	}
	header := r.Header.Get("Authorization")
	const prefix = "Bearer "
	if !strings.HasPrefix(header, prefix) {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "Bearer token requerido"})
		return
	}
	tok := strings.TrimSpace(strings.TrimPrefix(header, prefix))
	if !strings.HasPrefix(tok, session.TokenPrefix) {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "no es un session token"})
		return
	}
	if err := a.AuthSessionService.Logout(r.Context(), tok); err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "logout failed"})
		return
	}
	writeJSON(w, http.StatusOK, map[string]bool{"ok": true})
}

// meRolesResp payload de GET /api/v1/me/roles. Devuelve el user + la
// lista completa de roles disponibles (para el switcher del header).
// HU-41.1: el header del admin necesita listar los roles del user para
// permitir cambiar de contexto (ej: admin → owner → super_admin).
type meRolesResp struct {
	User  session.User   `json:"user"`
	Roles []session.Role `json:"roles"`
}

// authMeRoles GET /api/v1/me/roles → {user, roles[]}. Requiere session token.
// REQ-72 + HU-41.1: el switcher de rol del header llama este endpoint al boot
// para popular el FormSelectDirective de "rol activo".
func (a *API) authMeRoles(w http.ResponseWriter, r *http.Request) {
	active, ok := SessionFromContext(r.Context())
	if !ok {
		writeJSON(w, http.StatusUnauthorized, map[string]string{"error": "endpoint requiere session token"})
		return
	}
	if a.AuthSessionService == nil {
		writeJSON(w, http.StatusServiceUnavailable, map[string]string{"error": "session service no configurado"})
		return
	}
	roles, err := a.AuthSessionService.RolesOf(r.Context(), active.UserID)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "no se pudieron cargar los roles"})
		return
	}
	// Hidratar email/name del user
	u := session.User{ID: active.UserID, OrganizationID: active.OrganizationID}
	if tx := txctx.TxFromContext(r.Context()); tx != nil {
		_ = tx.QueryRow(r.Context(),
			`SELECT email, name FROM users WHERE id = $1 AND deleted_at IS NULL`,
			active.UserID,
		).Scan(&u.Email, &u.Name)
	}
	writeJSON(w, http.StatusOK, meRolesResp{User: u, Roles: roles})
}

func realIP(r *http.Request) string {
	if v := r.Header.Get("X-Forwarded-For"); v != "" {
		// solo la primera (la del cliente real, no la cadena de proxies).
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

// silence unused import warnings (apikey importado por compat con
// otros archivos del package que usan FromContext).
var _ = apikey.FromContext
