package main

import (
	"bufio"
	"context"
	"fmt"
	"os"
	"strings"

	"nunezlagos/domain/internal/auth/session"
)

// runAdminPasswd (REQ-72): CLI para setear/cambiar la contraseña de un
// user + opcionalmente asignarle un rol. Pensado para bootstrap: la
// primera vez no hay forma de loguear si nadie tiene password seteado.
//
// Uso:
//   domain admin-passwd <email>                       # solo password
//   domain admin-passwd <email> --role=admin          # password + role
//   domain admin-passwd <email> --role=admin --role=developer
//
// Lee password de stdin (no echo). Si STDIN no es tty, lee 1 línea.
func runAdminPasswd(args []string) {
	if len(args) < 1 {
		fmt.Fprintln(os.Stderr, "Uso: domain admin-passwd <email> [--role=<slug>]...")
		os.Exit(2)
	}
	email := strings.ToLower(strings.TrimSpace(args[0]))
	if email == "" {
		fmt.Fprintln(os.Stderr, "email requerido")
		os.Exit(2)
	}
	var roles []string
	for _, a := range args[1:] {
		if strings.HasPrefix(a, "--role=") {
			roles = append(roles, strings.TrimPrefix(a, "--role="))
		}
	}

	fmt.Print("Nueva contraseña: ")
	scanner := bufio.NewScanner(os.Stdin)
	if !scanner.Scan() {
		fmt.Fprintln(os.Stderr, "no se pudo leer password")
		os.Exit(1)
	}
	pw := scanner.Text()
	if len(pw) < 8 {
		fmt.Fprintln(os.Stderr, "password muy corta (mín 8 chars)")
		os.Exit(1)
	}

	ctx := context.Background()
	dsn := os.Getenv("DOMAIN_DATABASE_AUTH_URL")
	if dsn == "" {
		dsn = os.Getenv("DOMAIN_DATABASE_URL")
	}
	if dsn == "" {
		fmt.Fprintln(os.Stderr, "DOMAIN_DATABASE_AUTH_URL/DOMAIN_DATABASE_URL no seteado")
		os.Exit(1)
	}
	pool, err := pgxpoolNew(ctx, dsn)
	if err != nil {
		fmt.Fprintln(os.Stderr, "open pool:", err)
		os.Exit(1)
	}
	defer pool.Close()

	svc := session.New(pool)
	if err := svc.SetPassword(ctx, email, pw); err != nil {
		fmt.Fprintln(os.Stderr, "set password:", err)
		os.Exit(1)
	}
	fmt.Printf("✓ password actualizada para %s\n", email)

	for _, r := range roles {
		if err := svc.GrantRole(ctx, email, r, nil); err != nil {
			fmt.Fprintf(os.Stderr, "✗ grant role %s: %v\n", r, err)
			continue
		}
		fmt.Printf("✓ role asignado: %s\n", r)
	}
}
