//go:build integration

package mcpserver_test

import (
	"context"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"

	mcpserver "nunezlagos/domain/internal/mcp/server"
)

// DOMAINSERV-188: el estado de sesión vivía en la tabla projects —una fila por
// proyecto— así que dos personas trabajando el mismo proyecto se pisaban el
// last_known_head. A hacía bootstrap y guardaba su HEAD; B abría sesión y recibía el
// HEAD de A como su last_known, de modo que el `git log last_known..current` que el
// protocolo manda correr le mostraba los commits de A como novedades propias. Lo mismo
// con la rama y el cwd.
//
// Nada de esto fallaba con un error: cada usuario recibía un contexto de arranque que
// describía la sesión de otro. Por eso el test es el centro del ticket — sin él la
// regresión vuelve sin que nada avise.

// Dos usuarios, un proyecto: cada uno conserva SU puntero. Es el invariante del ticket.
func TestSessionState_DosUsuariosMismoProyecto_NoSePisanElHead(t *testing.T) {
	f := setupMCP(t)
	defer f.cleanup()
	ctx := context.Background()

	projID := f.projectID
	usuarioA := f.userID
	usuarioB := crearUsuario(t, ctx, f, "b@acme.com", "Usuario B")

	require.NoError(t, mcpserver.BumpUserProjectState(ctx, f.pool, usuarioA, projID,
		"aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa", "main", "/home/a/proyecto"))
	require.NoError(t, mcpserver.BumpUserProjectState(ctx, f.pool, usuarioB, projID,
		"bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb", "feature/x", "/home/b/proyecto"))

	estadoA, err := mcpserver.ReadUserProjectState(ctx, f.pool, usuarioA, projID)
	require.NoError(t, err)
	estadoB, err := mcpserver.ReadUserProjectState(ctx, f.pool, usuarioB, projID)
	require.NoError(t, err)

	require.Equal(t, "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa", estadoA.LastKnownHead,
		"el usuario A recibió un head que no es el suyo")
	require.Equal(t, "bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb", estadoB.LastKnownHead,
		"el usuario B recibió un head que no es el suyo")
	require.Equal(t, "main", estadoA.LastSeenBranch)
	require.Equal(t, "feature/x", estadoB.LastSeenBranch,
		"la rama de B quedó sobreescrita por la de A")
	require.Equal(t, "/home/a/proyecto", estadoA.LastSeenCwd)
	require.Equal(t, "/home/b/proyecto", estadoB.LastSeenCwd)
}

// Un usuario que entra por primera vez a un proyecto con historia ajena arranca con el
// puntero VACÍO: se lo trata como primera vez para el rango de git, en vez de mostrarle
// los commits de otro como novedades suyas. Decisión deliberada y asimétrica respecto
// del CONTENIDO del proyecto, que sí es del equipo y sigue compartido.
func TestSessionState_UsuarioNuevoEnProyectoAjeno_ArrancaSinHead(t *testing.T) {
	f := setupMCP(t)
	defer f.cleanup()
	ctx := context.Background()

	require.NoError(t, mcpserver.BumpUserProjectState(ctx, f.pool, f.userID, f.projectID,
		"aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa", "main", "/home/a/proyecto"))

	nuevo := crearUsuario(t, ctx, f, "nuevo@acme.com", "Recien Llegado")
	estado, err := mcpserver.ReadUserProjectState(ctx, f.pool, nuevo, f.projectID)
	require.NoError(t, err)

	require.Empty(t, estado.LastKnownHead,
		"el usuario nuevo heredó el head de otro: el git log de rango le va a mostrar commits ajenos")
	require.Empty(t, estado.LastSeenBranch)
}

// El UPSERT conserva el patrón COALESCE(NULLIF(...)) del código original: un bootstrap
// que llega sin git_head —por ejemplo desde un directorio que no es repo— no puede
// borrar el puntero que el usuario ya tenía.
func TestSessionState_BumpConValorVacio_NoBorraElEstadoPrevio(t *testing.T) {
	f := setupMCP(t)
	defer f.cleanup()
	ctx := context.Background()

	require.NoError(t, mcpserver.BumpUserProjectState(ctx, f.pool, f.userID, f.projectID,
		"aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa", "main", "/home/a/proyecto"))
	require.NoError(t, mcpserver.BumpUserProjectState(ctx, f.pool, f.userID, f.projectID,
		"", "", ""))

	estado, err := mcpserver.ReadUserProjectState(ctx, f.pool, f.userID, f.projectID)
	require.NoError(t, err)
	require.Equal(t, "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa", estado.LastKnownHead,
		"un bump vacío borró el head que el usuario ya tenía")
	require.Equal(t, "main", estado.LastSeenBranch)
}

// El estado es por (user_id, project_id): el mismo usuario en dos proyectos distintos
// lleva dos punteros. Sin esta parte de la clave, cambiar de repo pisaría el estado del
// anterior — que es el bug de este ticket con los roles invertidos.
func TestSessionState_MismoUsuarioDosProyectos_LlevaUnPunteroPorProyecto(t *testing.T) {
	f := setupMCP(t)
	defer f.cleanup()
	ctx := context.Background()

	otroProyecto := crearProyecto(t, ctx, f, "otro-proyecto")

	require.NoError(t, mcpserver.BumpUserProjectState(ctx, f.pool, f.userID, f.projectID,
		"aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa", "main", "/home/a/uno"))
	require.NoError(t, mcpserver.BumpUserProjectState(ctx, f.pool, f.userID, otroProyecto,
		"cccccccccccccccccccccccccccccccccccccccc", "develop", "/home/a/dos"))

	uno, err := mcpserver.ReadUserProjectState(ctx, f.pool, f.userID, f.projectID)
	require.NoError(t, err)
	dos, err := mcpserver.ReadUserProjectState(ctx, f.pool, f.userID, otroProyecto)
	require.NoError(t, err)

	require.Equal(t, "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa", uno.LastKnownHead)
	require.Equal(t, "cccccccccccccccccccccccccccccccccccccccc", dos.LastKnownHead)
}

func crearUsuario(t *testing.T, ctx context.Context, f *mcpFixture, email, nombre string) uuid.UUID {
	t.Helper()
	var id uuid.UUID
	require.NoError(t, f.pool.QueryRow(ctx,
		`INSERT INTO users (email, name, role) VALUES ($1, $2, 'member') RETURNING id`,
		email, nombre,
	).Scan(&id))
	return id
}

func crearProyecto(t *testing.T, ctx context.Context, f *mcpFixture, slug string) uuid.UUID {
	t.Helper()
	var id uuid.UUID
	require.NoError(t, f.pool.QueryRow(ctx,
		`INSERT INTO projects (name, slug) VALUES ($1, $2) RETURNING id`,
		slug, slug,
	).Scan(&id))
	return id
}
