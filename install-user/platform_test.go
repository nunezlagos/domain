package main

import (
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

func exeSuffix() string {
	if runtime.GOOS == "windows" {
		return ".exe"
	}
	return ""
}

func firstOrEmpty(s []string) string {
	if len(s) == 0 {
		return ""
	}
	return s[0]
}

func writeFakeBin(t *testing.T, path string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte("#!/bin/sh\nexit 0"), 0o755); err != nil {
		t.Fatal(err)
	}
}

// detección por PATH debe ganar sobre npm-global (orden estable)
func TestFindOpencode_InPathWinsOverNpmGlobal(t *testing.T) {
	binDir := t.TempDir()
	binPath := filepath.Join(binDir, "opencode"+exeSuffix())
	writeFakeBin(t, binPath)
	t.Setenv("PATH", binDir)

	home := t.TempDir()
	t.Setenv("HOME", home)
	npmBin := filepath.Join(home, ".npm-global", "bin", "opencode"+exeSuffix())
	writeFakeBin(t, npmBin)

	got, err := FindOpencode()
	if err != nil {
		t.Fatalf("FindOpencode: %v", err)
	}
	if got != binPath {
		t.Errorf("got %q, want %q (PATH debería tener prioridad)", got, binPath)
	}
}

func TestFindOpencode_NpmGlobalFallback(t *testing.T) {
	t.Setenv("PATH", t.TempDir())
	home := t.TempDir()
	t.Setenv("HOME", home)
	if runtime.GOOS == "windows" {
		t.Setenv("USERPROFILE", home)
		t.Setenv("APPDATA", filepath.Join(home, "AppData", "Roaming"))
		t.Setenv("LOCALAPPDATA", filepath.Join(home, "AppData", "Local"))
	}
	binPath := filepath.Join(home, ".npm-global", "bin", "opencode"+exeSuffix())
	writeFakeBin(t, binPath)

	got, err := FindOpencode()
	if err != nil {
		t.Fatalf("FindOpencode: %v", err)
	}
	if got != binPath {
		t.Errorf("got %q, want %q", got, binPath)
	}
}

func TestFindOpencode_LocalBin(t *testing.T) {
	t.Setenv("PATH", t.TempDir())
	home := t.TempDir()
	t.Setenv("HOME", home)
	if runtime.GOOS == "windows" {
		t.Setenv("USERPROFILE", home)
		t.Setenv("APPDATA", filepath.Join(home, "AppData", "Roaming"))
		t.Setenv("LOCALAPPDATA", filepath.Join(home, "AppData", "Local"))
	}
	binPath := filepath.Join(home, ".local", "bin", "opencode"+exeSuffix())
	writeFakeBin(t, binPath)

	got, err := FindOpencode()
	if err != nil {
		t.Fatalf("FindOpencode: %v", err)
	}
	if got != binPath {
		t.Errorf("got %q, want %q", got, binPath)
	}
}

func TestFindOpencode_NotFoundReturnsClearError(t *testing.T) {
	t.Setenv("PATH", t.TempDir())
	t.Setenv("HOME", t.TempDir())
	if runtime.GOOS == "windows" {
		t.Setenv("USERPROFILE", t.TempDir())
		t.Setenv("APPDATA", filepath.Join(t.TempDir(), "AppData", "Roaming"))
		t.Setenv("LOCALAPPDATA", filepath.Join(t.TempDir(), "AppData", "Local"))
	}
	_, err := FindOpencode()
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if !strings.Contains(err.Error(), "opencode not found") {
		t.Errorf("err = %q, want contains 'opencode not found'", err.Error())
	}
}

// Distro detection: leer /etc/os-release (linux only)
func TestDetectDistro_Arch(t *testing.T) {
	if runtime.GOOS != "linux" {
		t.Skip("linux only")
	}
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "os-release"), []byte(`ID=arch`), 0o644); err != nil {
		t.Fatal(err)
	}
	got := detectDistroFromFile(filepath.Join(dir, "os-release"))
	if got != "arch" {
		t.Errorf("got %q, want 'arch'", got)
	}
}

func TestDetectDistro_Debian(t *testing.T) {
	if runtime.GOOS != "linux" {
		t.Skip("linux only")
	}
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "os-release"), []byte(`ID=debian`), 0o644); err != nil {
		t.Fatal(err)
	}
	got := detectDistroFromFile(filepath.Join(dir, "os-release"))
	if got != "debian" {
		t.Errorf("got %q, want 'debian'", got)
	}
}

func TestDetectDistro_Ubuntu(t *testing.T) {
	if runtime.GOOS != "linux" {
		t.Skip("linux only")
	}
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "os-release"), []byte(`ID=ubuntu`), 0o644); err != nil {
		t.Fatal(err)
	}
	got := detectDistroFromFile(filepath.Join(dir, "os-release"))
	if got != "ubuntu" {
		t.Errorf("got %q, want 'ubuntu'", got)
	}
}

func TestDetectDistro_PassthroughUnknownID(t *testing.T) {
	if runtime.GOOS != "linux" {
		t.Skip("linux only")
	}
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "os-release"), []byte(`ID=gentoo`), 0o644); err != nil {
		t.Fatal(err)
	}
	got := detectDistroFromFile(filepath.Join(dir, "os-release"))
	if got != "gentoo" {
		t.Errorf("got %q, want 'gentoo' (ID passthrough)", got)
	}
}

func TestDetectDistro_EmptyFile(t *testing.T) {
	if runtime.GOOS != "linux" {
		t.Skip("linux only")
	}
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "os-release"), []byte(""), 0o644); err != nil {
		t.Fatal(err)
	}
	got := detectDistroFromFile(filepath.Join(dir, "os-release"))
	if got != "" {
		t.Errorf("got %q, want empty", got)
	}
}

// Install command builder — siempre hay Fallback npm install -g
func TestInstallOpencodeCmd_Arch(t *testing.T) {
	cmd := InstallOpencodeCmd(Platform{OS: "linux", Distro: "arch"})
	if cmd.Primary[0] != "pacman" {
		t.Errorf("Primary[0] = %q, want 'pacman'", firstOrEmpty(cmd.Primary))
	}
	if cmd.Fallback[0] != "npm" {
		t.Errorf("Fallback[0] = %q, want 'npm'", firstOrEmpty(cmd.Fallback))
	}
}

func TestInstallOpencodeCmd_Debian(t *testing.T) {
	cmd := InstallOpencodeCmd(Platform{OS: "linux", Distro: "debian"})
	if len(cmd.Primary) != 0 {
		t.Errorf("Primary = %v, want empty (no asumimos apt)", cmd.Primary)
	}
	if cmd.Fallback[0] != "npm" {
		t.Errorf("Fallback[0] = %q, want 'npm'", firstOrEmpty(cmd.Fallback))
	}
}

func TestInstallOpencodeCmd_Ubuntu(t *testing.T) {
	cmd := InstallOpencodeCmd(Platform{OS: "linux", Distro: "ubuntu"})
	if len(cmd.Primary) != 0 {
		t.Errorf("Primary = %v, want empty (no asumimos apt)", cmd.Primary)
	}
}

func TestInstallOpencodeCmd_UnknownLinux(t *testing.T) {
	cmd := InstallOpencodeCmd(Platform{OS: "linux", Distro: ""})
	if len(cmd.Primary) != 0 {
		t.Errorf("Primary = %v, want empty (sin package manager nativo conocido)", cmd.Primary)
	}
	if cmd.Fallback[0] != "npm" {
		t.Errorf("Fallback[0] = %q, want 'npm'", firstOrEmpty(cmd.Fallback))
	}
}

func TestInstallOpencodeCmd_Darwin(t *testing.T) {
	cmd := InstallOpencodeCmd(Platform{OS: "darwin"})
	if cmd.Primary[0] != "brew" {
		t.Errorf("Primary[0] = %q, want 'brew'", firstOrEmpty(cmd.Primary))
	}
}

func TestInstallOpencodeCmd_Windows(t *testing.T) {
	cmd := InstallOpencodeCmd(Platform{OS: "windows"})
	if cmd.Primary[0] != "winget" {
		t.Errorf("Primary[0] = %q, want 'winget'", firstOrEmpty(cmd.Primary))
	}
}

func TestPlatform_OSDetection(t *testing.T) {
	p := Platform{OS: runtime.GOOS}
	if p.OS != runtime.GOOS {
		t.Errorf("got %q, want %q", p.OS, runtime.GOOS)
	}
}
