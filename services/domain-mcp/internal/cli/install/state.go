// Package install — issue-01.10 deployment modes + install/update/restore.
//
// Filosofía: el comando `domain install` es el ÚNICO punto de entrada
// para tener Domain corriendo. Detecta el estado actual, pregunta
// el modo de deployment (local/cloud/hybrid), y orquesta el flujo
// completo con backups automáticos y sin destruir nada.
//
// Estados detectados:
//   - fresh: DB vacía + sin credenciales
//   - partial: DB con datos pero sin credenciales en disco
//   - installed: DB con users + credenciales presentes
//   - broken: credenciales presentes pero key inválida
//
// En cualquier estado, install es idempotente: skip lo que ya está.
package install

import (
	"context"
	"errors"
	"fmt"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"
)

// Mode selecciona el deployment mode.
type Mode string

const (
	ModeLocal  Mode = "local"  // docker compose up -d (Postgres+S3+SMTP)
	ModeCloud  Mode = "cloud"  // DSN provisto, sin docker
	ModeHybrid Mode = "hybrid" // per-service: local o cloud
)

// Estado actual de la instalación.
type InstallState struct {
	CredentialsExist bool        // ~/.config/domain/credentials.json existe
	EnvExist         bool        // .env existe
	DockerAvailable  bool        // `docker` en PATH
	DockerRunning    bool        // `docker compose ps` retorna containers up
	ServerReachable  bool        // /health responde
	FirstRun         bool        // DB vacía (no users)
	UserCount        int         // users en la DB
	BaseURL          string      // server URL configurado
}

// DetectState inspecciona el entorno y retorna el estado actual.
// Es read-only: no modifica nada.
func DetectState(baseURL string) (*InstallState, error) {
	st := &InstallState{BaseURL: baseURL}

	// Credenciales
	if _, err := os.Stat(CredentialsPath()); err == nil {
		st.CredentialsExist = true
	}

	// .env
	if _, err := os.Stat(".env"); err == nil {
		st.EnvExist = true
	}

	// Docker
	if path, err := exec.LookPath("docker"); err == nil && path != "" {
		st.DockerAvailable = true
		st.DockerRunning = checkDockerRunning()
	}

	// Server health (best-effort, no fatal)
	st.ServerReachable = pingHealth(baseURL)

	// First-run via /auth/first-run
	if st.ServerReachable {
		firstRun, count, err := getFirstRun(baseURL)
		if err == nil {
			st.FirstRun = firstRun
			st.UserCount = count
		}
	}

	return st, nil
}

// Summary retorna un resumen del estado para imprimir.
func (s *InstallState) Summary() string {
	creds := "no"
	if s.CredentialsExist {
		creds = "yes"
	}
	docker := "absent"
	if s.DockerAvailable {
		if s.DockerRunning {
			docker = "running"
		} else {
			docker = "stopped"
		}
	}
	server := "unreachable"
	if s.ServerReachable {
		server = "reachable"
	}
	firstRun := "no"
	if s.FirstRun {
		firstRun = "yes"
	}
	return fmt.Sprintf(
		"State: creds=%s, docker=%s, server=%s, first_run=%s, users=%d",
		creds, docker, server, firstRun, s.UserCount,
	)
}

// === Deployment mode helpers ===

// ServiceSelector selecciona local / cloud / none por servicio en hybrid mode.
type ServiceSelector string

const (
	SvcLocal ServiceSelector = "local"
	SvcCloud ServiceSelector = "cloud"
	SvcNone  ServiceSelector = "none"
)

// HybridConfig captura la decision del user en hybrid mode.
type HybridConfig struct {
	Postgres ServiceSelector
	S3       ServiceSelector
	SMTP     ServiceSelector
}

// Plan retorna la lista de servicios que deben correr local (docker).
// svcLocal/SvcCloud segun el selector del user. Retorna []string{}
// (no nil) para que callers puedan hacer len() sin nil-check.
func (h *HybridConfig) Plan() []string {
	local := []string{}
	if h.Postgres == SvcLocal {
		local = append(local, "postgres")
	}
	if h.S3 == SvcLocal {
		local = append(local, "minio")
	}
	if h.SMTP == SvcLocal {
		local = append(local, "mailpit")
	}
	return local
}

// LocalServices retorna los servicios que SI se arrancan en local mode.
func LocalServices() []string {
	return []string{"postgres", "minio", "mailpit"}
}

// StartDockerServices corre `docker compose up -d <services>...` con
// timeout de 90s (espera healthy). Retorna nil si OK, error si falla.
//
// En cloud mode este comando NO se llama (los servicios están en cloud).
func StartDockerServices(ctx context.Context, services []string) error {
	if len(services) == 0 {
		return nil
	}
	args := []string{"compose", "up", "-d"}
	args = append(args, services...)
	cmd := exec.CommandContext(ctx, "docker", args...)
	out, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("docker compose up failed: %w\n%s", err, string(out))
	}
	// Esperar healthy
	return WaitHealthy(ctx, services, 90*time.Second)
}

// WaitHealthy espera a que los servicios retornen status "healthy" via
// `docker compose ps`. Timeout default 90s.
func WaitHealthy(ctx context.Context, services []string, timeout time.Duration) error {
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		allHealthy := true
		for _, svc := range services {
			ok, _ := checkServiceHealthy(ctx, svc)
			if !ok {
				allHealthy = false
				break
			}
		}
		if allHealthy {
			return nil
		}
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(3 * time.Second):
			// reintentar
		}
	}
	return fmt.Errorf("services %v no llegaron a healthy en %v", services, timeout)
}

func checkServiceHealthy(ctx context.Context, service string) (bool, error) {
	cmd := exec.CommandContext(ctx, "docker", "compose", "ps", "--format", "{{.Service}}={{.Health}}", service)
	out, err := cmd.Output()
	if err != nil {
		return false, err
	}
	line := strings.TrimSpace(string(out))
	if line == "" {
		return false, nil
	}
	parts := strings.Split(line, "=")
	if len(parts) != 2 {
		return false, nil
	}
	health := strings.ToLower(parts[1])
	return strings.Contains(health, "healthy"), nil
}

func checkDockerRunning() bool {
	cmd := exec.Command("docker", "compose", "ps", "-q")
	out, err := cmd.Output()
	if err != nil {
		return false
	}
	return len(strings.TrimSpace(string(out))) > 0
}

// === DSN validation ===

// ErrInvalidDSN returned por ValidateDSN cuando la URL no es válida.
var ErrInvalidDSN = errors.New("invalid database URL")

// ErrPlaintextDSNInCloud returned cuando la URL es de un cloud provider
// conocido pero tiene sslmode=disable.
var ErrPlaintextDSNInCloud = errors.New("sslmode=disable not allowed for cloud providers (use require or verify-full)")

// cloudProviders lista substrings de hostnames de cloud providers conocidos.
// Si la URL contiene alguno, exigimos TLS.
var cloudProviders = []string{
	"rds.amazonaws.com",
	"neon.tech",
	"supabase.co",
	"heroku.com",
	"planetscale.com",
	"googleapis.com",
	"azure.com",
	"digitalocean.com",
}

// ValidateDSN parsea la URL y valida reglas de seguridad:
//   - debe ser postgres:// o postgresql://
//   - no debe tener password vacio
//   - si el host es de un cloud provider conocido, sslmode != disable
func ValidateDSN(dsn string) error {
	if dsn == "" {
		return errors.Join(ErrInvalidDSN, errors.New("empty"))
	}
	u, err := url.Parse(dsn)
	if err != nil {
		return errors.Join(ErrInvalidDSN, err)
	}
	if u.Scheme != "postgres" && u.Scheme != "postgresql" {
		return errors.Join(ErrInvalidDSN,
			fmt.Errorf("scheme must be postgres or postgresql, got %q", u.Scheme))
	}
	if u.User == nil {
		return errors.Join(ErrInvalidDSN, errors.New("missing user"))
	}
	pass, hasPass := u.User.Password()
	if !hasPass || pass == "" {
		return errors.Join(ErrInvalidDSN, errors.New("missing password"))
	}
	if u.Host == "" {
		return errors.Join(ErrInvalidDSN, errors.New("missing host"))
	}
	// Si el host es de un cloud provider conocido, sslmode != disable
	host := u.Hostname()
	for _, cp := range cloudProviders {
		if strings.Contains(host, cp) {
			sslMode := u.Query().Get("sslmode")
			if sslMode == "" || sslMode == "disable" || sslMode == "allow" || sslMode == "prefer" {
				// errors.Join preserva ambos: el sentinel y el mensaje custom.
				return errors.Join(ErrPlaintextDSNInCloud,
					fmt.Errorf("host %s is cloud provider, use sslmode=require or verify-full", host))
			}
		}
	}
	return nil
}

// === State detection helpers ===

func pingHealth(baseURL string) bool {
	// Best-effort: usa net/http con timeout corto.
	// Implementado en otro file (helpers.go) para mantener este chico.
	return pingHealthHTTP(baseURL)
}

func getFirstRun(baseURL string) (bool, int, error) {
	return getFirstRunHTTP(baseURL)
}

// CredentialsPath exportada para que install y wizard compartan path.
func CredentialsPath() string {
	home, err := os.UserHomeDir()
	if err != nil {
		return "~/.config/domain/credentials.json"
	}
	return filepath.Join(home, ".config", "domain", "credentials.json")
}
