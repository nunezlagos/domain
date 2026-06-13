package httpserver

import (
	"fmt"
	"net/http"
	"time"
)

// ProbeHealth hace un GET a http://127.0.0.1:<port>/health con
// timeout 1s. Retorna nil si responde 200, error en otro caso.
//
// Usado por el watchdog post-bind (issue-29.3) para detectar el
// caso "proceso vivo pero listener no responde" (bug detectado
// en sesión 2026-06-12: domain.service "active" pero /health 000).
func ProbeHealth(port int) error {
	client := &http.Client{Timeout: 1 * time.Second}
	url := fmt.Sprintf("http://127.0.0.1:%d/health", port)
	resp, err := client.Get(url)
	if err != nil {
		return fmt.Errorf("health probe failed for %s: %w", url, err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != 200 {
		return fmt.Errorf("health probe %s returned status %d (expected 200)", url, resp.StatusCode)
	}
	return nil
}
