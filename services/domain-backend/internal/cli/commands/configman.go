// issue-14.3 — `domain config view` y `domain man`.
package commands

import (
	"fmt"
	"os"

	clicli "nunezlagos/domain/internal/cli/client"
)

// configCmd muestra la configuración resuelta. El API key NUNCA se imprime
// completo: solo el prefix (security.md).
func configCmd(args []string) int {
	if len(args) == 0 || args[0] != "view" {
		fmt.Println("Uso: domain config view")
		return 2
	}
	gf, _ := parseGlobalFlags(args[1:])
	var c *clicli.Client
	var err error
	if gf.Config != "" {
		c, err = clicli.NewFromFile(gf.Config)
	} else {
		c, err = clicli.NewFromEnv()
	}
	if err != nil {
		fmt.Fprintln(os.Stderr, "error:", err)
		return 1
	}
	fmt.Printf("base_url:        %s\n", c.BaseURL)
	fmt.Printf("api_key_prefix:  %s\n", keyPrefix(c.APIKey))
	source := "env (DOMAIN_API_KEY / DOMAIN_BASE_URL)"
	if gf.Config != "" {
		source = "file: " + gf.Config
	}
	fmt.Printf("source:          %s\n", source)
	return 0
}

func keyPrefix(key string) string {
	if len(key) <= 16 {
		if key == "" {
			return "(no configurado)"
		}
		return key[:len(key)/2] + "..."
	}
	return key[:16] + "..."
}

// manCmd imprime la man page (troff). Instalar:
//
//	domain man > /usr/local/share/man/man1/domain.1
func manCmd() int {
	fmt.Print(manPage)
	return 0
}

const manPage = `.TH DOMAIN 1 "2026" "domain" "Domain CLI Manual"
.SH NAME
domain \- CLI para la plataforma Domain (memoria persistente + agents + flows)
.SH SYNOPSIS
.B domain
\fI<resource>\fR \fI<action>\fR [\fIargs\fR] [\fIflags\fR]
.SH DESCRIPTION
Interactúa con la API Domain: proyectos, observaciones de memoria, agents,
flows, skills y búsqueda híbrida.
.SH COMMANDS
.TP
.B projects ls | get <slug> | create <slug> <name>
Gestión de proyectos.
.TP
.B observations ls --project <slug> | save --project <slug> <content>
Memoria persistente (alias: obs).
.TP
.B agents ls | get <id> | run <id> <input>
Ejecución de agents.
.TP
.B flows ls | run <id>
Ejecución de flows.
.TP
.B skills ls [--type X] [--tag Y]
Catálogo de skills.
.TP
.B search <query> [--limit N]
Búsqueda híbrida (vector + texto).
.TP
.B config view
Configuración resuelta (API key solo prefix).
.TP
.B completion bash|zsh|fish|powershell
Scripts de autocompletado.
.SH FLAGS
.TP
.B --format json|table|yaml|csv
Formato de salida (default: table).
.TP
.B --verbose
Imprime las requests HTTP a stderr.
.TP
.B --quiet, -q
Solo errores.
.TP
.B --config <path>
Archivo YAML con api_key/base_url.
.SH ENVIRONMENT
.TP
.B DOMAIN_API_KEY
API key (requerido).
.TP
.B DOMAIN_BASE_URL
Base URL (default http://localhost:8000).
.SH EXAMPLES
.nf
domain projects ls
domain search "pgvector" --limit 5 --format json
domain agents run my-agent "Revisá el PR"
domain obs save --project demo "Decidimos usar X"
.fi
.SH SEE ALSO
Documentación: docs/ en el repositorio del proyecto.
`
