// Package embedding concentra el predicado de "embedding pendiente" y el worker que
// los completa fuera del camino del request (DOMAINSERV-227).
//
// El predicado vivía SOLO en cmd/domain/embed_backfill.go. Al aparecer un segundo
// consumidor —el completer que corre dentro del server— copiarlo habría dejado dos
// definiciones de qué cuenta como pendiente, y la que quedara atrás fallaría en
// silencio: un vector en cero que un lado repuebla y el otro considera válido es
// indistinguible de "no había nada pendiente". Es el mismo razonamiento por el que
// dsnCandidatosMantenimiento es una sola lista compartida.
package embedding

import "fmt"

// Target es una tabla con columna de embedding regenerable.
type Target struct {
	Table        string
	TextCol      string
	EmbCol       string
	HasDeletedAt bool
}

// Targets son las tablas que se repueblan. knowledge_chunks no tiene deleted_at:
// incluir el filtro la rompería con "column does not exist".
func Targets() []Target {
	return []Target{
		{Table: "knowledge_observations", TextCol: "content", EmbCol: "embedding", HasDeletedAt: true},
		{Table: "knowledge_chunks", TextCol: "content", EmbCol: "embedding", HasDeletedAt: false},
		// DOMAINSERV-157: skills era la única tabla con embedding fuera del backfill, así
		// que sus vectores en cero no se regeneraban ni con el binario arreglado. TextCol
		// es una expresión porque el service embebe name + description, no una columna
		{Table: "skills", TextCol: "name || ' ' || COALESCE(description, '')", EmbCol: "embedding", HasDeletedAt: true},
	}
}

// KnowledgeChunks es el target que le interesa al completer: es el único que está en
// el camino de una escritura interactiva (domain_knowledge_save).
func KnowledgeChunks() Target {
	for _, t := range Targets() {
		if t.Table == "knowledge_chunks" {
			return t
		}
	}
	panic("knowledge_chunks salió de Targets(): el completer se quedaría sin qué completar")
}

// PendingRowsQuery arma el SELECT de filas pendientes: sin embedding, o con uno en
// cero. Ese segundo caso existe porque el embedder degradado a noop (DOMAINSERV-157)
// escribió vectores de ceros, y un vector de ceros NO es NULL: quedaban fuera para
// siempre mientras competían en el ranking con la misma distancia contra cualquier
// búsqueda. Se detectan por el producto interno consigo mismo —-||v||², cero solo en
// el vector nulo— porque l2_norm es ambiguo en pgvector y vector_dims obligaría a
// conocer la dimensión.
//
// La idempotencia se conserva: una fila con norma real no vuelve a tomarse, así que
// re-correrlo no gasta llamadas al provider.
func PendingRowsQuery(t Target) string {
	deleted := ""
	if t.HasDeletedAt {
		deleted = "\n\t\t   AND deleted_at IS NULL"
	}
	return fmt.Sprintf(
		`SELECT %s, %s FROM %s
		 WHERE (%s IS NULL OR (%s <#> %s) = 0)%s
		   AND LENGTH(TRIM(%s)) > 0
		 ORDER BY created_at ASC
		 LIMIT $1`,
		"id", t.TextCol, t.Table, t.EmbCol, t.EmbCol, t.EmbCol, deleted, t.TextCol,
	)
}
