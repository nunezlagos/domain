package hubuilder

// docFlow define los steps del wizard para mode=doc.
var docFlow = []step{
	{
		Key:    "doc_type",
		Prompt: "¿Tipo de documentación?",
		options: []Option{
			{Value: "api", Label: "API reference", Description: "Endpoint docs, parámetros, ejemplos", Recommended: true},
			{Value: "guide", Label: "Guía/How-to", Description: "Tutorial paso a paso para完成任务"},
			{Value: "concept", Label: "Conceptual", Description: "Arquitectura, design docs, RFC"},
			{Value: "changelog", Label: "Changelog", Description: "Release notes, migration guides"},
		},
		Validate: requireOneOf("api", "guide", "concept", "changelog"),
	},
	{
		Key:    "audience",
		Prompt: "¿Audiencia objetivo?",
		options: []Option{
			{Value: "dev", Label: "Developer", Description: "Ingeniería que integra/extiende", Recommended: true},
			{Value: "ops", Label: "Platform Ops", Description: "Administración de la plataforma"},
			{Value: "end-user", Label: "End user", Description: "Usuario final de la aplicación"},
			{Value: "admin", Label: "Admin", Description: "Org owners y admins"},
		},
		Validate: requireOneOf("dev", "ops", "end-user", "admin"),
	},
	{
		Key:    "topic",
		Prompt: "¿Sobre qué tema? Descripción breve de lo que cubre la documentación",
		Validate: requireNonEmptyString(),
	},
	{
		Key:    "existing",
		Prompt: "¿Hay documentación existente que actualizar?",
		options: []Option{
			{Value: "no", Label: "No, desde cero", Description: "Documentación nueva"},
			{Value: "update", Label: "Actualizar existente", Description: "Mejorar docs actuales", Recommended: true},
			{Value: "migrate", Label: "Migrar desde otro formato", Description: "Reescribir en nuevo formato/ubicación"},
		},
		Validate: requireOneOf("no", "update", "migrate"),
	},
	{
		Key:    "slug",
		Prompt: "Slug corto kebab-case para esta HU (sin prefijo HU-XX.Y)",
		Validate: slugValidator,
	},
	{
		Key:    "summary",
		Prompt: "Resumen ejecutivo de la documentación (2-3 líneas) — qué cubre, para quién, formato",
		Validate: requireNonEmptyString(),
	},
}
