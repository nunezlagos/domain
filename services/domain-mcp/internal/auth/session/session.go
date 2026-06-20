// Package session: login con user+password y emisión de tokens
// opacos persistentes. REQ-72.
//
// Diseño:
//   - Login: email + password → temp_token (1 min) + lista roles
//   - SelectRole: temp_token + role_slug → session_token (8h por default)
//   - Token format: "sess_" + base32(32 random bytes). Plano nunca se
//     persiste — solo SHA-256(token).
//   - bcrypt cost=12 para password hashing (~250ms en CPU típica, OK).
//   - Revocación: 1 UPDATE auth_sessions SET revoked_at=NOW().
//
// Hash de token vs JWT: elegimos opaque + persist porque hacer logout
// con JWT requiere blacklist; con esto basta DELETE/UPDATE.
package session

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base32"
	"errors"
	"net"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"golang.org/x/crypto/bcrypt"
)

const (
	TokenPrefix      = "sess_"
	BcryptCost       = 12
	TempTokenTTL     = 15 * time.Minute
	SessionTTL       = 8 * time.Hour
	TempTokenSecret  = "domain-temp-token-v1" // prefix interno, no es secreto criptográfico
)

var (
	ErrInvalidCredentials = errors.New("session: credenciales inválidas")
	ErrUserHasNoRoles     = errors.New("session: el usuario no tiene roles asignados")
	ErrRoleNotGranted     = errors.New("session: el usuario no tiene ese rol")
	ErrTokenInvalid       = errors.New("session: token inválido o expirado")
)

type Service struct {
	Pool *pgxpool.Pool
	// Tablas con BYPASSRLS porque login cruza varios scopes
	// (users de cualquier org). Inyectar pool app_admin.
	AuthPool *pgxpool.Pool
	// Cripto reloj — sobrescribible para tests.
	Now func() time.Time
}

func New(authPool *pgxpool.Pool) *Service {
	return &Service{AuthPool: authPool, Now: time.Now}
}

func (s *Service) now() time.Time {
	if s.Now == nil {
		return time.Now()
	}
	return s.Now()
}

// User es el resultado del login (user + lista de roles disponibles).
type User struct {
	ID             uuid.UUID `json:"id"`
	OrganizationID uuid.UUID `json:"organization_id"`
	Email          string    `json:"email"`
	Name           string    `json:"name"`
}

type Role struct {
	ID          uuid.UUID `json:"id"`
	Slug        string    `json:"slug"`
	Name        string    `json:"name"`
	Permissions []string  `json:"permissions"`
}

type LoginResult struct {
	TempToken  string
	User       User
	Roles      []Role
}

// LoginInput agrega metadatos del request (ip, user_agent) para el
// audit log. REQ-79.
type LoginInput struct {
	Email     string
	Password  string
	IP        string
	UserAgent string
}

// Login verifica email+password contra users.password_hash. Devuelve un
// temp_token efímero (1 min) que el frontend canjeará por session
// vía SelectRole(role_slug).
//
// Defensa anti-enumeración: cualquier error (user no existe, password
// inválido, sin password seteado) devuelve ErrInvalidCredentials.
// El temp_token sale en logs SOLO con prefijo (no plano completo).
//
// REQ-79: cada intento (éxito o fallo) deja entry en auth_events.
func (s *Service) Login(ctx context.Context, email, password string) (*LoginResult, error) {
	return s.LoginWithMeta(ctx, LoginInput{Email: email, Password: password})
}

func (s *Service) LoginWithMeta(ctx context.Context, in LoginInput) (*LoginResult, error) {
	var u User
	var hash []byte
	err := s.AuthPool.QueryRow(ctx,
		// ISSUE-21.6: organization_id dropeado en Fase C; omitido del SELECT.
		`SELECT id, email, name, password_hash
		   FROM users
		   WHERE email = $1 AND deleted_at IS NULL`,
		in.Email,
	).Scan(&u.ID, &u.Email, &u.Name, &hash)
	if errors.Is(err, pgx.ErrNoRows) || len(hash) == 0 {
		// constant-time: hace bcrypt aunque no haya hash, para no leak
		// timing.
		_ = bcrypt.CompareHashAndPassword([]byte("$2a$12$dummyhashfortimingsafety"), []byte(in.Password))
		s.audit(ctx, authEvent{Kind: "login_attempt", EmailAttempted: in.Email,
			Success: false, Reason: "invalid_credentials", IP: in.IP, UserAgent: in.UserAgent})
		return nil, ErrInvalidCredentials
	}
	if err != nil {
		return nil, err
	}
	if bcryptErr := bcrypt.CompareHashAndPassword(hash, []byte(in.Password)); bcryptErr != nil {
		s.audit(ctx, authEvent{Kind: "login_attempt", UserID: &u.ID, OrgID: &u.OrganizationID,
			EmailAttempted: in.Email, Success: false, Reason: "invalid_credentials",
			IP: in.IP, UserAgent: in.UserAgent})
		return nil, ErrInvalidCredentials
	}

	roles, err := s.RolesOf(ctx, u.ID)
	if err != nil {
		return nil, err
	}
	if len(roles) == 0 {
		s.audit(ctx, authEvent{Kind: "login_attempt", UserID: &u.ID, OrgID: &u.OrganizationID,
			EmailAttempted: in.Email, Success: false, Reason: "user_has_no_roles",
			IP: in.IP, UserAgent: in.UserAgent})
		return nil, ErrUserHasNoRoles
	}

	tempToken, err := generateTempToken(u.ID, s.now().Add(TempTokenTTL))
	if err != nil {
		return nil, err
	}
	s.audit(ctx, authEvent{Kind: "login_success", UserID: &u.ID, OrgID: &u.OrganizationID,
		EmailAttempted: in.Email, Success: true, Reason: "ok",
		IP: in.IP, UserAgent: in.UserAgent})
	return &LoginResult{
		TempToken: tempToken,
		User:      u,
		Roles:     roles,
	}, nil
}

// RolesOf devuelve todos los roles del user. Vacío = no puede loguear.
func (s *Service) RolesOf(ctx context.Context, userID uuid.UUID) ([]Role, error) {
	rows, err := s.AuthPool.Query(ctx,
		`SELECT r.id, r.slug, r.name, r.permissions
		   FROM user_roles ur
		   JOIN roles r ON r.id = ur.role_id
		   WHERE ur.user_id = $1
		   ORDER BY r.slug`,
		userID,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []Role
	for rows.Next() {
		var r Role
		if err := rows.Scan(&r.ID, &r.Slug, &r.Name, &r.Permissions); err != nil {
			return nil, err
		}
		out = append(out, r)
	}
	return out, rows.Err()
}

// SelectRole canjea un temp_token + role_slug por session_token.
// Verifica que el user tenga ese rol asignado.
type SelectResult struct {
	Token       string
	SessionID   uuid.UUID
	User        User
	Role        Role
	ExpiresAt   time.Time
}

func (s *Service) SelectRole(ctx context.Context, tempToken, roleSlug, userAgent, ip string) (*SelectResult, error) {
	userID, err := parseTempToken(tempToken, s.now())
	if err != nil {
		s.audit(ctx, authEvent{Kind: "role_select_failed", Success: false,
			Reason: "token_invalid", IP: ip, UserAgent: userAgent})
		return nil, ErrTokenInvalid
	}
	// Recargar user + verificar rol.
	var u User
	err = s.AuthPool.QueryRow(ctx,
		// ISSUE-21.6: organization_id dropeado en Fase C; omitido del SELECT.
		`SELECT id, email, name
		   FROM users WHERE id = $1 AND deleted_at IS NULL`,
		userID,
	).Scan(&u.ID, &u.Email, &u.Name)
	if err != nil {
		s.audit(ctx, authEvent{Kind: "role_select_failed", UserID: &userID,
			Success: false, Reason: "user_not_found", IP: ip, UserAgent: userAgent})
		return nil, ErrTokenInvalid
	}
	var r Role
	err = s.AuthPool.QueryRow(ctx,
		`SELECT r.id, r.slug, r.name, r.permissions
		   FROM user_roles ur
		   JOIN roles r ON r.id = ur.role_id
		   WHERE ur.user_id = $1 AND r.slug = $2`,
		userID, roleSlug,
	).Scan(&r.ID, &r.Slug, &r.Name, &r.Permissions)
	if err != nil {
		s.audit(ctx, authEvent{Kind: "role_select_failed", UserID: &userID,
			OrgID: &u.OrganizationID, EmailAttempted: u.Email,
			Success: false, Reason: "role_not_granted",
			IP: ip, UserAgent: userAgent})
		return nil, ErrRoleNotGranted
	}

	// Generar session token + persistir.
	plain, err := newSessionToken()
	if err != nil {
		return nil, err
	}
	hash := hashToken(plain)
	expires := s.now().Add(SessionTTL)
	var sessID uuid.UUID
	ipVal := parseIP(ip)
	// ISSUE-21.6 Fase C: organization_id dropeado de auth_sessions
	// (migration 000142). INSERT sin la columna.
	err = s.AuthPool.QueryRow(ctx,
		`INSERT INTO auth_sessions
		   (user_id, active_role_id, token_hash,
		    user_agent, ip, expires_at)
		 VALUES ($1,$2,$3,$4,$5,$6)
		 RETURNING id`,
		u.ID, r.ID, hash, userAgent, ipVal, expires,
	).Scan(&sessID)
	if err != nil {
		return nil, err
	}
	s.audit(ctx, authEvent{Kind: "role_selected", UserID: &u.ID, OrgID: &u.OrganizationID,
		EmailAttempted: u.Email, Success: true, Reason: "ok",
		IP: ip, UserAgent: userAgent, SessionID: &sessID})
	return &SelectResult{
		Token:     plain,
		SessionID: sessID,
		User:      u,
		Role:      r,
		ExpiresAt: expires,
	}, nil
}

// Resolve resuelve un session_token plano a su Principal-like. Lo usa
// el middleware al recibir Bearer sess_<...>.
type Active struct {
	UserID         uuid.UUID
	OrganizationID uuid.UUID
	Role           Role
	SessionID      uuid.UUID
}

func (s *Service) Resolve(ctx context.Context, plainToken string) (*Active, error) {
	hash := hashToken(plainToken)
	row := s.AuthPool.QueryRow(ctx,
		// ISSUE-21.6: organization_id dropeado en Fase C; omitido del SELECT.
		`SELECT s.id, s.user_id, s.expires_at,
		        r.id, r.slug, r.name, r.permissions
		   FROM auth_sessions s
		   JOIN roles r ON r.id = s.active_role_id
		   WHERE s.token_hash = $1 AND s.revoked_at IS NULL`,
		hash,
	)
	var a Active
	var expiresAt time.Time
	if err := row.Scan(
		&a.SessionID, &a.UserID, &expiresAt,
		&a.Role.ID, &a.Role.Slug, &a.Role.Name, &a.Role.Permissions,
	); err != nil {
		return nil, ErrTokenInvalid
	}
	if expiresAt.Before(s.now()) {
		return nil, ErrTokenInvalid
	}
	// last_used_at refresh — best effort, no bloqueante.
	go func() {
		ctxBg, cancel := context.WithTimeout(context.Background(), 2*time.Second)
		defer cancel()
		_, _ = s.AuthPool.Exec(ctxBg,
			`UPDATE auth_sessions SET last_used_at=NOW() WHERE id=$1`, a.SessionID)
	}()
	return &a, nil
}

// Refresh extiende expires_at de la sesión activa por otro SessionTTL.
// Idempotente: si el token está revocado o expirado, devuelve ErrTokenInvalid.
// REQ-78.
func (s *Service) Refresh(ctx context.Context, plainToken string) (*Active, time.Time, error) {
	active, err := s.Resolve(ctx, plainToken)
	if err != nil {
		s.audit(ctx, authEvent{Kind: "refresh_failed", Success: false, Reason: "token_invalid"})
		return nil, time.Time{}, err
	}
	newExpiry := s.now().Add(SessionTTL)
	if _, err := s.AuthPool.Exec(ctx,
		`UPDATE auth_sessions SET expires_at = $2, last_used_at = NOW()
		   WHERE id = $1 AND revoked_at IS NULL`,
		active.SessionID, newExpiry,
	); err != nil {
		return nil, time.Time{}, err
	}
	s.audit(ctx, authEvent{Kind: "refreshed", UserID: &active.UserID, OrgID: &active.OrganizationID,
		Success: true, Reason: "ok", SessionID: &active.SessionID})
	return active, newExpiry, nil
}

// Logout marca la sesión como revocada. Idempotente.
func (s *Service) Logout(ctx context.Context, plainToken string) error {
	hash := hashToken(plainToken)
	// Tomamos los datos del session ANTES de revocar para que el audit
	// log sepa quién hizo logout. Si el token ya no existe es no-op.
	// ISSUE-21.6: organization_id dropeado en Fase C; omitido del SELECT.
	var sessID, userID uuid.UUID
	_ = s.AuthPool.QueryRow(ctx,
		`SELECT id, user_id FROM auth_sessions WHERE token_hash = $1`,
		hash,
	).Scan(&sessID, &userID)
	_, err := s.AuthPool.Exec(ctx,
		`UPDATE auth_sessions
		   SET revoked_at = NOW()
		   WHERE token_hash = $1 AND revoked_at IS NULL`,
		hash,
	)
	if err == nil && sessID != uuid.Nil {
		// ISSUE-21.6: orgID se omite del audit (single-org, no se selecciona
		// de auth_sessions.organization_id).
		s.audit(ctx, authEvent{Kind: "logout", UserID: &userID, OrgID: nil,
			Success: true, Reason: "ok", SessionID: &sessID})
	}
	return err
}

// SetPassword setea/actualiza la contraseña del user. Usado por el
// subcomando CLI `domain admin-passwd <email>` para bootstrap.
func (s *Service) SetPassword(ctx context.Context, email, newPassword string) error {
	hash, err := bcrypt.GenerateFromPassword([]byte(newPassword), BcryptCost)
	if err != nil {
		return err
	}
	ct, err := s.AuthPool.Exec(ctx,
		`UPDATE users SET password_hash = $2, password_set_at = NOW()
		   WHERE email = $1 AND deleted_at IS NULL`,
		email, hash,
	)
	if err != nil {
		return err
	}
	if ct.RowsAffected() == 0 {
		return errors.New("user no encontrado")
	}
	return nil
}

// GrantRole asigna un rol al user. Idempotente.
func (s *Service) GrantRole(ctx context.Context, userEmail, roleSlug string, grantedBy *uuid.UUID) error {
	var userID, roleID uuid.UUID
	if err := s.AuthPool.QueryRow(ctx,
		`SELECT id FROM users WHERE email=$1 AND deleted_at IS NULL`,
		userEmail).Scan(&userID); err != nil {
		return errors.New("user no encontrado")
	}
	if err := s.AuthPool.QueryRow(ctx,
		`SELECT id FROM roles WHERE slug=$1`, roleSlug).Scan(&roleID); err != nil {
		return errors.New("role no encontrado")
	}
	_, err := s.AuthPool.Exec(ctx,
		`INSERT INTO user_roles (user_id, role_id, granted_by)
		 VALUES ($1,$2,$3)
		 ON CONFLICT (user_id, role_id) DO NOTHING`,
		userID, roleID, grantedBy,
	)
	return err
}

// --- audit log REQ-79 ---

type authEvent struct {
	Kind           string
	UserID         *uuid.UUID
	OrgID          *uuid.UUID
	EmailAttempted string
	Success        bool
	Reason         string
	IP             string
	UserAgent      string
	SessionID      *uuid.UUID
}

func (s *Service) audit(ctx context.Context, e authEvent) {
	if s.AuthPool == nil {
		return
	}
	// Best-effort: si falla la inserción no rompemos el login. Hacemos
	// timeout corto para no bloquear el request principal.
	c, cancel := context.WithTimeout(ctx, 2*time.Second)
	defer cancel()
	// ISSUE-21.6: INSERT sin organization_id (la columna se dropea en Fase C).
	_, _ = s.AuthPool.Exec(c,
		`INSERT INTO auth_events
		   (user_id, kind, email_attempted, success,
		    reason, ip, user_agent, session_id)
		 VALUES ($1,$2,NULLIF($3,''),$4,NULLIF($5,''),
		         NULLIF($6,'')::inet,NULLIF($7,''),$8)`,
		e.UserID, e.Kind, e.EmailAttempted, e.Success,
		e.Reason, e.IP, e.UserAgent, e.SessionID,
	)
}

// --- helpers ---

func newSessionToken() (string, error) {
	var buf [32]byte
	if _, err := rand.Read(buf[:]); err != nil {
		return "", err
	}
	return TokenPrefix + base32.StdEncoding.WithPadding(base32.NoPadding).EncodeToString(buf[:]), nil
}

func hashToken(plain string) []byte {
	sum := sha256.Sum256([]byte(plain))
	return sum[:]
}

func parseIP(s string) any {
	if s == "" {
		return nil
	}
	if ip := net.ParseIP(s); ip != nil {
		return ip.String()
	}
	return nil
}

// generateTempToken / parseTempToken: token corto (no persistido) que
// codifica (userID, expiresAt) firmado con SHA-256 + secret interno.
// NO es JWT; es base32(userID || expiresAt || mac).
func generateTempToken(userID uuid.UUID, expires time.Time) (string, error) {
	raw := make([]byte, 16+8)
	copy(raw, userID[:])
	t := expires.Unix()
	for i := 0; i < 8; i++ {
		raw[16+i] = byte(t >> (8 * (7 - i)))
	}
	mac := sha256.Sum256(append([]byte(TempTokenSecret), raw...))
	out := append(raw, mac[:8]...)
	return "tmp_" + base32.StdEncoding.WithPadding(base32.NoPadding).EncodeToString(out), nil
}

func parseTempToken(token string, now time.Time) (uuid.UUID, error) {
	const prefix = "tmp_"
	if len(token) < len(prefix) || token[:len(prefix)] != prefix {
		return uuid.Nil, ErrTokenInvalid
	}
	out, err := base32.StdEncoding.WithPadding(base32.NoPadding).DecodeString(token[len(prefix):])
	if err != nil || len(out) != 32 {
		return uuid.Nil, ErrTokenInvalid
	}
	raw, mac := out[:24], out[24:]
	want := sha256.Sum256(append([]byte(TempTokenSecret), raw...))
	if !equalBytes(want[:8], mac) {
		return uuid.Nil, ErrTokenInvalid
	}
	var userID uuid.UUID
	copy(userID[:], raw[:16])
	var t int64
	for i := 0; i < 8; i++ {
		t = (t << 8) | int64(raw[16+i])
	}
	if time.Unix(t, 0).Before(now) {
		return uuid.Nil, ErrTokenInvalid
	}
	return userID, nil
}

func equalBytes(a, b []byte) bool {
	if len(a) != len(b) {
		return false
	}
	var v byte
	for i := range a {
		v |= a[i] ^ b[i]
	}
	return v == 0
}
