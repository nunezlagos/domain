// Package secrets — HU-25.10 password rotation para roles Postgres.
//
// Modelo dual-credentials (sin downtime):
//   1. ALTER ROLE app_user PASSWORD 'new_pwd' (Postgres acepta SCRAM con un solo password activo)
//   2. Operator actualiza K8s Secret con el nuevo password
//   3. Rolling deploy → pods nuevos toman el nuevo password
//   4. Verificar que todos los pods están healthy
//
// Esta herramienta cubre el paso 1: genera password seguro + ALTER ROLE.
// Los pasos 2-4 son responsabilidad del operator (K8s, AWS Secrets Manager).
package secrets

import (
	"context"
	"crypto/rand"
	"encoding/base64"
	"errors"
	"fmt"

	"github.com/jackc/pgx/v5/pgxpool"
)

// GeneratePassword genera N bytes random base64url-safe sin padding.
// 32 bytes → ~43 chars, suficiente entropía (256 bits).
func GeneratePassword(nBytes int) (string, error) {
	if nBytes < 16 {
		return "", errors.New("min 16 bytes for security")
	}
	buf := make([]byte, nBytes)
	if _, err := rand.Read(buf); err != nil {
		return "", fmt.Errorf("rand read: %w", err)
	}
	return base64.RawURLEncoding.EncodeToString(buf), nil
}

// Rotator rota passwords de roles Postgres.
type Rotator struct {
	// AdminPool con un user que tenga permisos ALTER ROLE (superuser o
	// owner del role target). En prod: usar app_admin solo si tiene CREATEROLE.
	AdminPool *pgxpool.Pool
}

// RotateRole ALTER ROLE <role> PASSWORD '<new_pwd>'. Retorna el nuevo password.
// El caller es responsable de propagarlo al Secret Manager / K8s.
func (r *Rotator) RotateRole(ctx context.Context, role string) (string, error) {
	if !isValidRoleName(role) {
		return "", errors.New("invalid role name (must be lowercase alphanumeric + underscore)")
	}
	newPass, err := GeneratePassword(32)
	if err != nil {
		return "", err
	}
	// pgx no acepta parameters en DDL — quoteIdentifier + literal escape.
	sql := fmt.Sprintf("ALTER ROLE %s PASSWORD %s",
		pgQuoteIdent(role), pgQuoteLiteral(newPass))
	if _, err := r.AdminPool.Exec(ctx, sql); err != nil {
		return "", fmt.Errorf("ALTER ROLE %s: %w", role, err)
	}
	return newPass, nil
}

// isValidRoleName: solo lowercase + digits + underscore. Bloquea SQL injection
// además del quote en pgQuoteIdent.
func isValidRoleName(s string) bool {
	if s == "" || len(s) > 63 {
		return false
	}
	for _, c := range s {
		if !((c >= 'a' && c <= 'z') || (c >= '0' && c <= '9') || c == '_') {
			return false
		}
	}
	return true
}

// pgQuoteIdent escapa un identifier (table/role/column).
func pgQuoteIdent(s string) string {
	out := make([]byte, 0, len(s)+2)
	out = append(out, '"')
	for i := 0; i < len(s); i++ {
		if s[i] == '"' {
			out = append(out, '"', '"')
		} else {
			out = append(out, s[i])
		}
	}
	out = append(out, '"')
	return string(out)
}

// pgQuoteLiteral escapa un string literal con sintaxis dollar-quoted para evitar
// problemas con caracteres de control en passwords base64.
func pgQuoteLiteral(s string) string {
	// Usar dollar-quoted con tag aleatorio improbable: $domain_pw$...$domain_pw$
	const tag = "domain_pw"
	return "$" + tag + "$" + s + "$" + tag + "$"
}
