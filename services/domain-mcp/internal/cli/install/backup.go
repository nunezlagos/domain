// Backup + Restore para domain install/update (issue-01.10).
//
// Filosofía: backup SIEMPRE antes de cualquier mutación. Restore es
// one-shot para recovery puntual. Los backups son timestamped (RFC3339)
// y NUNCA se sobrescriben entre updates.
//
// Marcadores:
//   - credentials.json: archivo completo, validar con ping a /auth/first-run
//   - .env: archivo completo, formato key=value
//   - opencode.json, AGENTS.md, .md stubs: archivo completo, restauración trivial
//   - domain-managed marker: <!-- domain-managed --> en archivos que
//     el binario modificó (para que restore pueda identificarlos)
package install

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"
)

// DomainManagedMarker es el comentario HTML que el binario inyecta en
// archivos que modifica (AGENTS.md, .md stubs). `domain restore` lo
// busca para identificar qué restaurar.
const DomainManagedMarker = "<!-- domain-managed -->"

// ErrNoBackup retornado cuando se pide restaurar un path que no es
// un backup reconocido.
var ErrNoBackup = errors.New("not a domain backup (missing timestamp or marker)")

// BackupResult resume una operacion de backup.
type BackupResult struct {
	Path         string // path del archivo original
	Backup       string // path del .bak creado (o del .bak previo si Deduplicated)
	Bytes        int64  // tamaño del archivo
	Deduplicated bool   // true si se skipeó la creación porque el último .bak tiene el mismo hash
}

// backupFile crea un .bak.<RFC3339> del archivo en path. Retorna el
// path del backup. Si el archivo no existe, no falla (skip).
//
// Si keepLast > 0, después de crear el backup prunea los backups
// viejos (mantiene solo los últimos N). Default: 0 = mantener todos.
//
// DEDUP (issue-29.2): si el último .bak existente tiene el mismo
// SHA-256 que el archivo actual, NO se crea un nuevo .bak — se
// retorna el path del .bak previo con Deduplicated: true. Esto
// evita el spam de backups idénticos cuando el install corre
// múltiples veces sin cambios.
func backupFile(path string, keepLast int) (*BackupResult, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil // skip silencioso
		}
		return nil, fmt.Errorf("read %s: %w", path, err)
	}


	if matches, prev := lastBackupMatchesHash(path, data); matches {
		return &BackupResult{
			Path:         path,
			Backup:       prev,
			Bytes:        int64(len(data)),
			Deduplicated: true,
		}, nil
	}

	ts := time.Now().UTC().Format("20060102T150405Z")
	backupPath := path + ".bak." + ts
	if err := os.WriteFile(backupPath, data, 0o600); err != nil {
		return nil, fmt.Errorf("write backup %s: %w", backupPath, err)
	}

	if keepLast > 0 {
		if err := pruneBackups(path, keepLast); err != nil {

			fmt.Fprintf(os.Stderr, "warn: prune backups for %s: %v\n", path, err)
		}
	}
	return &BackupResult{
		Path:         path,
		Backup:       backupPath,
		Bytes:        int64(len(data)),
		Deduplicated: false,
	}, nil
}

// lastBackupMatchesHash compara el SHA-256 de data con el SHA-256
// del .bak más reciente de path. Retorna (true, lastPath) si
// matchean, (false, "") si difieren o no hay backup previo.
//
// ListBackups (línea 245) retorna los .bak.* ordenados
// lexicograficamente, que para timestamps RFC3339 es el mismo
// orden cronológico.
func lastBackupMatchesHash(path string, data []byte) (bool, string) {
	backups, err := ListBackups(path)
	if err != nil || len(backups) == 0 {
		return false, ""
	}
	last := backups[len(backups)-1]
	prevHash, err := FileChecksum(last)
	if err != nil {
		return false, ""
	}
	sum := sha256.Sum256(data)
	curHash := hex.EncodeToString(sum[:])
	return prevHash == curHash, last
}

// pruneBackups mantiene solo los últimos keepLast backups (por timestamp
// en el nombre). Borra los más viejos.
func pruneBackups(originalPath string, keepLast int) error {
	matches, err := filepath.Glob(originalPath + ".bak.*")
	if err != nil {
		return err
	}
	if len(matches) <= keepLast {
		return nil
	}




	toDelete := matches[:len(matches)-keepLast]
	for _, p := range toDelete {
		if err := os.Remove(p); err != nil {
			return err
		}
	}
	return nil
}

// BackupCredentials crea .bak de credentials.json (chmod 600). Si
// el archivo no existe, no falla.
func BackupCredentials(keepLast int) (*BackupResult, error) {
	return backupFile(CredentialsPath(), keepLast)
}

// BackupEnv crea .bak de .env en el cwd. Skip si no existe.
func BackupEnv(keepLast int) (*BackupResult, error) {
	return backupFile(".env", keepLast)
}

// BackupOpenCodeConfig crea .bak del opencode.json del user.
// Path tipico: ~/.config/opencode/opencode.json.
func BackupOpenCodeConfig(keepLast int) (*BackupResult, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return nil, err
	}
	return backupFile(filepath.Join(home, ".config", "opencode", "opencode.json"), keepLast)
}

// BackupFile es el helper genérico: backup de cualquier archivo.
// Usado por los AGENTS.md injection y .md stub generators.
func BackupFile(path string) (*BackupResult, error) {
	return backupFile(path, 0) // keepLast=0 = sin prune
}

// IsDomainManaged retorna true si el archivo tiene el marker de
// domain-managed (indica que domain lo modificó y puede restaurarlo).
func IsDomainManaged(path string) (bool, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return false, err
	}
	return strings.Contains(string(data), DomainManagedMarker), nil
}



// RestoreResult resume una operacion de restore.
type RestoreResult struct {
	Backup    string // path del backup leido
	Target    string // path destino (donde se escribio)
	Bytes     int64
	Validated bool   // true si la validacion posterior paso
	Notes     string // mensaje del validador (e.g., "key validated")
}

// Restore restaura un archivo desde un backup. El caller pasa el
// path del backup (e.g., ~/.config/domain/credentials.json.bak.<ts>)
// y el path destino. Si el destino es credentials.json, valida
// la key con un ping a /auth/first-run.
func Restore(backupPath, targetPath, baseURL string) (*RestoreResult, error) {

	if !isBackupPath(backupPath) {
		return nil, fmt.Errorf("%w: %s", ErrNoBackup, backupPath)
	}
	data, err := os.ReadFile(backupPath)
	if err != nil {
		return nil, fmt.Errorf("read backup: %w", err)
	}

	if err := os.MkdirAll(filepath.Dir(targetPath), 0o700); err != nil {
		return nil, err
	}
	if err := os.WriteFile(targetPath, data, 0o600); err != nil {
		return nil, fmt.Errorf("write %s: %w", targetPath, err)
	}
	res := &RestoreResult{
		Backup: backupPath,
		Target: targetPath,
		Bytes:  int64(len(data)),
	}

	if strings.HasSuffix(targetPath, "credentials.json") {
		creds, err := ParseCredentials(data)
		if err == nil {
			ok, err := validateAPIKey(baseURL, creds.APIKey)
			if err == nil {
				res.Validated = ok
				if ok {
					res.Notes = "API key validated against server"
				} else {
					res.Notes = "API key present but rejected by server (revoked?)"
				}
			} else {
				res.Notes = fmt.Sprintf("API key validation failed: %v", err)
			}
		}
	}
	return res, nil
}

// isBackupPath retorna true si el path tiene el formato *.bak.<RFC3339>.
func isBackupPath(path string) bool {
	_, file := filepath.Split(path)

	if !strings.Contains(file, ".bak.") {
		return false
	}
	idx := strings.LastIndex(file, ".bak.")
	ts := file[idx+len(".bak."):]

	return len(ts) == 16 && strings.HasSuffix(ts, "Z")
}

// ParsedCredentials es la representacion minima de credentials.json
// necesaria para validar la key. No es el struct completo del wizard
// porque el paquete install no debe depender de onboard (separación).
type ParsedCredentials struct {
	APIKey string `json:"api_key"`
}

// ParseCredentials parsea el JSON de credentials.json.
func ParseCredentials(data []byte) (*ParsedCredentials, error) {
	var c ParsedCredentials
	if err := json.Unmarshal(data, &c); err != nil {
		return nil, err
	}
	return &c, nil
}

// validateAPIKey hace GET /api/v1/auth/first-run con la key.
// Retorna true si responde 200, false en otro caso.
func validateAPIKey(baseURL, apiKey string) (bool, error) {
	if baseURL == "" {
		baseURL = "http://localhost:8000"
	}
	req, err := http.NewRequest("GET", baseURL+"/api/v1/auth/first-run", nil)
	if err != nil {
		return false, err
	}
	req.Header.Set("Authorization", "Bearer "+apiKey)
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return false, err
	}
	defer resp.Body.Close()
	_, _ = io.Copy(io.Discard, resp.Body)
	return resp.StatusCode == 200, nil
}

// ListBackups retorna los backups existentes para un path dado,
// ordenados del más viejo al más nuevo. Retorna slice no-nil (puede
// ser vacío) para que callers puedan usar len() sin nil-check.
func ListBackups(originalPath string) ([]string, error) {
	matches, err := filepath.Glob(originalPath + ".bak.*")
	if err != nil {
		return nil, err
	}
	if matches == nil {
		matches = []string{}
	}

	return matches, nil
}

// FileChecksum retorna el SHA256 hex de un archivo (para detectar
// cambios entre backups).
func FileChecksum(path string) (string, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return "", err
	}
	sum := sha256.Sum256(data)
	return hex.EncodeToString(sum[:]), nil
}
