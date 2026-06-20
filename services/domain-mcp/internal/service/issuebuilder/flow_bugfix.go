package issuebuilder

// bugFixFlow define los steps del wizard para mode=bug-fix.
var bugFixFlow = []step{
	{
		Key:    "severity",
		Prompt: "¿Qué severidad tiene el bug?",
		options: []Option{
			{Value: "critical", Label: "Critical", Description: "Producción caída, datos perdidos"},
			{Value: "high", Label: "High", Description: "Funcionalidad mayor rota sin workaround", Recommended: true},
			{Value: "medium", Label: "Medium", Description: "Funcionalidad menor rota con workaround"},
			{Value: "low", Label: "Low", Description: "Cosmético, edge case raro"},
		},
		Validate: requireOneOf("critical", "high", "medium", "low"),
	},
	{
		Key:    "component",
		Prompt: "¿Qué componente/sistema?",
		options: []Option{
			{Value: "api", Label: "API HTTP"},
			{Value: "db", Label: "Base de datos"},
			{Value: "cli", Label: "CLI"},
			{Value: "mcp", Label: "MCP"},
			{Value: "auth", Label: "Auth/Security"},
			{Value: "webhook", Label: "Webhooks"},
			{Value: "runner", Label: "Agent/Flow runner"},
			{Value: "ui", Label: "UI/Frontend"},
		},
		Validate: requireNonEmptyString(),
	},
	{
		Key:    "root_cause",
		Prompt: "¿Tipo de causa raíz?",
		options: []Option{
			{Value: "logic", Label: "Lógica", Description: "Algoritmo incorrecto o condición mal manejada", Recommended: true},
			{Value: "race", Label: "Race condition", Description: "Concurrencia sin sync"},
			{Value: "perf", Label: "Performance", Description: "Timeout, OOM, latencia"},
			{Value: "security", Label: "Security", Description: "Input validation, auth bypass"},
			{Value: "ux", Label: "UX", Description: "Mensaje confuso, flow roto"},
		},
		Validate: requireOneOf("logic", "race", "perf", "security", "ux"),
	},
	{
		Key:    "has_repro",
		Prompt: "¿Hay pasos de reproducción claros?",
		options: []Option{
			{Value: "yes", Label: "Sí", Description: "Pasos conocidos y repetibles", Recommended: true},
			{Value: "no", Label: "No", Description: "Intermitente o no reproducible"},
		},
		Validate: requireOneOf("yes", "no"),
	},
	{
		Key:    "expected",
		Prompt: "¿Cuál es el comportamiento esperado?",
		Validate: requireNonEmptyString(),
	},
	{
		Key:    "actual",
		Prompt: "¿Cuál es el comportamiento actual (bug)?",
		Validate: requireNonEmptyString(),
	},
	{
		Key:    "slug",
		Prompt: "Slug corto kebab-case para esta HU (sin prefijo HU-XX.Y)",
		Validate: slugValidator,
	},
	{
		Key:    "summary",
		Prompt: "Resumen ejecutivo del bug (2-3 líneas) — qué falla, dónde, impacto",
		Validate: requireNonEmptyString(),
	},
}
