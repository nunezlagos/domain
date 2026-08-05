package mcpserver

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

// DOMAINSERV-220: de 4 tickets abordados el 2026-07-31, 3 tenían una afirmación factual falsa
// presentada con el mismo tono que las mediciones, y una cambió la CLASE de esfuerzo estimada.
// El ticket pide dos cosas de la plantilla: que distinga lo medido de lo inferido, y que
// registre contra qué HEAD se verificó. Las dos viven en la definición de la tool, así que
// estos son guards de contrato: si alguien las borra, esto falla.

func TestTicketCreate_DescripcionExigeSepararMedidoDeInferido(t *testing.T) {
	tool := toolTicketCreate()

	desc := ""
	for nombre, prop := range tool.InputSchema.Properties {
		if nombre != "description_md" {
			continue
		}
		m, ok := prop.(map[string]any)
		require.True(t, ok, "la propiedad description_md tiene que ser un objeto de schema")
		desc, _ = m["description"].(string)
	}
	require.NotEmpty(t, desc, "domain_ticket_create dejó de declarar description_md")

	require.Contains(t, desc, "## Medido",
		"la plantilla dejó de pedir la sección de lo MEDIDO: sin ella una inferencia y una "+
			"medición se escriben igual, que es la causa raíz de DOMAINSERV-220")
	require.Contains(t, desc, "## Hipotesis",
		"la plantilla dejó de pedir la sección de HIPÓTESIS: quien retome el ticket no sabe "+
			"qué re-verificar y qué no")
}

func TestTicketCreate_DeclaraElParametroDeHeadVerificado(t *testing.T) {
	tool := toolTicketCreate()

	_, existe := tool.InputSchema.Properties["verificado_contra_head"]
	require.True(t, existe,
		"domain_ticket_create dejó de aceptar verificado_contra_head: un ticket sin el HEAD "+
			"contra el que se midió no se puede declarar sospechoso sin leerlo entero")
}

// headCorto normaliza a 7 para que el label sea legible. Los dos casos que importan son el
// SHA completo (lo que devuelve git rev-parse HEAD) y uno más corto que 7, donde truncar
// produciría un label que parece un SHA sin serlo.
func TestHeadCorto_NormalizaA7YNoInventaSHAs(t *testing.T) {
	casos := []struct {
		nombre string
		entrada string
		espera string
	}{
		{"sha completo de 40 se corta a 7", "aae45a85f99bb88ee47f1ea2a834cb15ca512049", "aae45a8"},
		{"sha ya corto queda igual", "aae45a8", "aae45a8"},
		{"más corto que 7 NO se rellena ni se trunca", "abc12", "abc12"},
		{"espacios alrededor se limpian", "  aae45a85f99bb  ", "aae45a8"},
		{"vacío queda vacío", "", ""},
	}
	for _, c := range casos {
		t.Run(c.nombre, func(t *testing.T) {
			require.Equal(t, c.espera, headCorto(c.entrada))
		})
	}
}

// El label lo arma el server y no quien llama, para que el filtro de domain_ticket_list sirva:
// dos formatos distintos del mismo dato son dos datos que no se pueden cruzar.
func TestHeadCorto_ElPrefijoDelLabelEsUnoSolo(t *testing.T) {
	label := "head-" + headCorto("aae45a85f99bb88ee47f1ea2a834cb15ca512049")
	require.Equal(t, "head-aae45a8", label)
	require.True(t, strings.HasPrefix(label, "head-"),
		"el prefijo es parte del contrato: domain_ticket_list(label=...) filtra por él")
}
