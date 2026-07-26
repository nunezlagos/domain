package main

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"

	"nunezlagos/domain/internal/llm"
)

type backfillOpts struct {
	limit   int
	dryRun  bool
	all     bool
	pauseMS int
	orgArg  string
}

// backfillTarget describe una tabla con columna de embedding regenerable.
type backfillTarget struct {
	table        string
	textCol      string
	embCol       string
	hasDeletedAt bool
}

// backfillTargets son las tablas que el backfill repuebla. knowledge_chunks NO
// tiene deleted_at — incluir el filtro la rompería con "column does not exist".
func backfillTargets() []backfillTarget {
	return []backfillTarget{
		{table: "knowledge_observations", textCol: "content", embCol: "embedding", hasDeletedAt: true},
		{table: "knowledge_chunks", textCol: "content", embCol: "embedding", hasDeletedAt: false},
	}
}

func parseBackfillArgs(args []string) backfillOpts {
	o := backfillOpts{limit: 200, pauseMS: 100}
	for _, a := range args {
		switch {
		case a == "--dry-run":
			o.dryRun = true
		case a == "--all":
			o.all = true
		case strings.HasPrefix(a, "--limit="):
			fmt.Sscanf(a, "--limit=%d", &o.limit)
		case strings.HasPrefix(a, "--pause-ms="):
			fmt.Sscanf(a, "--pause-ms=%d", &o.pauseMS)
		default:
			if o.orgArg == "" {
				o.orgArg = a
			}
		}
	}
	return o
}

// buildBackfillQuery arma el SELECT de filas pendientes: sin embedding, o con uno
// en cero. Ese segundo caso existe porque el embedder degradado a noop
// (DOMAINSERV-157) escribió vectores de ceros, y un vector de ceros NO es NULL:
// quedaban fuera del backfill para siempre mientras competían en el ranking con
// la misma distancia contra cualquier búsqueda. Se detectan por el producto
// interno consigo mismo —-||v||², cero solo en el vector nulo— porque l2_norm es
// ambiguo en pgvector y vector_dims obligaría a conocer la dimensión.
//
// La idempotencia se conserva: una fila con norma real no vuelve a tomarse, así
// que re-correr el backfill no gasta llamadas al provider.
func buildBackfillQuery(table, textCol, embCol string, hasDeletedAt bool) string {
	deleted := ""
	if hasDeletedAt {
		deleted = "\n\t\t   AND deleted_at IS NULL"
	}
	return fmt.Sprintf(
		`SELECT %s, %s FROM %s
		 WHERE (%s IS NULL OR (%s <#> %s) = 0)%s
		   AND LENGTH(TRIM(%s)) > 0
		 ORDER BY created_at ASC
		 LIMIT $1`,
		"id", textCol, table, embCol, embCol, embCol, deleted, textCol,
	)
}

// runEmbedBackfill (REQ-68): recorre las tablas con embedding NULL, genera
// vectors con el provider actual y los persiste. Útil al activar un provider
// real sobre data sembrada con noop.
//
// Uso: domain embed-backfill [<organization-uuid>] [--limit=N] [--all]
//
//	[--dry-run] [--pause-ms=N]
//
// DOMAINSERV-80 H2: el organization-uuid es OPCIONAL y no scopea nada — se
// acepta por compatibilidad con invocaciones previas. Ninguna de las tablas
// tiene columna organization_id (knowledge_observations tiene project_id,
// knowledge_chunks va por knowledge_doc_id), así que el backfill es GLOBAL a
// la instancia. Exigirlo como obligatorio sugería un scoping inexistente.
//
// Performance:
//   - Procesa de a 1 con una pausa configurable (default 100ms) pensada para el
//     rate-limit de APIs remotas. Con un provider local (ollama) conviene
//     --pause-ms=0.
//   - --all itera hasta agotar las filas pendientes; sin él procesa un lote.
//   - Si el provider es noop, sale sin hacer nada.
func runEmbedBackfill(args []string) {
	o := parseBackfillArgs(args)
	limit, dryRun := o.limit, o.dryRun
	if o.orgArg != "" {
		if _, err := uuid.Parse(o.orgArg); err != nil {
			fmt.Fprintln(os.Stderr, "UUID inválido:", err)
			os.Exit(2)
		}
		fmt.Println("Nota: el backfill es global a la instancia; el organization-uuid se ignora.")
	}
	ctx := context.Background()
	logger := slog.Default()
	embedder := chooseEmbedder(logger)
	if _, isNoop := embedder.(llm.NopEmbedder); isNoop {
		fmt.Println("Embedder = noop → nada que backfillear. Configura DOMAIN_EMBEDDING_PROVIDER " +
			"(ollama con services/ollama levantado, u openai/voyage con su API key). " +
			"Si ya lo configuraste, el provider puede haber degradado a noop por dimensión " +
			"incompatible con el esquema: revisá el log del arranque.")
		return
	}

	dsn := os.Getenv("DOMAIN_DATABASE_URL")
	if dsn == "" {
		dsn = os.Getenv("DATABASE_URL")
	}
	if dsn == "" {
		fmt.Fprintln(os.Stderr, "DOMAIN_DATABASE_URL no seteado")
		os.Exit(1)
	}
	pool, err := pgxpoolNew(ctx, dsn)
	if err != nil {
		fmt.Fprintln(os.Stderr, "open pool:", err)
		os.Exit(1)
	}
	defer pool.Close()

	totals := map[string]int{}
	for _, tg := range backfillTargets() {
		for {
			escritos, candidatos, err := backfillTable(ctx, pool, embedder, tg, limit, dryRun, o.pauseMS)
			if err != nil {
				fmt.Fprintf(os.Stderr, "%s: %v\n", tg.table, err)
				os.Exit(1)
			}
			totals[tg.table] += escritos
			if !debeIterarOtroLote(o, escritos, candidatos, limit) {
				break
			}
		}
	}
	fmt.Printf("Backfill done: observations=%d knowledge_chunks=%d dry_run=%v\n",
		totals["knowledge_observations"], totals["knowledge_chunks"], dryRun)
}

// debeIterarOtroLote decide si queda trabajo por hacer. El corte por escrituras
// es lo que impide un loop infinito: un lote lleno que no escribió NADA son filas
// que el provider rechaza siempre (texto que excede su contexto, por ejemplo), así
// que volver a pedirlas devuelve el mismo lote para siempre. Antes el corte
// miraba la cantidad de candidatos, que no baja cuando el embed falla.
func debeIterarOtroLote(o backfillOpts, escritos, candidatos, limit int) bool {
	if !o.all || o.dryRun {
		return false
	}
	return escritos > 0 && candidatos >= limit
}

// backfillTable devuelve (escritos, candidatos): son distintos cuando el provider
// rechaza una fila, y confundirlos es lo que hacía que el backfill reportara éxito
// habiendo fallado en todas.
func backfillTable(ctx context.Context, pool *pgxpool.Pool, emb llm.Embedder,
	tg backfillTarget, limit int, dryRun bool, pauseMS int) (int, int, error) {
	table, textCol, embCol := tg.table, tg.textCol, tg.embCol
	rows, err := pool.Query(ctx,
		buildBackfillQuery(table, textCol, embCol, tg.hasDeletedAt), limit)
	if err != nil {
		return 0, 0, fmt.Errorf("query %s: %w", table, err)
	}
	type row struct {
		ID   uuid.UUID
		Text string
	}
	var items []row
	for rows.Next() {
		var r row
		if err := rows.Scan(&r.ID, &r.Text); err != nil {
			rows.Close()
			return 0, 0, err
		}
		items = append(items, r)
	}
	rows.Close()
	fmt.Printf("  %s: %d filas con embedding faltante o en cero\n", table, len(items))
	if dryRun {
		return 0, len(items), nil
	}
	escritos := 0
	for i, it := range items {
		if embedYPersistir(ctx, pool, emb, tg, it.ID, it.Text) {
			escritos++
		}
		if (i+1)%10 == 0 {
			fmt.Printf("  %s: %d/%d\n", table, i+1, len(items))
		}
		if pauseMS > 0 {
			time.Sleep(time.Duration(pauseMS) * time.Millisecond)
		}
	}
	if escritos < len(items) {
		fmt.Fprintf(os.Stderr, "  %s: %d de %d filas quedaron sin embedding\n",
			table, len(items)-escritos, len(items))
	}
	return escritos, len(items), nil
}

// embedYPersistir devuelve si la fila quedó con un embedding real. Una fila que
// falla no corta el backfill —el resto de la tabla sigue siendo recuperable— pero
// su fallo tiene que ser visible: es lo que distingue "no había nada que hacer" de
// "no se pudo hacer nada".
func embedYPersistir(ctx context.Context, pool *pgxpool.Pool, emb llm.Embedder,
	tg backfillTarget, id uuid.UUID, text string) bool {
	v, err := emb.Embed(ctx, text)
	if err != nil {
		fmt.Fprintf(os.Stderr, "  embed %s/%s: %v\n", tg.table, id, err)
		return false
	}
	if v == nil || llm.IsZero(v) {
		return false
	}
	if _, err := pool.Exec(ctx, fmt.Sprintf(
		`UPDATE %s SET %s = $2::vector WHERE id=$1`,
		tg.table, tg.embCol,
	), id, vectorLiteral(v)); err != nil {
		fmt.Fprintf(os.Stderr, "  update %s/%s: %v\n", tg.table, id, err)
		return false
	}
	return true
}

func vectorLiteral(v []float32) string {
	var sb strings.Builder
	sb.WriteByte('[')
	for i, x := range v {
		if i > 0 {
			sb.WriteByte(',')
		}
		fmt.Fprintf(&sb, "%g", x)
	}
	sb.WriteByte(']')
	return sb.String()
}
