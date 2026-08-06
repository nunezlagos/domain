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
	"nunezlagos/domain/internal/service/embedding"
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

// backfillTargets son las tablas que el backfill repuebla. La lista vive en
// internal/service/embedding porque el Completer que corre dentro del server barre las
// mismas filas (DOMAINSERV-227): con dos copias, la que quedara atrás dejaría de
// repoblar una tabla sin que nada falle.
func backfillTargets() []backfillTarget {
	targets := embedding.Targets()
	out := make([]backfillTarget, 0, len(targets))
	for _, t := range targets {
		out = append(out, backfillTarget{
			table: t.Table, textCol: t.TextCol, embCol: t.EmbCol, hasDeletedAt: t.HasDeletedAt,
		})
	}
	return out
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

// buildBackfillQuery delega en embedding.PendingRowsQuery: el predicado de "pendiente"
// es uno solo y lo comparte con el Completer del server (DOMAINSERV-227).
func buildBackfillQuery(table, textCol, embCol string, hasDeletedAt bool) string {
	return embedding.PendingRowsQuery(embedding.Target{
		Table: table, TextCol: textCol, EmbCol: embCol, HasDeletedAt: hasDeletedAt,
	})
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

	// DOMAINSERV-185: el orden lo decide dsnCandidatosMantenimiento y NO se lee
	// DOMAIN_DATABASE_URL directo. knowledge_chunks es uno de los tres targets del backfill y
	// desde la 000287 está bajo RLS: con el rol de la app (app_user, NOBYPASSRLS) y sin
	// app.current_project_id, el SELECT de pendientes devuelve CERO sin error y este comando
	// reporta éxito sin repoblar un solo vector.
	dsn, origenDSN := resolverDSNMantenimiento()
	if dsn == "" {
		fmt.Fprintln(os.Stderr, "sin DSN: seteá DOMAIN_DATABASE_ADMIN_URL, DOMAIN_DATABASE_AUTH_URL o DOMAIN_DATABASE_URL")
		os.Exit(1)
	}
	if origenDSN == "DOMAIN_DATABASE_URL" || origenDSN == "DATABASE_URL" {
		// no se aborta: en dev-local puede no haber otro DSN y el backfill sigue siendo útil
		// para las dos tablas que NO están bajo RLS (knowledge_observations y skills)
		fmt.Fprintf(os.Stderr,
			"advertencia: el DSN viene de %s, que es el rol de la app. Si tiene RLS activo, "+
				"knowledge_chunks va a verse VACÍO y sus embeddings no se van a repoblar (DOMAINSERV-185)\n",
			origenDSN)
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
	// recorre los targets en vez de nombrar tablas: un target nuevo que no
	// aparezca en el resumen se lee como que no se backfilleó nada
	var resumen strings.Builder
	for _, tg := range backfillTargets() {
		fmt.Fprintf(&resumen, "%s=%d ", tg.table, totals[tg.table])
	}
	fmt.Printf("Backfill done: %sdry_run=%v\n", resumen.String(), dryRun)
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
