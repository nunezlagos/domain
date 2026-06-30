// edge_inference.go — system cron de inferencia de aristas tipadas entre
// memorias usando MiniMax-M3 (vía endpoint anthropic-compatible), SIN embeddings.
//
// NO es un cron user-defined (tabla `crons`, REQ-10) ni un "agente liberado":
// es un job interno hardcoded, leader-gated y acotado a UNA sola operación
// (observation.Service.InferEdgesLLM). No tiene acceso a tools, ni a un loop de
// razonamiento abierto: arma pares candidatos por señales baratas, le pide a
// MiniMax que clasifique la relación de cada par y crea aristas con
// origin='inferred'. Nada más.
//
// Se enable por config flag (DOMAIN_EDGE_INFERENCE_ENABLED, default false) y
// sólo corre en el leader del cluster (igual patrón que HeartbeatWatcher y
// OrphanAuditor: el caller lo invoca dentro de un block RunAsLeader).
//
// DEGRADACIÓN ELEGANTE (regla dura 5):
//   - Sin MINIMAX_API_KEY el provider "minimax" no se registra; InferEdgesLLM
//     devuelve observation.ErrInferenceUnavailable. El job lo detecta UNA vez
//     al arrancar (precheck) y NO arranca el loop: loguea y sale limpio, sin
//     crashear el scheduler. Si se cae a mitad de corrida, el error por-proyecto
//     se loguea y se sigue con el resto.
package systemcron

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"

	"nunezlagos/domain/internal/service/observation"
)

// EdgeInferencer corre la inferencia de aristas por proyecto, periódicamente.
//
// Depende SOLO de:
//   - Obs: para InferEdgesLLM (resuelve el provider MiniMax y arma candidatos).
//   - Edges: el EdgeLinker que crea las aristas (origin='inferred').
//   - Pool: para enumerar los project_id activos (los que tienen observations).
//
// No recibe ninguna otra capability: el scope es lectura de memorias + creación
// de aristas, nada más.
type EdgeInferencer struct {
	Obs   *observation.Service
	Edges observation.EdgeLinker
	Pool  *pgxpool.Pool

	Tick     time.Duration // default 6h
	MaxPairs int           // pares candidatos por proyecto por pasada; default 30
	// ProjectBatch: cuántos proyectos procesar por pasada (acota costo LLM).
	// default 50.
	ProjectBatch int
	Logger       *slog.Logger
}

// Start corre el loop hasta ctx cancel. Asume llamado dentro de RunAsLeader.
//
// Precheck de degradación: si MiniMax no está disponible, NO arranca el loop.
// Esto evita que el ticker dispare pasadas que sólo van a devolver
// ErrInferenceUnavailable cada N horas.
func (e *EdgeInferencer) Start(ctx context.Context) {
	if e.Tick == 0 {
		e.Tick = 6 * time.Hour
	}
	if e.MaxPairs == 0 {
		e.MaxPairs = 30
	}
	if e.ProjectBatch == 0 {
		e.ProjectBatch = 50
	}
	logger := e.Logger
	if logger == nil {
		logger = slog.Default()
	}

	if e.Obs == nil || e.Edges == nil || e.Pool == nil {
		logger.Warn("edge-inference disabled: dependencias no inyectadas (Obs/Edges/Pool)")
		return
	}

	// Precheck: sin MiniMax, salir limpio sin arrancar el ticker.
	if _, err := e.Obs.InferEdgesLLM(ctx, e.Edges, observation.InferEdgesLLMInput{
		ProjectID: uuid.Nil, // proyecto inexistente: si MiniMax existe, devuelve 0 candidatos sin tocar nada
	}); errors.Is(err, observation.ErrInferenceUnavailable) {
		logger.Info("edge-inference disabled: MiniMax no configurado (sin MINIMAX_API_KEY); el job no corre")
		return
	}

	logger.Info("edge-inference started",
		slog.Duration("tick", e.Tick),
		slog.Int("max_pairs", e.MaxPairs),
		slog.Int("project_batch", e.ProjectBatch))

	e.runTick(ctx, logger)

	ticker := time.NewTicker(e.Tick)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			logger.Info("edge-inference stopping")
			return
		case <-ticker.C:
			e.runTick(ctx, logger)
		}
	}
}

func (e *EdgeInferencer) runTick(ctx context.Context, logger *slog.Logger) {
	projects, err := e.activeProjects(ctx)
	if err != nil {
		logger.Error("edge-inference: listar proyectos falló", slog.Any("err", err))
		return
	}
	if len(projects) == 0 {
		return
	}

	var totalCreated, totalExisting int
	for _, pid := range projects {
		select {
		case <-ctx.Done():
			return
		default:
		}

		res, err := e.Obs.InferEdgesLLM(ctx, e.Edges, observation.InferEdgesLLMInput{
			ProjectID: pid,
			MaxPairs:  e.MaxPairs,
		})
		if err != nil {
			// Degradación a mitad de corrida (ej. MiniMax cayó): loguear y seguir.
			if errors.Is(err, observation.ErrInferenceUnavailable) {
				logger.Warn("edge-inference: MiniMax no disponible a mitad de corrida; abortando pasada")
				return
			}
			logger.Error("edge-inference: proyecto falló",
				slog.String("project_id", pid.String()), slog.Any("err", err))
			continue
		}
		if res == nil {
			continue
		}
		totalCreated += res.Created
		totalExisting += res.Existing
		if res.Created > 0 {
			logger.Info("edge-inference: aristas creadas",
				slog.String("project_id", pid.String()),
				slog.Int("candidates", res.Candidates),
				slog.Int("created", res.Created),
				slog.Int("existing", res.Existing),
				slog.Int("skipped", res.Skipped))
		}
	}
	logger.Info("edge-inference: pasada completa",
		slog.Int("projects", len(projects)),
		slog.Int("created", totalCreated),
		slog.Int("existing", totalExisting))
}

// activeProjects devuelve los project_id que tienen al menos una observation
// activa. Single-tenant: NO filtra por organization_id (regla dura 1). Acota a
// ProjectBatch para no abrir un fan-out ilimitado de llamadas al LLM por pasada.
func (e *EdgeInferencer) activeProjects(ctx context.Context) ([]uuid.UUID, error) {
	rows, err := e.Pool.Query(ctx, `
		SELECT DISTINCT project_id
		FROM knowledge_observations
		WHERE deleted_at IS NULL
		ORDER BY project_id
		LIMIT $1
	`, e.ProjectBatch)
	if err != nil {
		return nil, fmt.Errorf("query active projects: %w", err)
	}
	defer rows.Close()

	var out []uuid.UUID
	for rows.Next() {
		var id uuid.UUID
		if err := rows.Scan(&id); err != nil {
			return nil, fmt.Errorf("scan project_id: %w", err)
		}
		out = append(out, id)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate project_ids: %w", err)
	}
	return out, nil
}
