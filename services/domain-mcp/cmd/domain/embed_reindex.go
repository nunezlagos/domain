package main

import (
	"context"
	"errors"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
)

type reindexOpts struct {
	dryRun       bool
	concurrently bool
	all          bool
	dsn          string
}

func parseReindexArgs(args []string) reindexOpts {
	o := reindexOpts{concurrently: true}
	for _, a := range args {
		switch {
		case a == "--dry-run":
			o.dryRun = true
		case a == "--no-concurrently":
			o.concurrently = false
		case a == "--all":
			o.all = true
		case strings.HasPrefix(a, "--dsn="):
			o.dsn = strings.TrimPrefix(a, "--dsn=")
		}
	}
	return o
}

// reindexTables son las tablas cuyos ivfflat quedan desalineados tras un backfill.
// El default se corresponde 1:1 con backfillTargets(): un UPDATE masivo de embeddings
// no re-entrena los centroides, así que la tabla que el backfill escribe es
// exactamente la que hay que reindexar.
//
// Con --all se suman las que tienen ivfflat pero el backfill no toca:
// chat_document_embeddings (columna NOT NULL, la puebla el RAG del chat de
// domain-admin) y llm_semantic_cache. A esas un backfill nunca las invalida.
func reindexTables(all bool) []string {
	var tables []string
	for _, tg := range backfillTargets() {
		tables = append(tables, tg.table)
	}
	if all {
		tables = append(tables, "chat_document_embeddings", "llm_semantic_cache")
	}
	return tables
}

// buildIndexDiscoveryQuery descubre los índices en vez de nombrarlos. La migración
// 000155 renombró observations_embedding_idx a knowledge_observations_embedding_idx,
// así que una lista literal se rompe ante el próximo rename en RUNTIME y no en
// compilación. Mismo criterio que usa la migración 000275 para reconstruirlos.
func buildIndexDiscoveryQuery() string {
	return `SELECT indexname FROM pg_indexes
	         WHERE schemaname = 'public'
	           AND tablename = ANY($1)
	           AND indexdef LIKE '%ivfflat%'
	         ORDER BY indexname`
}

// buildOwnershipCheckQuery devuelve los índices cuya tabla NO es del usuario actual.
// REINDEX exige ownership en PostgreSQL 16 —el privilegio MAINTAIN llegó en PG17— y
// el DSN del container apunta a app_user, que no es dueño de nada: las tablas las crea
// POSTGRES_USER vía domain-migrate.
func buildOwnershipCheckQuery() string {
	return `SELECT i.relname FROM pg_class i
	          JOIN pg_index ix ON ix.indexrelid = i.oid
	          JOIN pg_class t ON t.oid = ix.indrelid
	         WHERE i.relname = ANY($1)
	           AND pg_get_userbyid(t.relowner) <> current_user`
}

func buildReindexStmt(index string, concurrently bool) string {
	if concurrently {
		return fmt.Sprintf("REINDEX INDEX CONCURRENTLY public.%q", index)
	}
	return fmt.Sprintf("REINDEX INDEX public.%q", index)
}

// mensajeDeFalloConcurrente nombra la basura que deja un CONCURRENTLY abortado: el
// índice inválido sigue ocupando disco y no lo usa nadie, y sin nombrarlo el operador
// no tiene forma de saber que le toca limpiarlo.
func mensajeDeFalloConcurrente(index string) string {
	return fmt.Sprintf("el reindex concurrente de %s abortó y pudo dejar un índice inválido; "+
		"revisá con \\di %s* y limpialo con: DROP INDEX IF EXISTS public.%s_ccold",
		index, index, index)
}

// dsnCandidatosMantenimiento es el orden de resolución del DSN para los comandos de
// MANTENIMIENTO de la instancia (reindex y backfill de embeddings). El admin/auth va PRIMERO
// y eso no es una preferencia de estilo: desde el RLS de DOMAINSERV-185 las tablas
// knowledge_docs y knowledge_chunks solo son visibles con app.current_project_id seteado, y
// estos comandos son GLOBALES por diseño —recorren la instancia entera, no un proyecto—, así
// que no tienen un GUC que setear.
//
// Con el DSN de app_user (NOBYPASSRLS) el SELECT de filas pendientes devolvería CERO SIN ERROR
// y el comando reportaría éxito sin haber hecho nada. Ese modo de falla ya ocurrió en este repo
// por otra causa —el deploy del 2026-07-24 dejó 0 de 2065 observaciones con embedding porque el
// embedder había degradado a noop y el backfill igual salió bien— y es indistinguible de "no
// había nada pendiente", que es lo que lo hace peligroso.
//
// La lista es UNA SOLA y compartida a propósito: duplicar el orden en los dos comandos es la
// clase de cosa que diverge, y el que quede atrás falla en silencio.
var dsnCandidatosMantenimiento = []string{
	"DOMAIN_DATABASE_ADMIN_URL",
	"DOMAIN_DATABASE_AUTH_URL",
	"DOMAIN_DATABASE_URL",
	"DATABASE_URL",
}

// resolverDSNMantenimiento devuelve el primer DSN disponible del orden compartido y de dónde
// salió. El origen se devuelve porque quien llama necesita poder advertir cuando cayó al rol de
// la app: un backfill que corre con RLS activo no falla, miente.
func resolverDSNMantenimiento() (dsn string, origen string) {
	for _, k := range dsnCandidatosMantenimiento {
		if v := os.Getenv(k); v != "" {
			return v, k
		}
	}
	return "", ""
}

// resolveReindexDSN elige el DSN y declara de dónde salió. El origen se reporta porque
// un fallo de ownership posterior es incomprensible sin saber con qué rol se conectó.
func resolveReindexDSN(o reindexOpts) (dsn string, origen string, err error) {
	if o.dsn != "" {
		return o.dsn, "--dsn", nil
	}
	for _, k := range dsnCandidatosMantenimiento {
		if v := os.Getenv(k); v != "" {
			return v, k, nil
		}
	}
	return "", "", errors.New("sin DSN: pasá --dsn=... o seteá DOMAIN_DATABASE_ADMIN_URL con el rol dueño del schema (POSTGRES_USER)")
}

// assertOwnerOrFail corta ANTES de tocar el primer índice. Fallar a mitad de camino
// dejaría unos reindexados y otros no, sin forma de saber cuáles.
func assertOwnerOrFail(ctx context.Context, pool *pgxpool.Pool, indexes []string, origen string) error {
	rows, err := pool.Query(ctx, buildOwnershipCheckQuery(), indexes)
	if err != nil {
		return fmt.Errorf("preflight de ownership: %w", err)
	}
	defer rows.Close()
	var ajenos []string
	for rows.Next() {
		var name string
		if err := rows.Scan(&name); err != nil {
			return err
		}
		ajenos = append(ajenos, name)
	}
	if err := rows.Err(); err != nil {
		return err
	}
	if len(ajenos) == 0 {
		return nil
	}
	return fmt.Errorf("el rol de %s no es dueño de %d índice(s) (%s) y REINDEX lo exige en PostgreSQL 16.\n"+
		"  Corré con el rol dueño (POSTGRES_USER), desde el directorio del compose:\n"+
		"  docker compose run --rm --no-deps domain-seed embed-reindex\n"+
		"  domain-seed comparte la imagen y ya recibe el DSN de POSTGRES_USER. Un docker exec\n"+
		"  con $POSTGRES_USER escrito en la shell del host expande VACÍO y falla como nonroot.",
		origen, len(ajenos), strings.Join(ajenos, ", "))
}

func discoverReindexIndexes(ctx context.Context, pool *pgxpool.Pool, tables []string) ([]string, error) {
	rows, err := pool.Query(ctx, buildIndexDiscoveryQuery(), tables)
	if err != nil {
		return nil, fmt.Errorf("descubrir índices: %w", err)
	}
	defer rows.Close()
	var indexes []string
	for rows.Next() {
		var name string
		if err := rows.Scan(&name); err != nil {
			return nil, err
		}
		indexes = append(indexes, name)
	}
	return indexes, rows.Err()
}

// reindexAll no corta ante el primer error: el resto de los índices sigue siendo
// recuperable y detenerse dejaría el trabajo a medias sin decir cuánto se hizo.
func reindexAll(ctx context.Context, pool *pgxpool.Pool, indexes []string, o reindexOpts) int {
	fallidos := 0
	for _, idx := range indexes {
		stmt := buildReindexStmt(idx, o.concurrently)
		if o.dryRun {
			fmt.Printf("  [dry-run] %s\n", stmt)
			continue
		}
		inicio := time.Now()
		if _, err := pool.Exec(ctx, stmt); err != nil {
			fallidos++
			fmt.Fprintf(os.Stderr, "  %s: %v\n", idx, err)
			if o.concurrently {
				fmt.Fprintf(os.Stderr, "  %s\n", mensajeDeFalloConcurrente(idx))
			}
			continue
		}
		fmt.Printf("  %s: %s\n", idx, time.Since(inicio).Round(time.Millisecond))
	}
	return fallidos
}

// runEmbedReindex reconstruye los ivfflat después de un backfill masivo.
//
// Uso: domain embed-reindex [--all] [--no-concurrently] [--dry-run] [--dsn=DSN]
//
// install.sh lo corre al final de cada deploy. Antes solo imprimía el comando como
// sugerencia —para no congelar la búsqueda sin que nadie lo pidiera— pero en un mes
// nadie lo corrió y el recall quedó degradado: la sugerencia protegía de un riesgo que
// solo aplica con --no-concurrently, que no es el default.
//
// Requiere un DSN dueño del schema. El del container es app_user, que no lo es, y
// además la migración 000029 le fija statement_timeout=30s: de ahí el SET a 0.
func runEmbedReindex(args []string) {
	o := parseReindexArgs(args)
	dsn, origen, err := resolveReindexDSN(o)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(2)
	}
	ctx := context.Background()
	pool, err := pgxpoolNew(ctx, dsn)
	if err != nil {
		fmt.Fprintln(os.Stderr, "open pool:", err)
		os.Exit(1)
	}
	defer pool.Close()

	tables := reindexTables(o.all)
	indexes, err := discoverReindexIndexes(ctx, pool, tables)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	if len(indexes) == 0 {
		fmt.Printf("sin índices ivfflat en %s: nada que reindexar\n", strings.Join(tables, ", "))
		return
	}
	if err := assertOwnerOrFail(ctx, pool, indexes, origen); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	// el timeout de app_user mataría un reindex de segundos; CONCURRENTLY además no
	// puede correr dentro de una transacción, así que va por Exec directo al pool
	if !o.dryRun {
		if _, err := pool.Exec(ctx, "SET statement_timeout = 0"); err != nil {
			fmt.Fprintln(os.Stderr, "desactivar statement_timeout:", err)
			os.Exit(1)
		}
	}
	fmt.Printf("Reindex de %d índice(s) ivfflat (dsn=%s, concurrently=%v)\n", len(indexes), origen, o.concurrently)
	if fallidos := reindexAll(ctx, pool, indexes, o); fallidos > 0 {
		fmt.Fprintf(os.Stderr, "Reindex incompleto: %d de %d fallaron\n", fallidos, len(indexes))
		os.Exit(1)
	}
	fmt.Printf("Reindex done: %d índice(s) dry_run=%v\n", len(indexes), o.dryRun)
}
