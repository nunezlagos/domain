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
	"net"
	"net/http"
	neturl "net/url"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"syscall"
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
	content := serviceUnitContent(bin)
	port := portFromBaseURL(baseURL)

	// Idempotencia sin downtime: unit sin cambios + health OK + el
	// listener es realmente el proceso del service + el proceso corre el
	// binario ACTUAL → no-op. Si el binario en disco fue recompilado
	// (update), /proc/PID/exe queda "(deleted)" → reiniciamos para tomarlo.
	if existing, readErr := os.ReadFile(unitPath); readErr == nil && string(existing) == content {
		if waitServerHealth(baseURL, 2*time.Second) == nil && listenerIsService(port) && !serviceRunsStaleBinary() {
			return nil
		}
	}

	// Huérfanos: si el puerto lo ocupa un proceso `domain` que NO es el
	// del service (e.g. un `domain server` manual de una sesión vieja),
	// lo terminamos acá — el user no tiene que matar nada a mano. Si lo
	// ocupa OTRA app, error claro (no matamos procesos ajenos).
	if err := reapOrphanOnPort(port); err != nil {
		return err
	}

	if err := os.MkdirAll(filepath.Dir(unitPath), 0o755); err != nil {
		return err
	}
	if err := os.WriteFile(unitPath, []byte(content), 0o644); err != nil {
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
	// 60s: el restart hace graceful shutdown del proceso viejo (drain
	// ~5-25s) + boot completo del nuevo. Con 20s reportaba warning por
	// una carrera aunque el server terminara de levantar bien.
	if err := waitServerHealth(baseURL, 60*time.Second); err != nil {
		// Auto-diagnóstico: adjuntar el journal para que el warning diga
		// la CAUSA, no solo "no respondió".
		if tail := journalTail(5); tail != "" {
			return fmt.Errorf("%v | journal: %s", err, tail)
		}
		return err
	}
	return nil
}

// journalTail retorna las últimas n líneas del journal del service en
// una sola línea ("" si journalctl no está disponible).
func journalTail(n int) string {
	out, err := exec.Command("journalctl", "--user", "-u", serviceName,
		"-n", strconv.Itoa(n), "--no-pager", "-o", "cat").Output()
	if err != nil {
		return ""
	}
	lines := strings.Split(strings.TrimSpace(string(out)), "\n")
	return strings.Join(lines, " ⏎ ")
}

// portFromBaseURL extrae el puerto de la base URL ("8000" si no se puede).
func portFromBaseURL(baseURL string) int {
	if u, err := neturl.Parse(baseURL); err == nil && u.Port() != "" {
		if p, err := strconv.Atoi(u.Port()); err == nil {
			return p
		}
	}
	return 8000
}

// serviceMainPID retorna el MainPID del service (0 si no corre).
func serviceMainPID() int {
	out, err := exec.Command("systemctl", "--user", "show", serviceName, "-p", "MainPID", "--value").Output()
	if err != nil {
		return 0
	}
	pid, _ := strconv.Atoi(strings.TrimSpace(string(out)))
	return pid
}

// listenerIsService true si quien escucha el puerto es el MainPID del
// service (o un hijo directo suyo).
func listenerIsService(port int) bool {
	lpid := portListenerPID(port)
	if lpid == 0 {
		return false
	}
	mpid := serviceMainPID()
	return mpid != 0 && (lpid == mpid || procPPID(lpid) == mpid)
}

// reapOrphanOnPort libera el puerto si lo ocupa un proceso `domain`
// ajeno al service. SIGTERM + corta espera + SIGKILL (los huérfanos
// son desechables; su graceful shutdown puede demorar ~25s y no vale
// la pena esperarlo). Si lo ocupa otra app → error claro, NO se mata.
func reapOrphanOnPort(port int) error {
	pid := portListenerPID(port)
	if pid == 0 {
		return nil // puerto libre
	}
	if mpid := serviceMainPID(); mpid != 0 && (pid == mpid || procPPID(pid) == mpid) {
		return nil // es el propio service: systemctl restart lo maneja
	}
	comm := procComm(pid)
	if comm != "domain" {
		// Proceso ajeno (docker, otra app): NUNCA lo matamos. Informamos
		// y proponemos el siguiente puerto libre.
		return fmt.Errorf("puerto %d ocupado por otro proceso (%s, pid %d) — no lo toco; "+
			"re-corré el install con --base-url http://localhost:%d (puerto libre sugerido)",
			port, comm, pid, nextFreePort(port+1))
	}
	fmt.Fprintf(os.Stderr, "  (terminando server domain huérfano pid=%d en :%d)\n", pid, port)
	_ = syscall.Kill(pid, syscall.SIGTERM)
	deadline := time.Now().Add(3 * time.Second)
	for time.Now().Before(deadline) {
		if !pidAlive(pid) {
			return nil
		}
		time.Sleep(200 * time.Millisecond)
	}
	_ = syscall.Kill(pid, syscall.SIGKILL)
	// Esperar a que el kernel libere el puerto.
	deadline = time.Now().Add(3 * time.Second)
	for time.Now().Before(deadline) {
		if portListenerPID(port) == 0 {
			return nil
		}
		time.Sleep(200 * time.Millisecond)
	}
	return nil
}

// pidAlive chequea existencia con signal 0.
func pidAlive(pid int) bool {
	return syscall.Kill(pid, 0) == nil
}

// serviceRunsStaleBinary detecta si el proceso del service corre un
// binario que fue reemplazado en disco (go build sobre el mismo path):
// el symlink /proc/PID/exe queda apuntando a "<path> (deleted)".
func serviceRunsStaleBinary() bool {
	pid := serviceMainPID()
	if pid == 0 {
		return false
	}
	exe, err := os.Readlink(fmt.Sprintf("/proc/%d/exe", pid))
	if err != nil {
		return false
	}
	return strings.HasSuffix(exe, " (deleted)")
}

// nextFreePort busca el primer puerto TCP libre desde start.
func nextFreePort(start int) int {
	for p := start; p < start+50; p++ {
		ln, err := net.Listen("tcp", fmt.Sprintf("127.0.0.1:%d", p))
		if err == nil {
			_ = ln.Close()
			return p
		}
	}
	return start
}

// procComm lee /proc/pid/comm ("" si no accesible).
func procComm(pid int) string {
	data, err := os.ReadFile(fmt.Sprintf("/proc/%d/comm", pid))
	if err != nil {
		return ""
	}
	return strings.TrimSpace(string(data))
}

// procPPID lee el PPid de /proc/pid/status (0 si no accesible).
func procPPID(pid int) int {
	data, err := os.ReadFile(fmt.Sprintf("/proc/%d/status", pid))
	if err != nil {
		return 0
	}
	for _, line := range strings.Split(string(data), "\n") {
		if strings.HasPrefix(line, "PPid:") {
			p, _ := strconv.Atoi(strings.TrimSpace(strings.TrimPrefix(line, "PPid:")))
			return p
		}
	}
	return 0
}

// portListenerPID retorna el PID (del user actual) que escucha en el
// puerto TCP dado, o 0. Implementado vía /proc/net/tcp{,6} → inode del
// socket → scan de /proc/*/fd. Solo ve procesos del mismo user, que es
// exactamente el caso de los huérfanos `domain`.
func portListenerPID(port int) int {
	inodes := map[string]bool{}
	for _, f := range []string{"/proc/net/tcp", "/proc/net/tcp6"} {
		data, err := os.ReadFile(f)
		if err != nil {
			continue
		}
		for _, ino := range listenInodesForPort(string(data), port) {
			inodes[ino] = true
		}
	}
	if len(inodes) == 0 {
		return 0
	}
	procs, err := os.ReadDir("/proc")
	if err != nil {
		return 0
	}
	for _, p := range procs {
		pid, err := strconv.Atoi(p.Name())
		if err != nil {
			continue
		}
		fdDir := fmt.Sprintf("/proc/%d/fd", pid)
		fds, err := os.ReadDir(fdDir)
		if err != nil {
			continue // proceso ajeno o ya muerto
		}
		for _, fd := range fds {
			link, err := os.Readlink(filepath.Join(fdDir, fd.Name()))
			if err != nil {
				continue
			}
			if strings.HasPrefix(link, "socket:[") {
				ino := strings.TrimSuffix(strings.TrimPrefix(link, "socket:["), "]")
				if inodes[ino] {
					return pid
				}
			}
		}
	}
	return 0
}

// listenInodesForPort parsea el contenido de /proc/net/tcp{,6} y retorna
// los inodes de sockets en LISTEN (st=0A) cuyo local port coincide.
func listenInodesForPort(data string, port int) []string {
	var out []string
	wantHex := fmt.Sprintf("%04X", port)
	lines := strings.Split(data, "\n")
	for i, line := range lines {
		if i == 0 {
			continue // header
		}
		fields := strings.Fields(line)
		// sl local_address rem_address st ... inode en field 9
		if len(fields) < 10 || fields[3] != "0A" {
			continue
		}
		local := fields[1]
		colon := strings.LastIndex(local, ":")
		if colon < 0 || local[colon+1:] != wantHex {
			continue
		}
		out = append(out, fields[9])
	}
	return out
}

// waitServerHealth pollea GET /health hasta timeout. Emite un heartbeat
// a stderr cada ~2s para que la TUI muestre qué está esperando (sin
// esto, esperas largas se sienten como un cuelgue).
func waitServerHealth(baseURL string, timeout time.Duration) error {
	if baseURL == "" {
		baseURL = "http://localhost:8000"
	}
	client := &http.Client{Timeout: 2 * time.Second}
	start := time.Now()
	deadline := start.Add(timeout)
	var lastErr error
	lastBeat := time.Time{}
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
		if timeout > 5*time.Second && time.Since(lastBeat) >= 2*time.Second {
			fmt.Fprintf(os.Stderr, "  esperando /health del server (%ds)\n",
				int(time.Since(start).Seconds()))
			lastBeat = time.Now()
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
