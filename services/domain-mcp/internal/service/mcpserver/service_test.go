package mcpserver

import (
	"errors"
	"testing"
)

func TestNullStr(t *testing.T) {
	if nullStr("") != nil {
		t.Fatal("empty string must return nil")
	}
	if got := nullStr("x"); got != "x" {
		t.Fatalf("got %v, want 'x'", got)
	}
}

func TestCreate_RechazaTransportInvalido(t *testing.T) {

	s := &Service{}

	_, err := s.Create(nil, [16]byte{}, CreateInput{Transport: "websocket"})
	if !errors.Is(err, ErrInvalidTransport) {
		t.Fatalf("got %v, want ErrInvalidTransport", err)
	}
}

func TestCreate_StdioRequiereCommand(t *testing.T) {
	s := &Service{}
	_, err := s.Create(nil, [16]byte{}, CreateInput{Transport: TransportStdio})
	if !errors.Is(err, ErrCommandRequired) {
		t.Fatalf("got %v, want ErrCommandRequired", err)
	}
}

func TestCreate_HTTPRequiereURL(t *testing.T) {
	s := &Service{}
	_, err := s.Create(nil, [16]byte{}, CreateInput{Transport: TransportHTTP})
	if err == nil || !errors.Is(err, ErrInvalidTransport) {
		t.Fatalf("expected ErrInvalidTransport with url message, got %v", err)
	}
}
