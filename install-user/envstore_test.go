package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestSaveEnv_PermissionsAre0600(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "domain", "install.env")
	if err := saveEnv(path, EnvData{VPSURL: "http://x", Email: "u@x"}); err != nil {
		t.Fatal(err)
	}
	st, err := os.Stat(path)
	if err != nil {
		t.Fatal(err)
	}
	mode := st.Mode().Perm()
	if mode != 0o600 {
		t.Errorf("mode = %o, want 0600", mode)
	}
}

func TestSaveEnv_CreatesParentDir(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "deep", "nested", "install.env")
	if err := saveEnv(path, EnvData{VPSURL: "http://x", Email: "u@x"}); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(path); err != nil {
		t.Errorf("file no creado: %v", err)
	}
}

func TestLoadEnv_NonExistentReturnsEmpty(t *testing.T) {
	data, err := loadEnv(filepath.Join(t.TempDir(), "nope.env"))
	if err != nil {
		t.Fatalf("loadEnv: %v", err)
	}
	if data.VPSURL != "" || data.Email != "" {
		t.Errorf("got %+v, want empty", data)
	}
}

func TestLoadEnv_RoundTrip(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "install.env")
	orig := EnvData{VPSURL: "http://vps", Email: "u@v.cl"}
	if err := saveEnv(path, orig); err != nil {
		t.Fatal(err)
	}
	got, err := loadEnv(path)
	if err != nil {
		t.Fatal(err)
	}
	if got.VPSURL != orig.VPSURL || got.Email != orig.Email {
		t.Errorf("got %+v, want %+v", got, orig)
	}
}

func TestRemoveEnv_OnlyRemovesFile_NotDirectory(t *testing.T) {
	dir := t.TempDir()
	domainDir := filepath.Join(dir, "domain")
	if err := os.MkdirAll(domainDir, 0o755); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(domainDir, "install.env")
	if err := os.WriteFile(path, []byte("DOMAIN_VPS_URL=http://x"), 0o600); err != nil {
		t.Fatal(err)
	}

	removeEnv(path)

	if _, err := os.Stat(path); !os.IsNotExist(err) {
		t.Errorf("install.env no fue removido")
	}
	if _, err := os.Stat(domainDir); err != nil {
		t.Errorf("directorio padre fue removido: %v", err)
	}
}

// DetectURLMismatch: si la URL nueva difiere de la guardada, retorna warning + requiere confirmación
func TestDetectURLMismatch_Different(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("HOME", dir)
	path := filepath.Join(dir, ".config", "domain", "install.env")
	if err := saveEnv(path, EnvData{VPSURL: "http://old", Email: "u@v.cl"}); err != nil {
		t.Fatal(err)
	}
	warning, err := detectURLMismatch(path, "http://new")
	if err != nil {
		t.Fatalf("detectURLMismatch: %v", err)
	}
	if warning == "" {
		t.Error("warning vacío, debería indicar cambio de URL")
	}
	if !strings.Contains(warning, "old") || !strings.Contains(warning, "new") {
		t.Errorf("warning = %q, want contiene 'old' y 'new'", warning)
	}
}

func TestDetectURLMismatch_Same(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "install.env")
	if err := saveEnv(path, EnvData{VPSURL: "http://same", Email: "u@v.cl"}); err != nil {
		t.Fatal(err)
	}
	warning, err := detectURLMismatch(path, "http://same")
	if err != nil {
		t.Fatalf("detectURLMismatch: %v", err)
	}
	if warning != "" {
		t.Errorf("warning = %q, want empty (URLs iguales)", warning)
	}
}

func TestDetectURLMismatch_NoExistingFile(t *testing.T) {
	path := filepath.Join(t.TempDir(), "nope.env")
	warning, err := detectURLMismatch(path, "http://new")
	if err != nil {
		t.Fatalf("detectURLMismatch: %v", err)
	}
	if warning != "" {
		t.Errorf("warning = %q, want empty (archivo no existe)", warning)
	}
}
