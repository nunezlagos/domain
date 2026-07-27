package seeds

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"

	"nunezlagos/domain/internal/llm/registry"
)

// DOMAINSERV-162: todo modelo que un agent_template declara tiene que estar
// cotizado en el registry.
//
// Es la mitad accionable del drift. El precio real de un modelo cambia afuera y
// ningún test local puede saberlo — para eso está la skill claude-api y la
// policy validate-with-sources-context7. Pero un modelo DECLARADO y no cotizado
// sí es detectable acá, y es el caso que produce el daño silencioso: el runner
// descarta ErrModelNotFound y la corrida se persiste con costo 0, indistinguible
// de una corrida gratis.
//
// El test cruza dos catálogos que hoy nadie obliga a coincidir: el de templates
// (internal/seeds) y el de pricing (internal/llm/registry). Sin esto, agregar
// una fase SDD con un modelo nuevo pasa CI y deja su costo en cero.
func TestAgentTemplateCatalog_TodoModeloDeclarado_EstaCotizadoEnElRegistry(t *testing.T) {
	reg := registry.New()
	ctx := context.Background()

	templates := AgentTemplateCatalog()
	require.NotEmpty(t, templates, "el catálogo vino vacío: el test no estaría cubriendo nada")

	for _, tpl := range templates {
		require.NotEmpty(t, tpl.Model, "%s no declara model: heredaría el default de la tabla", tpl.Slug)

		_, err := reg.Get(ctx, "anthropic", tpl.Model)
		require.NoError(t, err,
			"la fase %s declara %q, que no está en el registry de pricing: sus corridas se registrarían con costo 0",
			tpl.Slug, tpl.Model)
	}
}
