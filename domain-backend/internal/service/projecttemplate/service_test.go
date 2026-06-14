package projecttemplate

import (
	"context"
	"errors"
	"testing"

	"github.com/google/uuid"
)

func TestReSlug_Valid(t *testing.T) {
	cases := map[string]bool{
		"go-backend":     true,
		"a":              true,
		"react-app-2026": true,
		"":               false,
		"-leading-dash":  false,
		"trailing-":      false,
		"UPPER":          false,
		"with space":     false,
		"with_underscore": false,
	}
	for in, want := range cases {
		got := reSlug.MatchString(in)
		if got != want {
			t.Fatalf("%q: got %v, want %v", in, got, want)
		}
	}
}

func TestCreate_InvalidSlug(t *testing.T) {
	s := &Service{}
	_, err := s.Create(context.Background(), uuid.New(), CreateInput{Slug: "Invalid Slug", Name: "x"})
	if !errors.Is(err, ErrInvalidSlug) {
		t.Fatalf("got %v, want ErrInvalidSlug", err)
	}
}

func TestErrUnknown_Sentinel(t *testing.T) {
	if !errors.Is(ErrUnknown, ErrUnknown) {
		t.Fatal("sentinel comparison failed")
	}
}
