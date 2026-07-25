package systemcron

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

// DOMAINSERV-153: la agregación por país es la única lógica no trivial del
// colector, y es pura a propósito — se testea sin red ni LAPI.
func TestCountByCountry_VariasAlertasDelMismoPais_SumaElConteo(t *testing.T) {
	got := countByCountry([]crowdsecAlert{
		{Source: alertSource{CN: "RU"}},
		{Source: alertSource{CN: "RU"}},
		{Source: alertSource{CN: "US"}},
	})

	assert.Equal(t, map[string]int{"RU": 2, "US": 1}, got)
}

func TestCountByCountry_SinAlertas_DevuelveMapaVacio(t *testing.T) {
	assert.Empty(t, countByCountry(nil))
	assert.Empty(t, countByCountry([]crowdsecAlert{}))
}

// CrowdSec no siempre resuelve el país: las IPs privadas y algunos rangos
// vienen con cn vacío. Descartarlas silenciosamente perdería ataques del
// total, así que se agrupan bajo una etiqueta explícita.
func TestCountByCountry_PaisVacio_SeAgrupaComoDesconocido(t *testing.T) {
	got := countByCountry([]crowdsecAlert{
		{Source: alertSource{CN: ""}},
		{Source: alertSource{CN: "  "}},
		{Source: alertSource{CN: "CL"}},
	})

	assert.Equal(t, 2, got[countryUnknown], "las dos sin país resoluble van a la etiqueta desconocida")
	assert.Equal(t, 1, got["CL"])
}

// Sabotaje del invariante de cardinalidad: el conteo agrupa SOLO por país.
// Si alguien agregara la IP a la clave, la cardinalidad de la métrica
// explotaría y violaría low-cardinality-metrics — dos alertas del mismo país
// desde IPs distintas tienen que seguir colapsando en una sola entrada.
func TestCountByCountry_MismoPaisDistintaIP_NoAbreUnaClavePorIP(t *testing.T) {
	got := countByCountry([]crowdsecAlert{
		{Source: alertSource{CN: "CN", IP: "1.2.3.4", ASName: "ISP Uno"}},
		{Source: alertSource{CN: "CN", IP: "5.6.7.8", ASName: "ISP Dos"}},
	})

	assert.Len(t, got, 1, "la IP y el AS NO pueden abrir claves nuevas: reventarían la cardinalidad")
	assert.Equal(t, 2, got["CN"])
}
