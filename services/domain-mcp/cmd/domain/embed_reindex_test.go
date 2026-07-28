package main

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

// DOMAINSERV-163: un UPDATE masivo de embeddings no re-entrena los centroides de un
// ivfflat, así que después del backfill el índice sigue particionando el espacio
// según una distribución que ya no existe. La consecuencia es de recall, no de
// corrección: las búsquedas devuelven resultados, pero el escaneo va a listas que no
// corresponden y se pierden vecinos relevantes. Es difícil de notar justamente porque
// algo siempre vuelve. Hasta este commit el REINDEX solo existía como una línea de
// prosa en el CHANGELOG, y un paso manual que solo vive en un changelog no se ejecuta.

func TestParseReindexArgs_SinArgumentos_UsaDefaults(t *testing.T) {
	o := parseReindexArgs(nil)
	require.True(t, o.concurrently, "CONCURRENTLY es el default porque la policy migration-safety lo exige")
	require.False(t, o.dryRun)
	require.False(t, o.all)
	require.Empty(t, o.dsn)
}

func TestParseReindexArgs_Flags_SeParsean(t *testing.T) {
	o := parseReindexArgs([]string{"--dry-run", "--no-concurrently", "--all"})
	require.True(t, o.dryRun)
	require.False(t, o.concurrently)
	require.True(t, o.all)
}

func TestParseReindexArgs_DSN_SeParsea(t *testing.T) {
	o := parseReindexArgs([]string{"--dsn=postgres://domain:x@postgres:5432/domain"})
	require.Equal(t, "postgres://domain:x@postgres:5432/domain", o.dsn)
}

// El universo por default son las tablas que el backfill invalida. Si mañana entra
// una 4a tabla a backfillTargets y nadie suma su tabla acá, el REINDEX queda
// incompleto EN SILENCIO: es exactamente el modo de falla que dejó a skills fuera del
// backfill en DOMAINSERV-157.
func TestReindexTables_SinAll_SeCorrespondeUnoAUnoConBackfillTargets(t *testing.T) {
	reindex := reindexTables(false)
	var backfill []string
	for _, tg := range backfillTargets() {
		backfill = append(backfill, tg.table)
	}
	require.ElementsMatch(t, backfill, reindex,
		"las tablas a reindexar tienen que ser las que el backfill toca: un backfill que escribe invalida su índice")
}

// chat_document_embeddings queda fuera del default a propósito: su columna es NOT
// NULL y la puebla el RAG del chat de domain-admin, así que un backfill nunca la
// invalida y "REINDEX post-backfill" no le aplica. Con --all se incluye igual, porque
// ahí el pedido es reindexar todo el esquema.
func TestReindexTables_ConAll_SumaLasTablasFueraDelBackfill(t *testing.T) {
	conAll := reindexTables(true)
	require.Subset(t, conAll, reindexTables(false), "--all no puede perder ninguna tabla del default")
	require.Contains(t, conAll, "chat_document_embeddings")
	require.Contains(t, conAll, "llm_semantic_cache",
		"llm_semantic_cache tiene ivfflat (migración 000067) y faltaba del inventario del ticket")
}

func TestBuildReindexStmt_ConConcurrently_LoIncluye(t *testing.T) {
	stmt := buildReindexStmt("knowledge_observations_embedding_idx", true)
	require.Contains(t, stmt, "CONCURRENTLY")
	require.Contains(t, stmt, "knowledge_observations_embedding_idx")
}

func TestBuildReindexStmt_SinConcurrently_NoLoIncluye(t *testing.T) {
	stmt := buildReindexStmt("skills_embedding_idx", false)
	require.NotContains(t, stmt, "CONCURRENTLY")
	require.Contains(t, stmt, "skills_embedding_idx")
}

// Los nombres NO se hardcodean: se descubren de pg_indexes, igual que hace la
// migración 000275. La 000155 ya renombró observations_embedding_idx →
// knowledge_observations_embedding_idx, así que una lista literal se rompe en runtime
// —no en compilación— ante el próximo rename.
func TestBuildIndexDiscoveryQuery_FiltraPorIvfflatYPorLasTablasPedidas(t *testing.T) {
	q := buildIndexDiscoveryQuery()
	require.Contains(t, q, "pg_indexes")
	require.Contains(t, q, "ivfflat")
	require.Contains(t, q, "schemaname = 'public'")
	require.Contains(t, q, "= ANY($1)", "las tablas van como parámetro, no interpoladas")
}

// El preflight es lo que convierte un "must be owner of index" crudo en algo
// accionable. app_user NO es dueño de las tablas —las crea POSTGRES_USER vía
// domain-migrate— y en PostgreSQL 16 REINDEX exige ownership, porque el privilegio
// MAINTAIN es de PG17. Sin este chequeo el comando falla en el primer uso real.
func TestBuildOwnershipCheckQuery_ComparaContraElUsuarioActual(t *testing.T) {
	q := buildOwnershipCheckQuery()
	require.Contains(t, q, "current_user")
	require.Contains(t, q, "pg_class")
	require.Contains(t, q, "= ANY($1)")
}

// El orden de resolución existe porque el DSN del container apunta a app_user, que es
// justamente el que NO puede reindexar. Si resolveReindexDSN cayera en él por
// defecto sin decir nada, el comando fallaría siempre con un error de permisos que no
// nombra la causa.
func TestResolveReindexDSN_FlagGanaSobreElEntorno(t *testing.T) {
	t.Setenv("DOMAIN_DATABASE_ADMIN_URL", "postgres://admin/from-env")
	dsn, origen, err := resolveReindexDSN(reindexOpts{dsn: "postgres://admin/from-flag"})
	require.NoError(t, err)
	require.Equal(t, "postgres://admin/from-flag", dsn)
	require.Contains(t, origen, "--dsn")
}

func TestResolveReindexDSN_SinFlag_PrefiereElAdminURL(t *testing.T) {
	t.Setenv("DOMAIN_DATABASE_ADMIN_URL", "postgres://admin/x")
	t.Setenv("DOMAIN_DATABASE_URL", "postgres://app_user/x")
	dsn, origen, err := resolveReindexDSN(reindexOpts{})
	require.NoError(t, err)
	require.Equal(t, "postgres://admin/x", dsn)
	require.Contains(t, origen, "DOMAIN_DATABASE_ADMIN_URL")
}

// Caer en DOMAIN_DATABASE_URL no es un error —puede ser un entorno donde el dueño sea
// ese mismo rol— pero el origen tiene que decirlo para que el mensaje de un fallo de
// ownership posterior tenga sentido.
func TestResolveReindexDSN_SoloConDatabaseURL_LoUsaYDeclaraElOrigen(t *testing.T) {
	t.Setenv("DOMAIN_DATABASE_ADMIN_URL", "")
	t.Setenv("DOMAIN_DATABASE_AUTH_URL", "")
	t.Setenv("DOMAIN_DATABASE_URL", "postgres://app_user/x")
	dsn, origen, err := resolveReindexDSN(reindexOpts{})
	require.NoError(t, err)
	require.Equal(t, "postgres://app_user/x", dsn)
	require.Contains(t, origen, "DOMAIN_DATABASE_URL")
}

func TestResolveReindexDSN_SinNingunaFuente_Falla(t *testing.T) {
	t.Setenv("DOMAIN_DATABASE_ADMIN_URL", "")
	t.Setenv("DOMAIN_DATABASE_AUTH_URL", "")
	t.Setenv("DOMAIN_DATABASE_URL", "")
	t.Setenv("DATABASE_URL", "")
	_, _, err := resolveReindexDSN(reindexOpts{})
	require.Error(t, err)
}

// Un CONCURRENTLY fallido deja el índice inválido <idx>_ccold ocupando disco y sin
// usarse. El mensaje tiene que nombrarlo, o el operador no tiene forma de saber que
// le quedó basura que hay que dropear a mano.
func TestMensajeDeFalloConcurrente_NombraElIndiceInvalido(t *testing.T) {
	msg := mensajeDeFalloConcurrente("skills_embedding_idx")
	require.Contains(t, msg, "skills_embedding_idx_ccold")
	require.Contains(t, strings.ToUpper(msg), "DROP INDEX")
}
