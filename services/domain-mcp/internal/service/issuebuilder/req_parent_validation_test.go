package issuebuilder

import (
	"testing"

	"github.com/stretchr/testify/require"
)

// DOMAINSERV-210: el slot req_parent aceptaba cualquier cosa que empezara con "REQ-",
// pero el commit lo parsea con reReqNumber (^REQ-(\d+)). Un valor como
// "REQ-mcp-tool-contract" pasaba el _answer y reventaba recién en el _commit, con el
// draft ya en status=finished: _answer responde "invalid status for operation", así que
// no hay forma de corregir el campo y hay que abandonar y repetir las 8 preguntas.
//
// El invariante es que las dos validaciones no pueden divergir: todo lo que el wizard
// acepta tiene que ser parseable al materializar.
func TestReqParentSlot_LoQueAceptaElAnswer_LoParseaElCommit(t *testing.T) {
	validar := validadorDelSlotReqParent(t)

	aceptados := []string{
		"REQ-03-memory-system",
		"REQ-54",
		"REQ-161-tool-contract",
	}
	for _, v := range aceptados {
		t.Run("acepta/"+v, func(t *testing.T) {
			require.NoError(t, validar(v), "%q tiene formato REQ-NN válido", v)
			require.NotEmpty(t, reqNumberFromSlug(v),
				"divergencia: el wizard aceptó %q pero el commit no puede extraerle el número", v)
		})
	}

	rechazados := []struct {
		valor  string
		motivo string
	}{
		{"REQ-mcp-tool-contract", "sin número tras REQ-, es el caso que reventó el 2026-07-30"},
		{"REQ-", "prefijo pelado"},
		{"memory-system", "sin prefijo REQ-"},
		{"", "vacío"},
	}
	for _, c := range rechazados {
		t.Run("rechaza/"+c.motivo, func(t *testing.T) {
			require.Error(t, validar(c.valor),
				"el _answer debe rechazar %q (%s), no dejarlo llegar al _commit", c.valor, c.motivo)
		})
	}
}

// Un no-string tiene que fallar antes del regex, no panicear en el type assert.
func TestReqParentSlot_ValorNoString_DevuelveError(t *testing.T) {
	validar := validadorDelSlotReqParent(t)
	require.Error(t, validar(42))
	require.Error(t, validar(nil))
}

// El slot vive en todos los flows que lo declaran, no solo en featureFlow: si otro modo
// lo copia con la validación laxa, el defecto vuelve por la puerta de al lado.
func TestReqParentSlot_TodosLosFlows_UsanLaMismaValidacion(t *testing.T) {
	encontrados := 0
	for modo, flow := range flowsByMode {
		for _, s := range flow {
			if s.Key != "req_parent" {
				continue
			}
			encontrados++
			require.NotNil(t, s.Validate, "el slot req_parent de %s no valida nada", modo)
			require.Error(t, s.Validate("REQ-mcp-tool-contract"),
				"el slot req_parent de %s acepta un valor que el commit no puede parsear", modo)
		}
	}
	require.Positive(t, encontrados, "ningún flow declara req_parent: el test no cubre nada")
}

func validadorDelSlotReqParent(t *testing.T) func(any) error {
	t.Helper()

	for _, s := range featureFlow {
		if s.Key == "req_parent" {
			require.NotNil(t, s.Validate, "el slot req_parent debe declarar Validate")
			return s.Validate
		}
	}
	t.Fatal("featureFlow debe declarar el slot req_parent")
	return nil
}
