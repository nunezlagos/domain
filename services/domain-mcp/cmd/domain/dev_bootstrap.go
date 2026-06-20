// `domain dev-bootstrap` — primer arranque dev plug-and-play.
//
// Crea org + admin user + emite api_key + escribe .env del repo current.
// Quita la fricción del primer arranque tras `migrate up`.
//
// NO usar en prod — para prod usar el flow OTP normal.

package main

import (
	"context"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"

	"nunezlagos/domain/internal/auth/apikey"
	"nunezlagos/domain/internal/config"
)

func runDevBootstrap(args []string) {
	orgSlug := "dev"
	orgName := "Dev"
	userEmail := "admin@example.local"
	envFile := ".env"
	envName := "DOMAIN_API_KEY"
	envOverride := false

	for i := 0; i < len(args); i++ {
		switch args[i] {
		case "--org-slug":
			if i+1 < len(args) {
				orgSlug = args[i+1]
				i++
			}
		case "--org-name":
			if i+1 < len(args) {
				orgName = args[i+1]
				i++
			}
		case "--user-email":
			if i+1 < len(args) {
				userEmail = args[i+1]
				i++
			}
		case "--env-file":
			if i+1 < len(args) {
				envFile = args[i+1]
				i++
			}
		case "--env-var":
			if i+1 < len(args) {
				envName = args[i+1]
				i++
			}
		case "--no-env":
			envFile = ""
		case "--force":
			envOverride = true
		case "-h", "--help":
			fmt.Println(`domain dev-bootstrap — primer arranque dev plug-and-play

Crea org + admin user + emite api_key + escribe .env del repo current.

Uso:
  domain dev-bootstrap [opts]

Opts:
  --org-slug <slug>      slug de la org (default 'dev')
  --org-name <name>      display name (default 'Dev')
  --user-email <email>   admin email (default 'admin@example.local')
  --env-file <path>      escribe DOMAIN_API_KEY=... en este file (default .env)
                         use --no-env para no escribir
  --env-var <NAME>       nombre de la var en .env (default DOMAIN_API_KEY)
  --force                revoca dev-bootstrap auth_api_keys previas
  -h, --help

Requiere DOMAIN_DATABASE_URL apuntando a la BD ya migrada.

NO USAR EN PROD — el user no tiene OTP real; el api_key sale en stdout.
Para prod usar el flow normal: domain server + POST /auth/request-otp.`)
			return
		}
	}

	cfg, err := config.Load()
	if err != nil {
		fmt.Fprintf(os.Stderr, "config: %v\n", err)
		os.Exit(1)
	}
	ctx := context.Background()
	pool, err := pgxpool.New(ctx, cfg.DatabaseURL)
	if err != nil {
		fmt.Fprintf(os.Stderr, "db open: %v\n", err)
		os.Exit(1)
	}
	defer pool.Close()

	// 1) Org idempotente.
	// ISSUE-21.6 Fase D clean Round 3: tabla organizations se dropea en
	// Fase C. dev-bootstrap es una herramienta setup-time obsoleta en
	// single-org (la org canónica existe implícita). Mantenemos el flujo
	// best-effort: si la tabla no existe (Fase C), usamos un UUID fijo
	// canónico sin intentar el INSERT.
	var orgID string
	err = pool.QueryRow(ctx,
		`INSERT INTO organizations (name, slug) VALUES ($1, $2)
		 ON CONFLICT (slug) DO UPDATE SET name = EXCLUDED.name
		 RETURNING id`,
		orgName, orgSlug,
	).Scan(&orgID)
	if err != nil {
		// Tabla dropeada: usar UUID canónico single-org.
		orgID = "00000000-0000-0000-0000-000000000001"
		fmt.Fprintf(os.Stderr, "warn: organizations table not available, using canonical single-org UUID %s\n", orgID)
	}
	fmt.Printf("✓ org id=%s slug=%s\n", orgID, orgSlug)

	// 2) Admin user idempotente.
	var userID string
	err = pool.QueryRow(ctx,
		`INSERT INTO users (email, name, role)
		 VALUES ($1, $2, 'owner')
		 ON CONFLICT (email) DO UPDATE SET role = 'owner'
		 RETURNING id`,
		userEmail, "Admin Local",
	).Scan(&userID)
	if err != nil {
		fmt.Fprintf(os.Stderr, "create user: %v\n", err)
		os.Exit(1)
	}
	fmt.Printf("✓ user id=%s email=%s\n", userID, userEmail)

	// 3) Emite api key fresh. --force revoca las dev-bootstrap previas.
	if envOverride {
		_, _ = pool.Exec(ctx,
			`UPDATE auth_api_keys SET revoked_at = now()
			 WHERE user_id = $1 AND name LIKE 'dev-bootstrap-%' AND revoked_at IS NULL`, userID)
	}

	rawKey, prefix, hash, err := apikey.Generate("dev")
	if err != nil {
		fmt.Fprintf(os.Stderr, "generate api_key: %v\n", err)
		os.Exit(1)
	}
	keyName := "dev-bootstrap-" + time.Now().Format("20060102-150405")

	var apiKeyID string
	err = pool.QueryRow(ctx,
		`INSERT INTO auth_api_keys (user_id, name, key_prefix, key_hash)
		 VALUES ($1, $2, $3, $4) RETURNING id`,
		userID, keyName, prefix, hash,
	).Scan(&apiKeyID)
	if err != nil {
		fmt.Fprintf(os.Stderr, "create api_key: %v\n", err)
		os.Exit(1)
	}
	fmt.Printf("✓ api_key id=%s prefix=%s\n", apiKeyID, prefix)

	fmt.Println()
	fmt.Println("API KEY (guardalo, no se vuelve a mostrar):")
	fmt.Println()
	fmt.Println("  " + rawKey)
	fmt.Println()

	if envFile == "" {
		fmt.Println("(--no-env) no escribí archivo .env")
		fmt.Println("Para usar: export " + envName + "=" + rawKey)
		return
	}

	if err := upsertEnvFile(envFile, envName, rawKey); err != nil {
		fmt.Fprintf(os.Stderr, "write .env: %v\n", err)
		os.Exit(1)
	}
	fmt.Printf("✓ %s actualizado: %s=...\n", envFile, envName)
	fmt.Println()
	fmt.Println("Probá:")
	fmt.Println("  source " + envFile)
	fmt.Println("  domain projects ls")
	fmt.Println("  curl -H \"Authorization: Bearer $" + envName + "\" http://localhost:8000/api/v1/prompt \\")
	fmt.Println("    -d '{\"raw_text\":\"hola, ¿cómo estás?\"}'")
}

// upsertEnvFile reemplaza la línea NAME=... si existe; sino la agrega.
func upsertEnvFile(path, name, value string) error {
	data, _ := os.ReadFile(path) // ok si no existe
	lines := strings.Split(string(data), "\n")
	prefix := name + "="
	found := false
	for i, line := range lines {
		if strings.HasPrefix(strings.TrimSpace(line), prefix) {
			lines[i] = prefix + value
			found = true
			break
		}
	}
	if !found {
		if len(lines) > 0 && lines[len(lines)-1] != "" {
			lines = append(lines, "")
		}
		lines = append(lines, prefix+value)
	}
	out := strings.Join(lines, "\n")
	if !strings.HasSuffix(out, "\n") {
		out += "\n"
	}
	return os.WriteFile(path, []byte(out), 0o600)
}
