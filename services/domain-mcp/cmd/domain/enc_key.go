package main

import (
	"context"
	"crypto/rand"
	"encoding/base64"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
)

// encKeyEnvName es la variable de entorno que comparten Go (config.go:154
// → cfg.FieldEncKey) y Django (settings/base.py:98 → FIELD_ENC_KEY). Es la
// passphrase de pgp_sym_encrypt sobre auth_api_keys.key_ciphertext.
const encKeyEnvName = "DOMAIN_FIELD_ENC_KEY"

// encKeyPlaceholder es el valor que trae .env.example (línea 40). Se trata
// como "no seteada" para gatillar generación en un install fresh.
const encKeyPlaceholder = "CHANGE_ME"

// resolveExistingEncKey busca una enc-key ya disponible, en orden de
// precedencia, SIN generar nada:
//  1. env del proceso (os.Getenv) — gana siempre (incluye lo cargado por
//     loadEnvCascade desde .env y ~/.config/domain/env).
//  2. ~/.config/domain/env (env global preservado entre reinstalls).
//  3. .env del cwd (lo que lee Django/compose).
//
// Retorna "" si no hay ninguna válida. El placeholder CHANGE_ME se ignora
// (cuenta como ausente) para que un fresh install genere una real.
func resolveExistingEncKey() string {
	if v := sanitizeEncKey(os.Getenv(encKeyEnvName)); v != "" {
		return v
	}
	if home, err := os.UserHomeDir(); err == nil {
		if v := readEnvKeyFromFile(filepath.Join(home, ".config", "domain", "env"), encKeyEnvName); v != "" {
			return v
		}
	}
	if v := readEnvKeyFromFile(".env", encKeyEnvName); v != "" {
		return v
	}
	return ""
}

// sanitizeEncKey normaliza un valor de enc-key: recorta espacios y comillas,
// y descarta el placeholder CHANGE_ME (lo trata como vacío).
func sanitizeEncKey(v string) string {
	v = strings.TrimSpace(v)
	v = strings.Trim(v, `"'`)
	if v == "" || v == encKeyPlaceholder {
		return ""
	}
	return v
}

// readEnvKeyFromFile lee KEY=VALUE de un archivo .env minimal y retorna el
// valor saneado para name, o "" si no está. No setea nada en el proceso.
func readEnvKeyFromFile(path, name string) string {
	data, err := os.ReadFile(path)
	if err != nil {
		return ""
	}
	prefix := name + "="
	for _, line := range strings.Split(string(data), "\n") {
		line = strings.TrimSpace(line)
		if strings.HasPrefix(line, "#") {
			continue
		}
		if strings.HasPrefix(line, prefix) {
			return sanitizeEncKey(strings.TrimPrefix(line, prefix))
		}
	}
	return ""
}

// generateEncKey produce una passphrase fuerte aleatoria: 32 bytes de
// crypto/rand en base64 RawStd (sin padding). ~43 chars, alta entropía.
func generateEncKey() (string, error) {
	buf := make([]byte, 32)
	if _, err := rand.Read(buf); err != nil {
		return "", fmt.Errorf("crypto/rand: %w", err)
	}
	return base64.RawStdEncoding.EncodeToString(buf), nil
}

// persistEncKey guarda la enc-key en los DOS destinos canónicos (para
// idempotencia y para que Django/compose la vean) y la exporta al proceso
// para el resto del install (validador, bootstrap de keys):
//   - ~/.config/domain/env  (env global preservado entre reinstalls)
//   - .env del cwd          (lo que lee Django y docker compose)
//
// Usa el writer idempotente upsertEnvFile (dev_bootstrap.go:188).
func persistEncKey(key string) error {
	if home, err := os.UserHomeDir(); err == nil {
		dir := filepath.Join(home, ".config", "domain")
		if err := os.MkdirAll(dir, 0o700); err != nil {
			return fmt.Errorf("mkdir %s: %w", dir, err)
		}
		if err := upsertEnvFile(filepath.Join(dir, "env"), encKeyEnvName, key); err != nil {
			return fmt.Errorf("write global env: %w", err)
		}
	}
	if err := upsertEnvFile(".env", encKeyEnvName, key); err != nil {
		return fmt.Errorf("write .env: %w", err)
	}
	_ = os.Setenv(encKeyEnvName, key)
	return nil
}

// validateEncKeyAgainstDB verifica que la enc-key actual pueda descifrar las
// API keys YA cifradas en auth_api_keys. Si hay filas con key_ciphertext y el
// pgp_sym_decrypt falla (passphrase incorrecta → "Wrong key or corrupt data"),
// retorna error: ABORTAR antes de pisar la key (perderías acceso a las keys
// emitidas). Si no hay filas con ciphertext, o descifra OK, retorna nil.
//
// El canary se ejecuta en su propio statement: el error de pgcrypto es
// esperado y NO debe contaminar otras consultas.
func validateEncKeyAgainstDB(ctx context.Context, pool *pgxpool.Pool, key string) error {
	var n int
	if err := pool.QueryRow(ctx,
		`SELECT COUNT(*) FROM auth_api_keys WHERE key_ciphertext IS NOT NULL`,
	).Scan(&n); err != nil {
		return fmt.Errorf("count ciphertext keys: %w", err)
	}
	if n == 0 {
		return nil // no hay nada cifrado todavía: cualquier key sirve.
	}

	var plain string
	err := pool.QueryRow(ctx,
		`SELECT pgp_sym_decrypt(key_ciphertext, $1)::text
		   FROM auth_api_keys
		  WHERE key_ciphertext IS NOT NULL
		  LIMIT 1`, key,
	).Scan(&plain)
	if err != nil {
		return fmt.Errorf(
			"la enc-key (%s) no coincide con las %d API key(s) cifradas en la BD; "+
				"no se pisará nada (perderías acceso a las keys emitidas). "+
				"Restaura la enc-key correcta en ~/.config/domain/env y .env, "+
				"o si es intencional, rota/revoca las keys antes de reinstalar. "+
				"(detalle pgcrypto: %v)",
			encKeyEnvName, n, err)
	}
	return nil
}

// ensureFieldEncKey es el paso de install que garantiza una enc-key válida,
// persistida y compatible con las keys ya cifradas. Devuelve un detalle
// human-readable para el progress y error fatal si la validación aborta.
//
// Orden importa: se llama DESPUÉS de migraciones+seeders (pgcrypto disponible
// y auth_api_keys existe) y junto/antes de "Global MCP env". La BD se abre con
// un pool propio de vida corta (mismo patrón que ensureAPIKey).
func ensureFieldEncKey(databaseURL string) (string, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	pool, err := pgxpool.New(ctx, databaseURL)
	if err != nil {
		return "", fmt.Errorf("connect: %w", err)
	}
	defer pool.Close()

	key := resolveExistingEncKey()
	reused := key != ""

	if !reused {
		key, err = generateEncKey()
		if err != nil {
			return "", err
		}
	}

	// Validar SIEMPRE (reused o fresh): en reinstall confirma que la key
	// persistida sigue descifrando; en fresh con BD vacía es no-op.
	if err := validateEncKeyAgainstDB(ctx, pool, key); err != nil {
		return "", err
	}

	if err := persistEncKey(key); err != nil {
		return "", err
	}

	if reused {
		return "enc-key reusada (persistida previamente)", nil
	}
	return "enc-key generada (32B crypto/rand → base64) y persistida", nil
}
