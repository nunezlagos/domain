// Package config — HU-01.2 config-system (skeleton).
//
// Carga config desde env vars con prefijo DOMAIN_*. Validación + defaults.
// Implementación completa viene en Fase 1.
package config

import (
	"fmt"
	"os"
	"strconv"
)

// Config concentra toda la configuración runtime de Domain.
// TODO Fase 1: completar con todas las vars de .env.example.
type Config struct {
	Env            string
	HTTPBind       string
	HTTPPort       int
	DatabaseURL    string
	LogLevel       string
	LogFormat      string
	SeedOnBoot     bool
}

// Load lee config desde env vars y aplica defaults.
// Validación strict viene en Fase 1.
func Load() (*Config, error) {
	c := &Config{
		Env:         getEnv("DOMAIN_ENV", "dev"),
		HTTPBind:    getEnv("DOMAIN_HTTP_BIND", "127.0.0.1"),
		HTTPPort:    getEnvInt("DOMAIN_HTTP_PORT", 8000),
		DatabaseURL: getEnv("DOMAIN_DATABASE_URL", ""),
		LogLevel:    getEnv("DOMAIN_LOG_LEVEL", "info"),
		LogFormat:   getEnv("DOMAIN_LOG_FORMAT", "text"),
		SeedOnBoot:  getEnvBool("DOMAIN_SEED_ON_BOOT", true),
	}
	if c.DatabaseURL == "" {
		return nil, fmt.Errorf("DOMAIN_DATABASE_URL is required")
	}
	return c, nil
}

func getEnv(k, def string) string {
	if v := os.Getenv(k); v != "" {
		return v
	}
	return def
}

func getEnvInt(k string, def int) int {
	if v := os.Getenv(k); v != "" {
		if n, err := strconv.Atoi(v); err == nil {
			return n
		}
	}
	return def
}

func getEnvBool(k string, def bool) bool {
	if v := os.Getenv(k); v != "" {
		if b, err := strconv.ParseBool(v); err == nil {
			return b
		}
	}
	return def
}
