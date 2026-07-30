package mcpserver

import (
	knowledgesvc "nunezlagos/domain/internal/service/knowledge"
	obssvc "nunezlagos/domain/internal/service/observation"
)

// snippetBytes sale del único precedente medido del repo, y es unánime: 11 sitios ya
// truncan a 200 (service/search/service.go ×3, service/timeline/service.go ×7,
// service/orchestrator/status.go ×1). Los 3000/4000 de project_index_tools.go son
// cuerpos de documento, no previews de listado. Fijarlo acá evita que cada una de las
// tres tools de búsqueda elija el suyo (DOMAINSERV-161, ADR-161.2).
const snippetBytes = 200

// el mismo marcador que agrega el helper truncate del paquete
const marcadorDeTruncado = "\n...[truncated]"

// Estas tres tools NO omiten el cuerpo como los listados: lo acotan. Cero texto rompe en
// silencio a la policy context-preservation, al agente domain-memory y al hook
// UserPromptSubmit. Y el snippet viaja bajo la MISMA key que hoy — renombrarla a
// "snippet" es lo que rompería el hook, que la lee por nombre exacto y ya trunca a 160.
// El campo *_len es lo que le dice al consumidor que hay más y cuánto, para que decida
// el fan-out con domain_mem_get_observation o domain_knowledge_get.

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
