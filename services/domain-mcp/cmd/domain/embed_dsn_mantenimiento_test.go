package main

import (
	"os"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

// DOMAINSERV-185: los comandos de mantenimiento de embeddings (reindex y backfill) recorren la
// instancia ENTERA por diseño, así que no tienen un app.current_project_id que setear. Desde el
// RLS de la 000287, knowledge_chunks —uno de los tres targets del backfill— solo es visible con
// ese GUC. Con el rol de la app el SELECT de pendientes devuelve CERO SIN ERROR y el comando
// reporta éxito sin repoblar nada.
//
// Ese modo de falla ya ocurrió acá por otra causa: el deploy del 2026-07-24 dejó 0 de 2065
// observaciones con embedding porque el embedder había degradado a noop y el backfill salió
// bien igual. Lo que lo hace peligroso es que es indistinguible de "no había nada pendiente",
// que es exactamente lo que devuelve hoy en prod (0 de 157 chunks sin embedding). O sea: si esto
// se rompe, se rompe callado y se descubre tarde.

func TestResolverDSNMantenimiento_PrefiereElRolQueNoEstaSujetoARLS(t *testing.T) {
	// las cuatro seteadas: gana la de admin
	t.Setenv("DOMAIN_DATABASE_ADMIN_URL", "postgres://admin/x")
	t.Setenv("DOMAIN_DATABASE_AUTH_URL", "postgres://auth/x")
	t.Setenv("DOMAIN_DATABASE_URL", "postgres://app/x")
	t.Setenv("DATABASE_URL", "postgres://legacy/x")

	dsn, origen := resolverDSNMantenimiento()
	require.Equal(t, "postgres://admin/x", dsn)
	require.Equal(t, "DOMAIN_DATABASE_ADMIN_URL", origen,
		"con el DSN de admin disponible, el mantenimiento NO debe caer al rol de la app")
}

func TestResolverDSNMantenimiento_SinAdmin_CaeAlDeAuthYNoAlDeLaApp(t *testing.T) {
	os.Unsetenv("DOMAIN_DATABASE_ADMIN_URL")
	t.Setenv("DOMAIN_DATABASE_AUTH_URL", "postgres://auth/x")
	t.Setenv("DOMAIN_DATABASE_URL", "postgres://app/x")

	dsn, origen := resolverDSNMantenimiento()
	require.Equal(t, "postgres://auth/x", dsn)
	require.Equal(t, "DOMAIN_DATABASE_AUTH_URL", origen,
		"app_admin tiene BYPASSRLS y es el que el deploy ya tiene seteado: es el fallback correcto")
}

// El fallback al rol de la app se CONSERVA a propósito: en dev-local puede no haber otro DSN, y
// el backfill sigue sirviendo para las dos tablas que no están bajo RLS. Lo que no se conserva
// es el silencio — quien llama tiene que poder advertirlo, y para eso se devuelve el origen.
func TestResolverDSNMantenimiento_SoloElDeLaApp_LoDevuelvePeroDelataElOrigen(t *testing.T) {
	os.Unsetenv("DOMAIN_DATABASE_ADMIN_URL")
	os.Unsetenv("DOMAIN_DATABASE_AUTH_URL")
	t.Setenv("DOMAIN_DATABASE_URL", "postgres://app/x")

	dsn, origen := resolverDSNMantenimiento()
	require.Equal(t, "postgres://app/x", dsn)
	require.Equal(t, "DOMAIN_DATABASE_URL", origen,
		"el origen es lo que permite advertir que knowledge_chunks va a verse vacío")
}

func TestResolverDSNMantenimiento_SinNinguna_NoInventaUnDSN(t *testing.T) {
	for _, k := range dsnCandidatosMantenimiento {
		os.Unsetenv(k)
	}
	dsn, origen := resolverDSNMantenimiento()
	require.Empty(t, dsn)
	require.Empty(t, origen)
}

// GUARD DE FUENTE contra la reincidencia, que es el defecto que este ticket encontró: el
// backfill leía os.Getenv("DOMAIN_DATABASE_URL") DIRECTO mientras reindex usaba el orden con
// admin primero. Los dos comandos hacen mantenimiento global y tenían resoluciones distintas;
// el que quedó atrás fallaba en silencio. Si alguien vuelve a leer la variable de la app a mano
// en cualquiera de los dos, esto falla.
func TestEmbedCommands_NingunoLeeElDSNDeLaAppAMano(t *testing.T) {
	for _, archivo := range []string{"embed_backfill.go", "embed_reindex.go"} {
		b, err := os.ReadFile(archivo)
		require.NoError(t, err, "leer %s", archivo)
		src := string(b)

		require.NotContains(t, src, `os.Getenv("DOMAIN_DATABASE_URL")`,
			"%s lee el DSN del rol de la app a mano y se saltea el orden compartido: bajo RLS "+
				"knowledge_chunks se ve vacío y el comando reporta éxito sin hacer nada", archivo)
		require.NotContains(t, src, `os.Getenv("DATABASE_URL")`,
			"%s lee DATABASE_URL a mano, salteando dsnCandidatosMantenimiento", archivo)
	}
}

// Y la contra-prueba del guard de arriba: que la lista compartida siga teniendo al admin ANTES
// que a la app. Sin este orden el guard de fuente pasaría con la lista invertida, que es el bug
// con otra forma.
func TestDsnCandidatosMantenimiento_ElAdminVaAntesQueElRolDeLaApp(t *testing.T) {
	pos := map[string]int{}
	for i, k := range dsnCandidatosMantenimiento {
		pos[k] = i
	}
	require.Contains(t, pos, "DOMAIN_DATABASE_ADMIN_URL")
	require.Contains(t, pos, "DOMAIN_DATABASE_URL")
	require.Less(t, pos["DOMAIN_DATABASE_ADMIN_URL"], pos["DOMAIN_DATABASE_URL"],
		"el rol con BYPASSRLS tiene que ganarle al de la app, o el mantenimiento vuelve a "+
			"correr sujeto al RLS que no puede satisfacer")
	require.Less(t, pos["DOMAIN_DATABASE_AUTH_URL"], pos["DOMAIN_DATABASE_URL"],
		"app_admin es el que el deploy ya tiene seteado: si queda después del de la app, el "+
			"fix no cambia nada en producción")
	require.True(t, strings.HasPrefix(dsnCandidatosMantenimiento[0], "DOMAIN_DATABASE_ADMIN"),
		"el primero de la lista es el DSN del dueño del schema")
}
