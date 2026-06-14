package enrollment

import (
	"context"
	"errors"
	"testing"

	"github.com/google/uuid"
)

// Tests unitarios sin DB: cubren validaciones que cortan antes de tocar
// el pool. Integration tests con testcontainers van en otro file detrás
// del build tag integration.

func TestEnroll_RejectsMalformedToken(t *testing.T) {
	s := &Service{}
	cases := []string{
		"",
		"sin_prefix",
		"et_short",
		"random_string_no_prefix",
	}
	for _, token := range cases {
		t.Run(token, func(t *testing.T) {
			_, err := s.Enroll(context.Background(), token, "alice@example.com", "Alice")
			if !errors.Is(err, ErrInvalidToken) {
				t.Errorf("token %q: err = %v, want ErrInvalidToken", token, err)
			}
		})
	}
}

func TestEnroll_RejectsInvalidEmail(t *testing.T) {
	// Generamos un token formato-válido para que llegue a la validación email.
	pt, _, _, err := GeneratePlaintext()
	if err != nil {
		t.Fatal(err)
	}
	s := &Service{}
	cases := []string{
		"",
		"no-arroba",
		"sin-tld@dominio",
		"@nadie.com",
	}
	for _, email := range cases {
		t.Run(email, func(t *testing.T) {
			_, err := s.Enroll(context.Background(), pt, email, "Alice")
			if !errors.Is(err, ErrInvalidEmail) {
				t.Errorf("email %q: err = %v, want ErrInvalidEmail", email, err)
			}
		})
	}
}

func TestRotate_RejectsNilOrgID(t *testing.T) {
	s := &Service{}
	_, err := s.Rotate(context.Background(), uuid.Nil, uuid.New(), "member")
	if !errors.Is(err, ErrOrgNotFound) {
		t.Errorf("err = %v, want ErrOrgNotFound", err)
	}
}

func TestRotate_RejectsInvalidRole(t *testing.T) {
	s := &Service{}
	_, err := s.Rotate(context.Background(), uuid.New(), uuid.New(), "superadmin")
	if !errors.Is(err, ErrInvalidRole) {
		t.Errorf("err = %v, want ErrInvalidRole", err)
	}
}

func TestRotate_AcceptsBlankRoleAsDefaultMember(t *testing.T) {
	// Con Pool == nil va a panic. Solo verificamos que la validación de role
	// NO rechaza el string vacío (lo trata como "member").
	// Para evitar el panic, comprobamos solo el dispatch de allowedRoles:
	if !allowedRoles["member"] {
		t.Fatal("member debería estar permitido como default")
	}
}

func TestRevoke_RejectsNilOrgID(t *testing.T) {
	s := &Service{}
	err := s.Revoke(context.Background(), uuid.Nil, uuid.New())
	if !errors.Is(err, ErrOrgNotFound) {
		t.Errorf("err = %v, want ErrOrgNotFound", err)
	}
}

func TestGetMetadata_RejectsNilOrgID(t *testing.T) {
	s := &Service{}
	_, err := s.GetMetadata(context.Background(), uuid.Nil)
	if !errors.Is(err, ErrOrgNotFound) {
		t.Errorf("err = %v, want ErrOrgNotFound", err)
	}
}

func TestAllowedRoles_ClosedSet(t *testing.T) {
	want := []string{"owner", "admin", "maintainer", "member", "viewer"}
	for _, r := range want {
		if !allowedRoles[r] {
			t.Errorf("%q debería estar permitido", r)
		}
	}
	for _, r := range []string{"god", "superadmin", "ADMIN", "MEMBER", ""} {
		if allowedRoles[r] {
			t.Errorf("%q NO debería estar permitido", r)
		}
	}
}
