package install

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"time"
)

// pingHealthHTTP hace GET /health/ready con timeout corto. Retorna
// true si responde 200.
func pingHealthHTTP(baseURL string) bool {
	if baseURL == "" {
		baseURL = "http://localhost:8000"
	}
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	req, err := http.NewRequestWithContext(ctx, "GET", baseURL+"/health/ready", nil)
	if err != nil {
		return false
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return false
	}
	defer resp.Body.Close()
	_, _ = io.Copy(io.Discard, resp.Body)
	return resp.StatusCode == 200
}

// getFirstRunHTTP hace GET /api/v1/auth/first-run. Retorna isFirstRun,
// userCount, error. Si el server no responde, retorna error (caller
// usa defaults).
func getFirstRunHTTP(baseURL string) (bool, int, error) {
	if baseURL == "" {
		baseURL = "http://localhost:8000"
	}
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	req, err := http.NewRequestWithContext(ctx, "GET", baseURL+"/api/v1/auth/first-run", nil)
	if err != nil {
		return false, 0, err
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return false, 0, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != 200 {
		return false, 0, nil
	}
	var out struct {
		IsFirstRun bool `json:"is_first_run"`
		UserCount  int  `json:"user_count"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
		return false, 0, err
	}
	return out.IsFirstRun, out.UserCount, nil
}
