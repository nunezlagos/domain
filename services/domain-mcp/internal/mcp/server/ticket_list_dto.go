package mcpserver

import (
	ticketsvc "nunezlagos/domain/internal/service/ticket"
)

// campoOmitido tapa por shadowing un campo del struct embebido: como es un puntero
// siempre nil, omitempty lo elimina del JSON. Gana sobre el campo embebido porque
// está a menor profundidad.
type campoOmitido *struct{}

// ticketListItem es un ticket tal como lo devuelve domain_ticket_list: hereda todo
// ticketsvc.Ticket menos el body. Las descripciones de este repo son documentos de
// varios KB y un listado de 5 tickets medía 10.889 tokens reales, casi todos body
// (DOMAINSERV-177). Quien necesite el texto lo pide con domain_ticket_get, patrón que
// el agente ticket-triage ya seguía antes de que el server lo respetara.
type ticketListItem struct {
	ticketsvc.Ticket
	DescriptionMD  campoOmitido `json:"description_md,omitempty"`
	DescriptionLen int          `json:"description_len"`
}

func proyectarTicketsParaListado(list []*ticketsvc.Ticket) []ticketListItem {
	items := make([]ticketListItem, 0, len(list))
	for _, t := range list {
		if t == nil {
			continue
		}
		items = append(items, ticketListItem{Ticket: *t, DescriptionLen: len(t.DescriptionMD)})
	}
	return items
}
