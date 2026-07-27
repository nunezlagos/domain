package mcpserver

import (
	ticketsvc "nunezlagos/domain/internal/service/ticket"
)

// campoOmitido tapa por shadowing un campo del struct embebido: como es un puntero
// siempre nil, omitempty lo elimina del JSON. Gana sobre el campo embebido porque
// está a menor profundidad.
type campoOmitido *struct{}

// ticketSlim es un ticket sin el body: hereda todo ticketsvc.Ticket menos la
// descripción. Cubre los dos caminos donde el texto no aporta — el listado y las 8
// tools que mutan estado. Las descripciones de este repo son documentos de varios KB:
// un listado de 5 tickets medía 10.889 tokens reales (DOMAINSERV-177) y un solo
// change_status devolvía 5.221 chars para confirmar una transición de 3 campos
// (DOMAINSERV-178). Quien necesite el texto lo pide con domain_ticket_get.
// El shadowing depende de que el campo siga llamándose DescriptionMD: si se renombra
// en ticketsvc.Ticket, el body vuelve al JSON sin error de compilación.
type ticketSlim struct {
	ticketsvc.Ticket
	DescriptionMD  campoOmitido `json:"description_md,omitempty"`
	DescriptionLen int          `json:"description_len"`
}

// nil devuelve zero value en vez de puntero: los handlers ya cortaron por error antes,
// y un *ticketSlim nil serializaría "null"
func proyectarTicketSlim(t *ticketsvc.Ticket) ticketSlim {
	if t == nil {
		return ticketSlim{}
	}
	return ticketSlim{Ticket: *t, DescriptionLen: len(t.DescriptionMD)}
}

func proyectarTicketsParaListado(list []*ticketsvc.Ticket) []ticketSlim {
	items := make([]ticketSlim, 0, len(list))
	for _, t := range list {
		if t == nil {
			continue
		}
		items = append(items, proyectarTicketSlim(t))
	}
	return items
}
