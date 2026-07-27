package mcpserver

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"

	ticketsvc "nunezlagos/domain/internal/service/ticket"
)

func TestProyectarTicketsParaListado_DescripcionLarga_OmiteBodyYExponeLongitud(t *testing.T) {
	body := strings.Repeat("x", 5000)
	tk := &ticketsvc.Ticket{
		ID:            uuid.New(),
		ProjectID:     uuid.New(),
		Key:           "DOMAINSERV-1",
		DisplayKey:    "DOMAINSERV-1",
		Title:         "un ticket con descripcion larga",
		DescriptionMD: body,
		Status:        "backlog",
		Priority:      "high",
	}

	items := proyectarTicketsParaListado([]*ticketsvc.Ticket{tk})
	require.Len(t, items, 1)

	raw, err := json.Marshal(items[0])
	require.NoError(t, err)
	s := string(raw)

	// el body no viaja: son 10.889 tokens reales medidos en un listado de 5 (DOMAINSERV-177)
	require.NotContains(t, s, "description_md")
	require.NotContains(t, s, strings.Repeat("x", 100))
	// su tamaño sí, para que el consumidor decida si pedir el ticket completo con _get
	require.Contains(t, s, `"description_len":5000`)
	// id y project_id son Required en 8 tools: el encadenamiento list->get no se rompe
	require.Contains(t, s, tk.ID.String())
	require.Contains(t, s, tk.ProjectID.String())
	require.Contains(t, s, "DOMAINSERV-1")
}

func TestProyectarTicketsParaListado_SinDescripcion_LongitudCero(t *testing.T) {
	tk := &ticketsvc.Ticket{ID: uuid.New(), ProjectID: uuid.New(), Key: "X-1"}

	items := proyectarTicketsParaListado([]*ticketsvc.Ticket{tk})

	require.Len(t, items, 1)
	require.Equal(t, 0, items[0].DescriptionLen)
}

func TestProyectarTicketsParaListado_ListaVacia_DevuelveVacio(t *testing.T) {
	require.Empty(t, proyectarTicketsParaListado(nil))
}
