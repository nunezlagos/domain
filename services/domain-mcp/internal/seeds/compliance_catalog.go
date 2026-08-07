package seeds

// Catálogo inicial de marcos normativos y controles (issue-56.4).
//
// SOLO LEYES Y REGLAMENTOS EN ESTA CARGA. El esquema soporta normas técnicas desde el día uno
// —ISO 27001 entra con tipo='norma_tecnica', jurisdiccion NULL, certificable=true y
// fuente_tipo='solo_referencia'— pero no se siembra acá porque su texto es de pago y las
// referencias de cláusula hay que tomarlas de la norma comprada, no de memoria ni de un blog. La
// numeración del Anexo A además cambió entre :2013 y :2022, así que una referencia sin verificar
// contra la edición sería falsa precisión.
//
// Las referencias de artículo de abajo son las que se pudieron atar a la fuente. Donde no hay
// certeza, el campo va vacío: es preferible un crosswalk sin cita a una cita inventada — la regla
// que compliance-cl aplica y que este catálogo hereda.

// ComplianceFrameworkSeed es una entrada del catálogo de marcos.
type ComplianceFrameworkSeed struct {
	Slug         string
	Nombre       string
	Tipo         string
	Jurisdiccion string // vacío = no territorial (normas técnicas)
	Obligatorio  bool
	Certificable bool
	VigenteDesde string // YYYY-MM-DD; vacío = ya rige
	FuenteTipo   string
	Descripcion  string
}

// ComplianceControlSeed es un control reutilizable del catálogo.
type ComplianceControlSeed struct {
	Slug           string
	Nombre         string
	Descripcion    string
	ComoSeVerifica string
}

// ComplianceCrosswalkSeed vincula un control con el marco que lo exige y su referencia.
type ComplianceCrosswalkSeed struct {
	FrameworkSlug string
	ControlSlug   string
	Referencia    string
}

// ComplianceFrameworkSeeds — los marcos de la carga inicial.
func ComplianceFrameworkSeeds() []ComplianceFrameworkSeed {
	return []ComplianceFrameworkSeed{
		{
			Slug: "ley-21719", Nombre: "Ley 21.719 — Protección de datos personales",
			Tipo: "ley", Jurisdiccion: "CL", Obligatorio: true, Certificable: false,
			// entra en vigencia en diciembre de 2026: hasta entonces las obligaciones existen
			// pero no rigen, y esa diferencia decide la severidad del finding
			VigenteDesde: "2026-12-01", FuenteTipo: "texto_libre",
			Descripcion: "Regula el tratamiento de datos personales en Chile y crea la Agencia de Protección de Datos.",
		},
		{
			Slug: "ley-21595", Nombre: "Ley 21.595 — Delitos económicos",
			Tipo: "ley", Jurisdiccion: "CL", Obligatorio: true, Certificable: false,
			FuenteTipo:  "texto_libre",
			Descripcion: "Responsabilidad penal de la persona jurídica y modelo de prevención de delitos.",
		},
		{
			Slug: "gdpr", Nombre: "Reglamento (UE) 2016/679 — GDPR",
			Tipo: "reglamento", Jurisdiccion: "EU", Obligatorio: true, Certificable: false,
			FuenteTipo:  "texto_libre",
			Descripcion: "Aplica a quien trate datos de personas en la UE, con independencia de dónde esté establecido.",
		},
	}
}

// ComplianceControlSeeds — controles compartidos por varios marcos. Son los que hacen valioso el
// crosswalk: cada uno se implementa y audita UNA vez.
func ComplianceControlSeeds() []ComplianceControlSeed {
	return []ComplianceControlSeed{
		{
			Slug: "cifrado-en-reposo", Nombre: "Cifrado de datos en reposo",
			Descripcion:    "Los datos personales almacenados están cifrados.",
			ComoSeVerifica: "Configuración del motor de base de datos y del almacenamiento de objetos.",
		},
		{
			Slug: "cifrado-en-transito", Nombre: "Cifrado en tránsito",
			Descripcion:    "Todo transporte de datos personales usa TLS.",
			ComoSeVerifica: "Configuración del borde HTTP y de las conexiones a la base.",
		},
		{
			Slug: "registro-de-tratamientos", Nombre: "Registro de actividades de tratamiento",
			Descripcion:    "Existe un inventario de qué datos se tratan, con qué finalidad y por cuánto tiempo.",
			ComoSeVerifica: "Documento vigente; no es verificable desde el código.",
		},
		{
			Slug: "plazos-de-retencion", Nombre: "Plazos de retención definidos",
			Descripcion:    "Cada categoría de dato tiene un plazo de conservación y un borrado efectivo al vencer.",
			ComoSeVerifica: "Job o política de retención aplicada, más el plazo declarado por categoría.",
		},
		{
			Slug: "derechos-del-titular", Nombre: "Ejercicio de derechos del titular",
			Descripcion:    "Hay un canal para acceso, rectificación, cancelación y oposición.",
			ComoSeVerifica: "Endpoint o procedimiento documentado con plazo de respuesta.",
		},
		{
			Slug: "notificacion-de-brechas", Nombre: "Plan de respuesta y notificación de brechas",
			Descripcion:    "Existe un procedimiento para detectar, contener y notificar una vulneración.",
			ComoSeVerifica: "Plan escrito con responsables y plazos.",
		},
		{
			Slug: "registro-de-auditoria", Nombre: "Registro de auditoría con actor",
			Descripcion:    "Las operaciones sobre datos personales quedan registradas con quién y cuándo.",
			ComoSeVerifica: "Tabla de auditoría poblada, con actor e IP.",
		},
		{
			Slug: "control-de-acceso", Nombre: "Control de acceso a datos personales",
			Descripcion:    "El acceso está restringido por rol y autenticado.",
			ComoSeVerifica: "Modelo de permisos y aislamiento por tenant.",
		},
	}
}

// ComplianceCrosswalkSeeds — qué marco exige qué control, con la referencia de cada uno.
//
// Acá se ve por qué el crosswalk vale: cifrado-en-reposo aparece bajo los tres marcos. Se
// implementa una vez y se reporta tres veces.
func ComplianceCrosswalkSeeds() []ComplianceCrosswalkSeed {
	return []ComplianceCrosswalkSeed{
		// GDPR — las referencias del reglamento son públicas y verificables
		{FrameworkSlug: "gdpr", ControlSlug: "cifrado-en-reposo", Referencia: "Art. 32"},
		{FrameworkSlug: "gdpr", ControlSlug: "cifrado-en-transito", Referencia: "Art. 32"},
		{FrameworkSlug: "gdpr", ControlSlug: "registro-de-tratamientos", Referencia: "Art. 30"},
		{FrameworkSlug: "gdpr", ControlSlug: "derechos-del-titular", Referencia: "Arts. 15-22"},
		{FrameworkSlug: "gdpr", ControlSlug: "notificacion-de-brechas", Referencia: "Arts. 33-34"},
		{FrameworkSlug: "gdpr", ControlSlug: "control-de-acceso", Referencia: "Art. 32"},
		{FrameworkSlug: "gdpr", ControlSlug: "plazos-de-retencion", Referencia: "Art. 5.1.e"},

		// Ley 21.719 — sin número de artículo donde no se pudo verificar contra el texto
		// oficial. Un crosswalk sin cita sirve igual para saber QUÉ se exige; una cita
		// inventada corrompe el reporte entero.
		{FrameworkSlug: "ley-21719", ControlSlug: "cifrado-en-reposo"},
		{FrameworkSlug: "ley-21719", ControlSlug: "cifrado-en-transito"},
		{FrameworkSlug: "ley-21719", ControlSlug: "registro-de-tratamientos"},
		{FrameworkSlug: "ley-21719", ControlSlug: "derechos-del-titular"},
		{FrameworkSlug: "ley-21719", ControlSlug: "notificacion-de-brechas"},
		{FrameworkSlug: "ley-21719", ControlSlug: "plazos-de-retencion"},
		{FrameworkSlug: "ley-21719", ControlSlug: "control-de-acceso"},

		// Ley 21.595 — no es de datos personales: lo que exige del sistema es trazabilidad
		{FrameworkSlug: "ley-21595", ControlSlug: "registro-de-auditoria"},
		{FrameworkSlug: "ley-21595", ControlSlug: "control-de-acceso"},
	}
}
