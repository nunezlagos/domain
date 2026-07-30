package mcpserver

import (
	knowledgesvc "nunezlagos/domain/internal/service/knowledge"
	obssvc "nunezlagos/domain/internal/service/observation"
)

// 200 es el único precedente medido del repo y es unánime en 11 sitios (ADR-161.2)
const snippetBytes = 200

// el mismo marcador que agrega el helper truncate del paquete
const marcadorDeTruncado = "\n...[truncated]"

// estas tres acotan el cuerpo en vez de omitirlo: cero texto rompe en silencio a
// context-preservation, al agente domain-memory y al hook UserPromptSubmit
// la key NO se renombra: el hook la lee por nombre exacto y ya trunca a 160

func proyectarBusquedaDeObservaciones(results []obssvc.SearchResult) []map[string]any {
	out := make([]map[string]any, 0, len(results))
	for _, r := range results {
		out = append(out, map[string]any{
			"id":               r.ID.String(),
			"content":          truncate(r.Content, snippetBytes),
			"content_len":      len(r.Content),
			"observation_type": r.ObservationType,
			"tags":             r.Tags,
			"score":            r.Score,
			"bm25_rank":        r.BM25Rank,
			"vector_rank":      r.VectorRank,
			"created_at":       r.CreatedAt,
		})
	}
	return out
}

func proyectarContextoDeObservaciones(obs []obssvc.Observation) []map[string]any {
	out := make([]map[string]any, 0, len(obs))
	for _, o := range obs {
		out = append(out, map[string]any{
			"id":               o.ID.String(),
			"content":          truncate(o.Content, snippetBytes),
			"content_len":      len(o.Content),
			"observation_type": o.ObservationType,
			"tags":             o.Tags,
			"created_at":       o.CreatedAt,
		})
	}
	return out
}

// el Snippet de knowledge ERA el Content completo, sin truncar en ningún punto
func proyectarBusquedaDeKnowledge(results []knowledgesvc.SearchResult) []map[string]any {
	out := make([]map[string]any, 0, len(results))
	for _, r := range results {
		out = append(out, map[string]any{
			"document_id": r.DocumentID.String(),
			"chunk_id":    r.ChunkID.String(),
			"chunk_index": r.ChunkIndex,
			"title":       r.Title,
			"snippet":     truncate(r.Snippet, snippetBytes),
			"snippet_len": len(r.Snippet),
			"score":       r.Score,
			"project_id":  r.ProjectID.String(),
			"created_at":  r.CreatedAt,
		})
	}
	return out
}
