package main

import (
	"strings"
	"testing"
)

// DOMAINSERV-158: el contrato de retorno de los agentes era costumbre, no contrato. La REGLA
// vive en la BD —policy `delegar-lecturas-multiples`: "El retorno debe distinguir vacío real,
// degradación y truncamiento"— pero el TEXTO se copia a mano en cada template, y nada lo
// verificaba: los chequeos de frontmatter de agent_templates_test.go están atados al slug
// `domain-memory`, así que el resto del catálogo no se miraba.
//
// Que hacía falta no es hipotético: domain-memory es el único template escrito ANTES del
// contrato y era el único que no lo tenía, dos rondas de trabajo después. Copiar a mano sin
// guard ya falló una vez.
//
// Este guard NO inventa doctrina: exige lo que la policy manda. Deliberadamente NO exige
// "cubrí N de M", que hoy tienen 3 de 5 templates y no está en la policy — un guard que
// obliga a editar archivos que nadie decidió tocar se desactiva en vez de arreglarse.

// Los tres estados que la policy exige distinguir. Se busca el MARCADOR con el que el retorno
// los declara, no la palabra suelta: "truncamiento" en una oración de prosa no le sirve a
// quien lee el retorno; `truncado:` sí.
var marcadoresDeEstado = []struct {
	nombre string
	// alternativas: cualquiera alcanza. El acento de "vacío" no se puede dar por seguro.
	alternativas []string
}{
	{"vacío real", []string{"vacío:", "vacio:"}},
	{"degradación declarada", []string{"degradado:"}},
	{"truncamiento declarado", []string{"truncado:"}},
}

func TestAgentContract_TodoAgenteDeclaraLosTresEstadosDelRetorno(t *testing.T) {
	cat, err := agentCatalog()
	if err != nil {
		t.Fatalf("agentCatalog: %v", err)
	}

	for _, a := range cat {
		cuerpo := string(a.claude)
		for _, m := range marcadoresDeEstado {
			if !contieneAlguno(cuerpo, m.alternativas) {
				t.Errorf(
					"%s no declara %s: falta alguno de %v en el template.\n"+
						"La policy delegar-lecturas-multiples lo exige: \"El retorno debe distinguir "+
						"vacío real, degradación y truncamiento\". Sin el marcador, quien lee el retorno "+
						"no puede distinguir \"busqué y no hay\" de \"no pude buscar\".",
					a.slug, m.nombre, m.alternativas,
				)
			}
		}
	}
}

// La sección es donde los tres estados se declaran. Un template que los explica en prosa pero
// no reserva el lugar donde van deja al agente sin dónde ponerlos.
func TestAgentContract_TodoAgenteReservaLaSeccionNota(t *testing.T) {
	cat, err := agentCatalog()
	if err != nil {
		t.Fatalf("agentCatalog: %v", err)
	}

	for _, a := range cat {
		if !strings.Contains(string(a.claude), "## Nota") {
			t.Errorf(
				"%s no tiene la sección \"## Nota\" en su formato de retorno: es el lugar "+
					"donde se declaran vacío/degradado/truncado", a.slug)
		}
	}
}

// El techo de tamaño es lo que hace que delegar ahorre contexto. Un agente sin tope devuelve
// lo que quiera y el ahorro se evapora — es el 14:1 del que vive la delegación.
func TestAgentContract_TodoAgenteDeclaraUnTechoDeTamano(t *testing.T) {
	cat, err := agentCatalog()
	if err != nil {
		t.Fatalf("agentCatalog: %v", err)
	}

	for _, a := range cat {
		if !strings.Contains(string(a.claude), "400 palabras") {
			t.Errorf("%s no declara el techo de 400 palabras del retorno", a.slug)
		}
	}
}

// El `description` es el mecanismo de ruteo: es lo que el orquestador lee para decidir a quién
// delegar. Uno que solo dice para qué sirve invita a usarlo de más; el "cuándo NO" es la mitad
// que evita el spawn inútil, y el spawn tiene piso medido (~20.800 tokens por lote grande,
// project_policy modelo-por-clase-de-tarea).
func TestAgentContract_TodoDescriptionDiceCuandoNoUsarlo(t *testing.T) {
	cat, err := agentCatalog()
	if err != nil {
		t.Fatalf("agentCatalog: %v", err)
	}

	for _, a := range cat {
		desc := descripcionDeAgente(string(a.claude))
		if desc == "" {
			t.Errorf("%s no tiene campo description en el frontmatter", a.slug)
			continue
		}
		if !contieneAlguno(desc, []string{"No utilizar para", "No usar para"}) {
			t.Errorf(
				"%s: su description no dice cuándo NO usarlo. Es lo que el orquestador lee "+
					"para rutear, y sin la cláusula negativa se delega de más.\ndescription: %q",
				a.slug, desc)
		}
	}
}

func contieneAlguno(texto string, agujas []string) bool {
	for _, aguja := range agujas {
		if strings.Contains(texto, aguja) {
			return true
		}
	}
	return false
}

// descripcionDeAgente extrae solo el valor de `description:` del frontmatter. Acotar la ventana
// es deliberado: buscar la cláusula negativa en el archivo entero daría verde por una mención
// en el cuerpo, que es el falso positivo por subcadena que ya se cometió tres veces en este
// repo (DOMAINSERV-190, DOMAINSERV-182 y el primer guard de DOMAINSERV-205).
func descripcionDeAgente(template string) string {
	const marca = "\ndescription:"
	i := strings.Index(template, marca)
	if i == -1 {
		return ""
	}
	resto := template[i+len(marca):]
	// el valor termina en el próximo campo del frontmatter o en el cierre `---`; description
	// es de una sola línea en todos los templates del catálogo
	if fin := strings.IndexByte(resto, '\n'); fin != -1 {
		return strings.TrimSpace(resto[:fin])
	}
	return strings.TrimSpace(resto)
}
