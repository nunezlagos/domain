// `domain service` — systemd user service para que el server quede
// corriendo siempre (plug-and-play, issue-01.10).
//
//	domain service install    crea + habilita + arranca el service
//	domain service status     estado del service
//	domain service uninstall  detiene + deshabilita + borra el unit
//
// El unit NO necesita EnvironmentFile: el binario carga su config en
// cascada (~/.config/domain/env) vía loadEnvCascade. Linux only;
// en macOS se difiere launchd.

package main

import (
	"fmt"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"time"
)

const serviceName = "domain.service"

// serviceUnitContent genera el unit file apuntando al binario dado.
func serviceUnitContent(binPath string) string {
	return `[Unit]
Description=Domain server (personal & project memory platform)
After=network.target

[Service]
ExecStart=` + binPath + ` server
Restart=always
RestartSec=5
StartLimitIntervalSec=0

[Install]
WantedBy=default.target
`
}

// serviceUnitPath retorna ~/.config/systemd/user/domain.service.
func serviceUnitPath() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(home, ".config", "systemd", "user", serviceName), nil
}

// systemdUserAvailable chequea que systemctl --user sea operativo
// (Linux con sesión dbus de usuario; falso en containers/macOS).
func systemdUserAvailable() bool {
	if runtime.GOOS != "linux" {
		return false
	}
	if _, err := exec.LookPath("systemctl"); err != nil {
		return false
	}
	// is-system-running falla en entornos sin user manager.
	cmd := exec.Command("systemctl", "--user", "is-system-running")
	out, _ := cmd.CombinedOutput()
	state := strings.TrimSpace(string(out))
	return state != "" && state != "offline" && state != "unknown"
}

// resolveDomainBinary encuentra el binario estable para ExecStart:
// PATH primero (instalación normal en ~/go/bin), después el ejecutable
// actual como fallback.
func resolveDomainBinary() (string, error) {
	if p, err := exec.LookPath("domain"); err == nil {
		return filepath.Abs(p)
	}
	return os.Executable()
}

// installUserService escribe el unit, recarga systemd y habilita +
// arranca el service. Espera /health hasta healthTimeout.
func installUserService(baseURL string) error {
	bin, err := resolveDomainBinary()
	if err != nil {
		return fmt.Errorf("resolve binary: %w", err)
	}
	unitPath, err := serviceUnitPath()
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(unitPath), 0o755); err != nil {
		return err
	}
	if err := os.WriteFile(unitPath, []byte(serviceUnitContent(bin)), 0o644); err != nil {
		return fmt.Errorf("write unit: %w", err)
	}
	for _, args := range [][]string{
		{"--user", "daemon-reload"},
		{"--user", "enable", "--now", serviceName},
		{"--user", "restart", serviceName}, // pick-up de binario/config nuevos en re-installs
	} {
		cmd := exec.Command("systemctl", args...)
		if out, err := cmd.CombinedOutput(); err != nil {
			return fmt.Errorf("systemctl %s: %v (%s)", strings.Join(args, " "), err, strings.TrimSpace(string(out)))
		}
	}
	return waitServerHealth(baseURL, 20*time.Second)
}

// waitServerHealth pollea GET /health hasta timeout.
func waitServerHealth(baseURL string, timeout time.Duration) error {
	if baseURL == "" {
		baseURL = "http://localhost:8000"
	}
	client := &http.Client{Timeout: 2 * time.Second}
	deadline := time.Now().Add(timeout)
	var lastErr error
	for time.Now().Before(deadline) {
		resp, err := client.Get(baseURL + "/health")
		if err == nil {
			resp.Body.Close()
			if resp.StatusCode == http.StatusOK {
				return nil
			}
			lastErr = fmt.Errorf("health status %d", resp.StatusCode)
		} else {
			lastErr = err
		}
		time.Sleep(500 * time.Millisecond)
	}
	return fmt.Errorf("server no respondió /health en %s: %v (revisá: journalctl --user -u %s)", timeout, lastErr, serviceName)
}

// runService es el entrypoint de `domain service`.
func runService(args []string) int {
	action := "status"
	if len(args) > 0 {
		action = args[0]
	}
	switch action {
	case "install":
		if !systemdUserAvailable() {
			fmt.Fprintln(os.Stderr, "systemd user manager no disponible (¿macOS/container?).")
			fmt.Fprintln(os.Stderr, "Corré el server manualmente: domain server")
			return 1
		}
		baseURL := envOr("DOMAIN_BASE_URL", "http://localhost:8000")
		if err := installUserService(baseURL); err != nil {
			fmt.Fprintf(os.Stderr, "service install: %v\n", err)
			return 1
		}
		fmt.Println("✓ domain.service habilitado y corriendo (arranca al login)")
		fmt.Println("  logs: journalctl --user -u domain -f")
		return 0
	case "status":
		out, _ := exec.Command("systemctl", "--user", "status", serviceName, "--no-pager", "-n", "5").CombinedOutput()
		fmt.Print(string(out))
		return 0
	case "uninstall":
		_ = exec.Command("systemctl", "--user", "disable", "--now", serviceName).Run()
		if unitPath, err := serviceUnitPath(); err == nil {
			_ = os.Remove(unitPath)
		}
		_ = exec.Command("systemctl", "--user", "daemon-reload").Run()
		fmt.Println("✓ domain.service detenido y eliminado")
		return 0
	case "--help", "-h", "help":
		fmt.Println("Usage: domain service [install|status|uninstall]")
		return 0
	default:
		fmt.Fprintf(os.Stderr, "acción desconocida: %s (install|status|uninstall)\n", action)
		return 2
	}
}
