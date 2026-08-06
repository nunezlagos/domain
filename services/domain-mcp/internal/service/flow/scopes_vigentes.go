package flow

import "fmt"

// ScopeVigente es el territorio que un agente tiene reservado en un flow: lo que la tabla
// flow_agent_scopes devuelve para las filas no vencidas ni revocadas.
type ScopeVigente struct {
	AgentID      string
	AllowedPaths []string
}

// SolapamientoConOtros es el caller que le faltaba a ValidarParticionDisjunta (DOMAINSERV-218,
// criterio 3). La función existía desde el incremento 1 con sus tests, pero nadie la invocaba
// porque un token aislado no puede responder "qué scopes hay vigentes en este flow": esa
// respuesta es la que trae `vigentes`.
//
// EXCLUYE AL PROPIO SOLICITANTE, y ese detalle es el que hace la diferencia entre un guard útil
// y uno que se dispara solo. Cada cierre de fase re-emite el token, así que el agente pide su
// mismo scope una y otra vez; comparándolo contra su propia fila anterior se bloquearía a sí
// mismo en la segunda fase de cualquier flow real.
//
// Una allowlist vacía —de cualquiera de los dos lados— no reserva territorio: es un flow normal
// y no batch-mode. Tratarla como conflicto dejaría sin token a todo flow que no declare scopes.
func SolapamientoConOtros(solicitante string, nueva []string, vigentes []ScopeVigente) error {
	if len(nueva) == 0 {
		return nil
	}
	if err := ValidarAllowlist(nueva); err != nil {
		return err
	}

	for _, v := range vigentes {
		if v.AgentID == solicitante || len(v.AllowedPaths) == 0 {
			continue
		}
		// se delega en ValidarParticionDisjunta en vez de comparar acá: duplicar el criterio de
		// solapamiento haría que un arreglo en uno de los dos no llegue al otro
		if err := ValidarParticionDisjunta([][]string{nueva, v.AllowedPaths}); err != nil {
			return fmt.Errorf("%w (contra el agente %q)", err, v.AgentID)
		}
	}
	return nil
}
