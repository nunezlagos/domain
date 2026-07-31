// mem_usage_tools.go — DOMAINSERV-145.
//
// domain_mem_used: el cliente reporta, al cerrar el turno, qué memorias tuvo a
// la vista y cuáles usó. Mismo patrón que domain_orchestrate_phase_result — el
// cliente ejecuta y reporta, el server valida y persiste.
//
// POR QUÉ LO REPORTA EL CLIENTE Y NO LO INFIERE EL SERVER: el server solo
// podría adivinar mirando el prompt siguiente, y el propósito de esta señal es
// alimentar el ranking. Un ranking entrenado con adivinanzas es peor que uno
// por relevancia: agrega ruido con apariencia de dato.
//
// ESTADO: invocable desde DOMAINSERV-145. El hook UserPromptSubmit inyecta el id
// completo de cada observación junto a su texto y el prompt_id del turno, que son
// los dos parámetros requeridos. La señal la consume el ranking de SearchHybrid
// como tercera modalidad del RRF: reportar cambia lo que la búsqueda devuelve.
package mcpserver

import (
	"context"

	"github.com/google/uuid"
	"github.com/mark3labs/mcp-go/mcp"
)

func toolMemUsed() mcp.Tool {
	return mcp.NewTool("domain_mem_used",
		mcp.WithDescription("Reporta que memorias inyectadas en este turn te sirvieron. Llamar UNA vez al cerrar el turn. "+
			"candidate_ids = TODAS las memorias que viste (es el denominador: sin el no hay tasa que medir). "+
			"observation_ids = las que realmente influyeron; si ninguna sirvio, mandalo vacio — eso tambien es una "+
			"senal valida y se registra. No reportar NO significa 'no sirvieron': significa que no hay dato. "+
			"Los dos parametros requeridos te los inyecta el hook UserPromptSubmit en el bloque de memorias. "+
			"Si NO estan ahi, no la llames con valores inventados: un UUID adivinado envenena la senal, y no "+
			"reportar es el estado correcto."),
		mcp.WithString("prompt_id",
			mcp.Description("UUID del turn que devolvio domain_prompt_capture. El hook UserPromptSubmit te lo "+
				"inyecta al final del bloque de memorias, en la llamada de ejemplo a domain_mem_used"),
			mcp.Required(),
		),
		mcp.WithArray("candidate_ids",
			mcp.Description("UUIDs de TODAS las observaciones que se te inyectaron en este turn. El hook los "+
				"antepone completos a cada memoria del bloque: son esos, no un prefijo ni un resumen"),
			mcp.Required(),
		),
		mcp.WithArray("observation_ids",
			mcp.Description("UUIDs de las observaciones que efectivamente usaste. Vacio = ninguna sirvio"),
		),
	)
}

func (d *Deps) handleMemUsed(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	if d.Principal == nil {
		return mcp.NewToolResultError("no authenticated principal (set DOMAIN_API_KEY)"), nil
	}
	args := req.GetArguments()

	promptRaw, _ := args["prompt_id"].(string)
	promptID, err := uuid.Parse(promptRaw)
	if err != nil {
		return mcp.NewToolResultError("prompt_id invalido: se espera un UUID"), nil
	}

	candidatos := parseUUIDArray(args["candidate_ids"])
	usadas := parseUUIDArray(args["observation_ids"])
	if len(candidatos) == 0 && len(usadas) == 0 {
		return mcp.NewToolResultError("candidate_ids requerido: sin el no hay denominador para medir"), nil
	}

	recorded, skipped, err := d.Observations.RecordUsage(ctx, promptID, candidatos, usadas)
	if err != nil {
		return mcp.NewToolResultError("no se pudo registrar el consumo: " + err.Error()), nil
	}

	out := map[string]any{"recorded": recorded, "skipped": skipped}
	// Un prompt_id desconocido es esperable —el cliente reporta al cerrar y
	// puede equivocarse— así que se informa con status OK en vez de abortar la
	// tx del handler. Un reporte perdido no justifica romperle el turno.
	if recorded == 0 && skipped > 0 {
		out["reason"] = "prompt desconocido o sin observaciones vigentes; no se registro nada"
	}
	return toolResultJSON(out)
}

// parseUUIDArray descarta lo que no sea un UUID valido en vez de fallar: el
// reporte llega al final del turno y perder TODO por un id mal formado seria
// peor que perder ese id. El servicio se encarga del dedup y del cap.
func parseUUIDArray(raw any) []uuid.UUID {
	items, ok := raw.([]any)
	if !ok {
		return nil
	}
	out := make([]uuid.UUID, 0, len(items))
	for _, it := range items {
		s, ok := it.(string)
		if !ok {
			continue
		}
		if id, err := uuid.Parse(s); err == nil {
			out = append(out, id)
		}
	}
	return out
}
