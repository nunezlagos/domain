package hubuilder

import (
	"context"
	"fmt"
	"regexp"
	"strings"

	"github.com/jackc/pgx/v5/pgxpool"
)

// step describe un paso del wizard. Implementación interna: ni Option fn ni
// Validate son exportadas porque el flow se define en compile-time.
type step struct {
	Key       string
	Prompt    string
	options   []Option
	optionsFn func(context.Context, *pgxpool.Pool, *Draft) ([]Option, error)
	Validate  func(any) error
}

// flowsByMode registra las secuencias de steps por mode.
// Por ahora solo feature está implementado.
var flowsByMode = map[string][]step{
	ModeFeature:  featureFlow,
	ModeBugFix:   nil, // futura HU
	ModeRefactor: nil,
	ModeDoc:      nil,
	ModeRFC:      nil,
}

var slugRegex = regexp.MustCompile(`^[a-z0-9][a-z0-9-]*[a-z0-9]$`)

var featureFlow = []step{
	{
		Key:    "change_type",
		Prompt: "¿Qué tipo de cambio?",
		options: []Option{
			{Value: "feature", Label: "Nueva capacidad", Description: "Funcionalidad user-facing nueva", Recommended: true},
			{Value: "infrastructure", Label: "Infraestructura", Description: "Tooling/CI/observability"},
			{Value: "hardening", Label: "Hardening", Description: "Seguridad o robustez"},
		},
		Validate: requireOneOf("feature", "infrastructure", "hardening"),
	},
	{
		Key:    "audience",
		Prompt: "¿Quién es la audiencia principal?",
		optionsFn: func(ctx context.Context, pool *pgxpool.Pool, d *Draft) ([]Option, error) {
			// Tabla audiences fue archivada (ver HU-01.9 archivada). Devolvemos
			// catálogo estático mientras se reemplaza por agent_personalities (HU-08.5).
			return []Option{
				{Value: "dx-engineer", Label: "DX engineer"},
				{Value: "platform-engineer", Label: "Platform engineer"},
				{Value: "security-officer", Label: "Security officer"},
				{Value: "org-owner", Label: "Org owner"},
				{Value: "org-admin", Label: "Org admin"},
				{Value: "org-member", Label: "Org member"},
				{Value: "platform-admin", Label: "Platform admin"},
				{Value: "auditor", Label: "Auditor"},
				{Value: "integrator", Label: "Integrator"},
				{Value: "data-scientist", Label: "Data scientist"},
			}, nil
		},
		Validate: requireNonEmptyString(),
	},
	{
		Key:    "req_parent",
		Prompt: "¿Bajo qué REQ vive esta HU? (slug, ej REQ-03-memory-system)",
		Validate: func(a any) error {
			s, ok := a.(string)
			if !ok || s == "" {
				return fmt.Errorf("string required")
			}
			if !strings.HasPrefix(s, "REQ-") {
				return fmt.Errorf("must start with REQ-")
			}
			return nil
		},
	},
	{
		Key:    "effort",
		Prompt: "¿Esfuerzo estimado?",
		options: []Option{
			{Value: "S", Label: "Small", Description: "<1 día"},
			{Value: "M", Label: "Medium", Description: "1-3 días"},
			{Value: "L", Label: "Large", Description: "1+ semana"},
		},
		Validate: requireOneOf("S", "M", "L"),
	},
	{
		Key:    "priority",
		Prompt: "¿Prioridad tentativa?",
		options: []Option{
			{Value: "alta", Label: "Alta"},
			{Value: "media", Label: "Media", Recommended: true},
			{Value: "baja", Label: "Baja"},
		},
		Validate: requireOneOf("alta", "media", "baja"),
	},
	{
		Key:    "slug",
		Prompt: "¿Cuál es el slug corto kebab-case para esta HU? (sin prefijo HU-XX.Y)",
		Validate: func(a any) error {
			s, ok := a.(string)
			if !ok || s == "" {
				return fmt.Errorf("string required")
			}
			if !slugRegex.MatchString(s) {
				return fmt.Errorf("must match ^[a-z0-9][a-z0-9-]*[a-z0-9]$")
			}
			return nil
		},
	},
	{
		Key:    "goal",
		Prompt: "Describí en una línea cómo se ve el éxito (Goal del usuario)",
		Validate: requireNonEmptyString(),
	},
	{
		Key:    "summary",
		Prompt: "Resumen ejecutivo (2-3 líneas) — qué hace y por qué importa",
		Validate: requireNonEmptyString(),
	},
}

func requireOneOf(allowed ...string) func(any) error {
	return func(a any) error {
		s, ok := a.(string)
		if !ok {
			return fmt.Errorf("string required")
		}
		for _, v := range allowed {
			if s == v {
				return nil
			}
		}
		return fmt.Errorf("must be one of %v", allowed)
	}
}

func requireNonEmptyString() func(any) error {
	return func(a any) error {
		s, ok := a.(string)
		if !ok {
			return fmt.Errorf("string required")
		}
		if strings.TrimSpace(s) == "" {
			return fmt.Errorf("non-empty string required")
		}
		return nil
	}
}
