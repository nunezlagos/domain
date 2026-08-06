package embedding

import (
	"context"
	"fmt"
	"log/slog"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"

	"nunezlagos/domain/internal/llm"
)

// Completer repuebla los embeddings de knowledge_chunks que quedaron pendientes.
//
// Existe porque knowledge_save embebía los chunks DENTRO del request: ~2,3s por chunk
// contra Ollama, lineal en la cantidad de chunks, así que un documento de 157KB
// agotaba el timeout del cliente antes de contestar (DOMAINSERV-227). Ahora el save
// persiste los chunks con embedding NULL y este worker los completa después.
//
// El Pool tiene que ser el de app_admin (BYPASSRLS): desde la 000287 knowledge_chunks
// está bajo RLS por app.current_project_id y este barrido es GLOBAL por diseño, así
// que no hay un GUC único que setear. Con el rol de la app el SELECT devuelve CERO SIN
// ERROR y el worker reportaría trabajo hecho sin haber escrito un solo vector — por eso
// Start verifica el rol y lo grita en vez de asumirlo.
type Completer struct {
	Pool     *pgxpool.Pool
	Embedder llm.Embedder
	Tick     time.Duration
	Batch    int
	Logger   *slog.Logger

	backoff int
}

const (
	tickPorDefecto  = 30 * time.Second
	batchPorDefecto = 50
	backoffMaximo   = 16
)

// Start corre el loop hasta que se cancele el ctx. Asume llamado dentro de RunAsLeader:
// dos réplicas barriendo el mismo lote gastarían el doble de llamadas al provider para
// escribir el mismo vector.
func (c *Completer) Start(ctx context.Context) {
	if c.Tick == 0 {
		c.Tick = tickPorDefecto
	}
	if c.Batch == 0 {
		c.Batch = batchPorDefecto
	}
	logger := c.Logger
	if logger == nil {
		logger = slog.Default()
	}
	if c.Pool == nil || c.Embedder == nil {
		logger.Warn("embedding-completer: sin pool o sin embedder — no arranca")
		return
	}
	if _, esNoop := c.Embedder.(llm.NopEmbedder); esNoop {
		logger.Info("embedding-completer: embedder = noop, no hay vectores que completar")
		return
	}
	c.avisarSiElRolNoBypaseaRLS(ctx, logger)

	logger.Info("embedding-completer started",
		slog.Duration("tick", c.Tick), slog.Int("batch", c.Batch))

	ticker := time.NewTicker(c.Tick)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			logger.Info("embedding-completer stopping")
			return
		case <-ticker.C:
			c.correrTick(ctx, logger)
		}
	}
}

// avisarSiElRolNoBypaseaRLS es el guard contra el falso verde: sin BYPASSRLS el
// barrido no ve una sola fila y el log de "0 pendientes" es indistinguible de estar
// al día.
func (c *Completer) avisarSiElRolNoBypaseaRLS(ctx context.Context, logger *slog.Logger) {
	var bypass bool
	err := c.Pool.QueryRow(ctx,
		`SELECT rolbypassrls FROM pg_roles WHERE rolname = current_user`).Scan(&bypass)
	if err != nil {
		logger.Warn("embedding-completer: no pude verificar BYPASSRLS del rol",
			slog.Any("err", err))
		return
	}
	if !bypass {
		logger.Warn("embedding-completer: el rol NO tiene BYPASSRLS — knowledge_chunks está " +
			"bajo RLS y el barrido va a ver CERO filas sin fallar. Seteá DOMAIN_DATABASE_AUTH_URL " +
			"con el rol app_admin (DOMAINSERV-185)")
	}
}

// correrTick hace una pasada y ajusta el backoff. Un lote lleno que no escribió NADA
// son filas que el provider rechaza siempre —texto que excede su contexto, por
// ejemplo—, así que volver a pedirlas cada tick golpearía al provider para siempre.
func (c *Completer) correrTick(ctx context.Context, logger *slog.Logger) {
	if c.backoff > 0 {
		c.backoff--
		return
	}
	escritos, candidatos, err := c.Run(ctx)
	if err != nil {
		logger.Error("embedding-completer tick failed", slog.Any("err", err))
		c.crecerBackoff()
		return
	}
	if candidatos == 0 {
		c.backoff = 0
		return
	}
	logger.Info("embedding-completer tick",
		slog.Int("escritos", escritos), slog.Int("candidatos", candidatos))
	if escritos == 0 {
		c.crecerBackoff()
		return
	}
	c.backoff = 0
}

func (c *Completer) crecerBackoff() {
	if c.backoff == 0 {
		c.backoff = 1
		return
	}
	if c.backoff < backoffMaximo {
		c.backoff *= 2
	}
}

type filaPendiente struct {
	ID    uuid.UUID
	Texto string
}

// Run hace UNA pasada y devuelve (escritos, candidatos). Son distintos cuando el
// provider rechaza una fila, y confundirlos es lo que hacía que el backfill reportara
// éxito habiendo fallado en todas.
func (c *Completer) Run(ctx context.Context) (int, int, error) {
	target := KnowledgeChunks()
	pendientes, err := c.leerPendientes(ctx, target)
	if err != nil {
		return 0, 0, err
	}
	escritos := 0
	for _, fila := range pendientes {
		vec, err := c.Embedder.Embed(ctx, fila.Texto)
		if err != nil {
			return escritos, len(pendientes), fmt.Errorf("embed %s: %w", fila.ID, err)
		}
		v := VectorOrNil(vec)
		if v == nil {
			continue
		}
		sql := fmt.Sprintf(`UPDATE %s SET %s = $1 WHERE id = $2`, target.Table, target.EmbCol)
		if _, err := c.Pool.Exec(ctx, sql, v, fila.ID); err != nil {
			return escritos, len(pendientes), fmt.Errorf("update %s: %w", fila.ID, err)
		}
		escritos++
	}
	return escritos, len(pendientes), nil
}

func (c *Completer) leerPendientes(ctx context.Context, t Target) ([]filaPendiente, error) {
	rows, err := c.Pool.Query(ctx, PendingRowsQuery(t), c.Batch)
	if err != nil {
		return nil, fmt.Errorf("query pendientes de %s: %w", t.Table, err)
	}
	defer rows.Close()
	var out []filaPendiente
	for rows.Next() {
		var f filaPendiente
		if err := rows.Scan(&f.ID, &f.Texto); err != nil {
			return nil, fmt.Errorf("scan: %w", err)
		}
		out = append(out, f)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate: %w", err)
	}
	return out, nil
}
