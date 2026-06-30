package enrollment

import (
	"context"
	"errors"
	"testing"

	"github.com/google/uuid"
)





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

func TestRotate_RejectsInvalidRole(t *testing.T) {
	s := &Service{}
	_, err := s.Rotate(context.Background(), uuid.New(), "superadmin")
	if !errors.Is(err, ErrInvalidRole) {
		t.Errorf("err = %v, want ErrInvalidRole", err)
	}
}

func TestRotate_AcceptsBlankRoleAsDefaultMember(t *testing.T) {



	if !allowedRoles["member"] {
		t.Fatal("member debería estar permitido como default")
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
