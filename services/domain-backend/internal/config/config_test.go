package config

import (
	"testing"
)

func TestLoad_RequiresDatabaseURL(t *testing.T) {
	t.Setenv("DOMAIN_DATABASE_URL", "")
	if _, err := Load(); err == nil {
		t.Fatal("expected error when DATABASE_URL missing")
	}
}

func TestLoad_Defaults(t *testing.T) {
	t.Setenv("DOMAIN_DATABASE_URL", "postgres://x:y@h/d?sslmode=disable")
	c, err := Load()
	if err != nil {
		t.Fatalf("unexpected: %v", err)
	}
	if c.Env != "dev" {
		t.Errorf("Env default: got %q want dev", c.Env)
	}
	if c.HTTPBind != "127.0.0.1" {
		t.Errorf("HTTPBind default: got %q", c.HTTPBind)
	}
	if c.HTTPPort != 8000 {
		t.Errorf("HTTPPort default: got %d", c.HTTPPort)
	}
	if c.LogLevel != "info" {
		t.Errorf("LogLevel default: got %q", c.LogLevel)
	}
}

func TestValidate_PortRange(t *testing.T) {
	c := &Config{
		Env:         "dev",
		HTTPPort:    99999,
		DatabaseURL: "x",
		LogLevel:    "info",
		LogFormat:   "text",
		SMTPAuth:    "none",
	}
	if err := c.Validate(); err == nil {
		t.Fatal("expected port out of range error")
	}
}

func TestValidate_EnvInvalid(t *testing.T) {
	c := &Config{
		Env:         "wat",
		HTTPPort:    8000,
		DatabaseURL: "x",
		LogLevel:    "info",
		LogFormat:   "text",
		SMTPAuth:    "none",
	}
	if err := c.Validate(); err == nil {
		t.Fatal("expected env invalid error")
	}
}

func TestIsProduction(t *testing.T) {
	c := &Config{Env: "prod"}
	if !c.IsProduction() {
		t.Error("IsProduction should be true for prod")
	}
	c.Env = "dev"
	if c.IsProduction() {
		t.Error("IsProduction should be false for dev")
	}
}
