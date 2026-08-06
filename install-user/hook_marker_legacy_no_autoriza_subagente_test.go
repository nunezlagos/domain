package main

import (
	"strings"
	"testing"
)

// DOMAINSERV-218, incremento 4. MEDIDO ejecutando el criterio 4 contra el server desplegado
// (2026-08-06, schema_version 289): un subagente real con scope declarado ["openspec/**"] pudo
// escribir en .github/.
//
// La causa NO estaba en el token ni en el guard de solapamiento —los dos funcionan contra prod,
// verificado con las tools— sino acá. El hook no consiguió el token del server y escribió el
// marker v1 LEGACY, que es "<timestamp>\t<flow_run_id>\t<mode>" y no puede llevar scope. Y la
// rama legacy de pre-edit.sh valida el flow con flow_status y hace `exit 0` en cuanto está
// running, SIN MIRAR allowed_paths.
//
// O sea que el camino de DEGRADACIÓN POR DEFECTO anula el aislamiento entero: cualquier fallo
// transitorio pidiendo el token convierte a un subagente scopeado en uno sin restricciones.
//
// Para el HILO PRINCIPAL eso es aceptable y es el comportamiento histórico: nunca tuvo scope, y
// convertirlo en deny volvería el gate insatisfacible en un server sin HMAC — el modo de falla
// de DOMAINSERV-111/175/195. Para un SUBAGENTE no lo es: el aislamiento por agente es
// exactamente lo que el ticket pide.

// ramaLegacyDe devuelve el bloque del hook que valida el marker v1 contra flow_status: desde la
// invocación real de la tool hasta el `elif` que abre la rama v2. Se ancla al comando y no a la
// simple mención del nombre, que también aparece en comentarios.
func ramaLegacyDe(t *testing.T, hook string) string {
	t.Helper()
	i := strings.Index(hook, "domain_call_tool domain_flow_status")
	if i < 0 {
		t.Fatal("no se encontró la invocación de flow_status de la rama legacy")
	}
	resto := hook[i:]
	if fin := strings.Index(resto, "elif ["); fin > 0 {
		return resto[:fin]
	}
	return resto
}

func TestPreEdit_RamaLegacy_NoAutorizaAmpliamenteAUnSubagente(t *testing.T) {
	rama := ramaLegacyDe(t, leerHook(t, "domain-pre-edit.sh"))

	if !strings.Contains(rama, "agent_id") {
		t.Error("la rama legacy no consulta agent_id: un SUBAGENTE cuyo marker degradó a legacy " +
			"obtiene autorización SIN restricción de path. El aislamiento por agente queda anulado " +
			"por el camino de degradación por defecto — medido el 2026-08-06 con un subagente real " +
			"que escribió en .github/ estando scopeado a openspec/**")
	}
}

// El hilo principal NO se toca: convertir su degradación en deny volvería el gate insatisfacible.
func TestPreEdit_RamaLegacy_ElHiloPrincipalConservaSuSalidaAutorizada(t *testing.T) {
	rama := ramaLegacyDe(t, leerHook(t, "domain-pre-edit.sh"))

	if !strings.Contains(rama, "exit 0") {
		t.Error("la rama legacy dejó de autorizar a todos: el hilo principal en un server sin HMAC " +
			"quedaría sin poder editar y el gate se vuelve insatisfacible (DOMAINSERV-111/175/195)")
	}
}
