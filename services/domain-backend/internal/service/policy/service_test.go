package policy

import (
	"context"
	"errors"
	"testing"
)

func TestValidKinds_Catalog(t *testing.T) {
	required := []string{
		KindConvention, KindSecurityRule, KindArchitecture, KindSDDWorkflow,
		KindObservability, KindMigrationRule, KindLinterConfig,
	}
	for _, k := range required {
		if !validKinds[k] {
			t.Fatalf("kind %s should be valid", k)
		}
	}
	if validKinds["random"] {
		t.Fatal("random kind should NOT be valid")
	}
}

func TestCreate_RechazaSlugInvalido(t *testing.T) {
	s := &Service{}
	_, err := s.Create(context.Background(), CreateInput{
		Slug: "Invalid Slug", Name: "x", Kind: KindConvention, BodyMD: "body",
	})
	if !errors.Is(err, ErrInvalidSlug) {
		t.Fatalf("got %v, want ErrInvalidSlug", err)
	}
}

func TestCreate_RechazaKindInvalido(t *testing.T) {
	s := &Service{}
	_, err := s.Create(context.Background(), CreateInput{
		Slug: "valid-slug", Name: "x", Kind: "random_kind", BodyMD: "body",
	})
	if !errors.Is(err, ErrInvalidKind) {
		t.Fatalf("got %v, want ErrInvalidKind", err)
	}
}

func TestErrUnknown_Sentinel(t *testing.T) {
	if !errors.Is(ErrUnknown, ErrUnknown) {
		t.Fatal("sentinel comparison failed")
	}
}
