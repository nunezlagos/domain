package org

import (
	"context"
	"errors"
	"testing"

	"github.com/google/uuid"
)

// Tests unitarios sin DB: solo validan los errores de input antes de tocar el
// pool. Para tests con DB (atomicidad, RLS, audit), ver el integration test
// que corre con testcontainers cuando el build tag integration está activo.

func TestAddMemberWithAPIKey_RejectsInvalidEmail(t *testing.T) {
	s := &Service{} // sin Pool: cortamos en validation antes de tocar DB
	cases := []string{
		"",
		"no-arroba",
		"sin-tld@dominio",
		"@nadie.com",
		"espacios @x.com",
	}
	for _, email := range cases {
		t.Run(email, func(t *testing.T) {
			_, err := s.AddMemberWithAPIKey(context.Background(), uuid.New(), uuid.New(), email, "Alice", RoleMember)
			if !errors.Is(err, ErrInvalidEmail) {
				t.Fatalf("email %q: err = %v, want ErrInvalidEmail", email, err)
			}
		})
	}
}

func TestAddMemberWithAPIKey_RejectsInvalidRole(t *testing.T) {
	s := &Service{}
	cases := []string{
		"",
		"superadmin",
		"god",
		"ADMIN", // case-sensitive: solo lowercase
	}
	for _, role := range cases {
		t.Run(role, func(t *testing.T) {
			_, err := s.AddMemberWithAPIKey(context.Background(), uuid.New(), uuid.New(), "ok@example.com", "Alice", role)
			if !errors.Is(err, ErrInvalidRole) {
				t.Fatalf("role %q: err = %v, want ErrInvalidRole", role, err)
			}
		})
	}
}

func TestAddMemberWithAPIKey_RejectsNilOrgID(t *testing.T) {
	s := &Service{}
	_, err := s.AddMemberWithAPIKey(context.Background(), uuid.Nil, uuid.New(), "ok@example.com", "Alice", RoleMember)
	if !errors.Is(err, ErrNotFound) {
		t.Fatalf("err = %v, want ErrNotFound", err)
	}
}

func TestAllowedRoles_CoversAllConstants(t *testing.T) {
	for _, role := range []string{RoleOwner, RoleAdmin, RoleMaintainer, RoleMember, RoleViewer} {
		if !allowedRoles[role] {
			t.Errorf("role %q debería estar en allowedRoles", role)
		}
	}
	// Anti-test: roles no definidos NO deben estar permitidos.
	for _, role := range []string{"god", "superadmin", "ADMIN", ""} {
		if allowedRoles[role] {
			t.Errorf("role %q NO debería estar permitido", role)
		}
	}
}

func TestEmailRegex_Accepts(t *testing.T) {
	good := []string{
		"user@example.com",
		"user.name+tag@subdomain.example.co",
		"a@b.cd",
		"USER@EXAMPLE.COM", // luego se lowercase en el service
	}
	for _, e := range good {
		if !emailRegex.MatchString(e) {
			t.Errorf("%q debería matchear regex", e)
		}
	}
}

func TestEmailRegex_Rejects(t *testing.T) {
	bad := []string{
		"",
		"no-arroba",
		"sin-tld@dominio",
		"@nadie.com",
		"user@.com",
	}
	for _, e := range bad {
		if emailRegex.MatchString(e) {
			t.Errorf("%q no debería matchear regex", e)
		}
	}
}
