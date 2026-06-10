package issuebuilder

// refactorFlow define los steps del wizard para mode=refactor.
var refactorFlow = []step{
	{
		Key:    "scope",
		Prompt: "¿Scope del refactor?",
		options: []Option{
			{Value: "internal-api", Label: "API interna", Description: "Interfaces/contratos entre paquetes", Recommended: true},
			{Value: "db", Label: "Base de datos", Description: "Schema, queries, migraciones"},
			{Value: "data-flow", Label: "Data flow", Description: "Pipeline, transformación, ETL"},
			{Value: "config", Label: "Configuración", Description: "Env vars, boot, initialization"},
			{Value: "cross-cutting", Label: "Cross-cutting", Description: "Logging, metrics, errors común"},
		},
		Validate: requireNonEmptyString(),
	},
	{
		Key:    "reason",
		Prompt: "¿Razón principal del refactor?",
		options: []Option{
			{Value: "tech-debt", Label: "Deuda técnica", Description: "Código difícil de mantener/extender", Recommended: true},
			{Value: "perf", Label: "Performance", Description: "Cuello de botella medido"},
			{Value: "maintainability", Label: "Mantenibilidad", Description: "Reducir complejidad cognitiva"},
			{Value: "compliance", Label: "Compliance", Description: "Audit, regulatory, security standard"},
		},
		Validate: requireOneOf("tech-debt", "perf", "maintainability", "compliance"),
	},
	{
		Key:    "current_state",
		Prompt: "Descripción del estado actual (qué se va a cambiar)",
		Validate: requireNonEmptyString(),
	},
	{
		Key:    "target_state",
		Prompt: "Descripción del estado deseado post-refactor",
		Validate: requireNonEmptyString(),
	},
	{
		Key:    "migration",
		Prompt: "¿Estrategia de migración?",
		options: []Option{
			{Value: "big-bang", Label: "Big Bang", Description: "Todo de una vez", Recommended: true},
			{Value: "incremental", Label: "Incremental", Description: "Remplazo progresivo conviviendo"},
			{Value: "parallel", Label: "Parallel run", Description: "Ambas implementaciones en paralelo"},
		},
		Validate: requireOneOf("big-bang", "incremental", "parallel"),
	},
	{
		Key:    "backward_compat",
		Prompt: "¿Requiere backward compatibility?",
		options: []Option{
			{Value: "yes", Label: "Sí", Description: "APIs/contratos existentes no pueden romperse", Recommended: true},
			{Value: "no", Label: "No", Description: "Breaking change aceptado"},
		},
		Validate: requireOneOf("yes", "no"),
	},
	{
		Key:    "slug",
		Prompt: "Slug corto kebab-case para esta HU (sin prefijo HU-XX.Y)",
		Validate: slugValidator,
	},
	{
		Key:    "summary",
		Prompt: "Resumen ejecutivo del refactor (2-3 líneas) — qué se cambia, por qué, impacto esperado",
		Validate: requireNonEmptyString(),
	},
}
