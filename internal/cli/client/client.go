// Package clicli — cliente HTTP minimal para el CLI Domain.
//
// El CLI usa la misma API REST que cualquier cliente externo (consistencia).
// Auth via env DOMAIN_API_KEY y DOMAIN_BASE_URL (default http://localhost:8000).
package clicli

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"
	"time"

	"gopkg.in/yaml.v3"
)

type ConfigFile struct {
	APIKey  string `yaml:"api_key"`
	BaseURL string `yaml:"base_url"`
}

type Client struct {
	APIKey  string
	BaseURL string
	HTTP    *http.Client
}

// NewFromEnv toma DOMAIN_API_KEY + DOMAIN_BASE_URL del environment.
func NewFromEnv() (*Client, error) {
	key := os.Getenv("DOMAIN_API_KEY")
	if key == "" {
		return nil, errors.New("DOMAIN_API_KEY env var requerida")
	}
	base := os.Getenv("DOMAIN_BASE_URL")
	if base == "" {
		base = "http://localhost:8000"
	}
	return &Client{
		APIKey:  key,
		BaseURL: strings.TrimRight(base, "/"),
		HTTP:    &http.Client{Timeout: 30 * time.Second},
	}, nil
}

// NewFromFile lee configuración desde archivo YAML.
// Si un campo está vacío en el archivo, cae a env var.
func NewFromFile(path string) (*Client, error) {
	raw, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("leer config %s: %w", path, err)
	}
	var cfg ConfigFile
	if err := yaml.Unmarshal(raw, &cfg); err != nil {
		return nil, fmt.Errorf("parsear config %s: %w", path, err)
	}
	key := cfg.APIKey
	if key == "" {
		key = os.Getenv("DOMAIN_API_KEY")
	}
	if key == "" {
		return nil, errors.New("api_key requerida en config o DOMAIN_API_KEY env")
	}
	base := cfg.BaseURL
	if base == "" {
		base = os.Getenv("DOMAIN_BASE_URL")
	}
	if base == "" {
		base = "http://localhost:8000"
	}
	return &Client{
		APIKey:  key,
		BaseURL: strings.TrimRight(base, "/"),
		HTTP:    &http.Client{Timeout: 30 * time.Second},
	}, nil
}

// Do envía una request y devuelve body.data parsed.
// Si status >= 400 retorna error con code/message del body.error.
func (c *Client) Do(method, path string, body any, query map[string]string) (any, error) {
	url := c.BaseURL + "/api/v1" + path
	if len(query) > 0 {
		var qs []string
		for k, v := range query {
			if v != "" {
				qs = append(qs, k+"="+v)
			}
		}
		if len(qs) > 0 {
			url += "?" + strings.Join(qs, "&")
		}
	}
	var bodyReader io.Reader
	if body != nil {
		raw, err := json.Marshal(body)
		if err != nil {
			return nil, fmt.Errorf("marshal body: %w", err)
		}
		bodyReader = bytes.NewReader(raw)
	}
	req, err := http.NewRequest(method, url, bodyReader)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Authorization", "Bearer "+c.APIKey)
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("User-Agent", "domain-cli/0.1.0")

	resp, err := c.HTTP.Do(req)
	if err != nil {
		return nil, fmt.Errorf("request: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode == http.StatusNoContent {
		return nil, nil
	}
	raw, _ := io.ReadAll(resp.Body)
	var parsed map[string]any
	if len(raw) > 0 {
		_ = json.Unmarshal(raw, &parsed)
	}
	if resp.StatusCode >= 400 {
		errObj, _ := parsed["error"].(map[string]any)
		code, _ := errObj["code"].(string)
		msg, _ := errObj["message"].(string)
		return nil, fmt.Errorf("HTTP %d %s: %s", resp.StatusCode, code, msg)
	}
	return parsed["data"], nil
}
