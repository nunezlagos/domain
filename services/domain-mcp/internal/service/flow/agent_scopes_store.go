package flow

import (
	"context"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
)

// ejecutorSQL es la interfaz chica que este paquete CONSUME: alcanza con una tx (lo normal,
// porque flow_agent_scopes tiene FORCE RLS y el GUC de proyecto solo vive dentro de una) o un
// pool en tests. Se define acá y no junto a pgx por la policy de acoplamiento.
type ejecutorSQL interface {
	Query(ctx context.Context, sql string, args ...any) (pgx.Rows, error)
	Exec(ctx context.Context, sql string, args ...any) (pgconn.CommandTag, error)
}

// ScopesVigentesDelFlow devuelve el territorio reservado por cada agente del flow: filas no
// revocadas y no vencidas. Es la respuesta que un token aislado no puede dar y sin la cual
// ValidarParticionDisjunta no tiene contra qué comparar.
func ScopesVigentesDelFlow(ctx context.Context, db ejecutorSQL, flowRunID uuid.UUID) ([]ScopeVigente, error) {
	rows, err := db.Query(ctx, `
		SELECT agent_id, allowed_paths
		FROM flow_agent_scopes
		WHERE flow_run_id = $1
		  AND revoked_at IS NULL
		  AND expires_at > now()`, flowRunID)
	if err != nil {
		return nil, fmt.Errorf("scopes vigentes: %w", err)
	}
	defer rows.Close()

	var out []ScopeVigente
	for rows.Next() {
		var s ScopeVigente
		if err := rows.Scan(&s.AgentID, &s.AllowedPaths); err != nil {
			return nil, fmt.Errorf("scopes vigentes: scan: %w", err)
		}
		out = append(out, s)
	}
	return out, rows.Err()
}

// RegistrarScope reserva el territorio del agente. Es un UPSERT y no un INSERT porque cada
// cierre de fase re-emite el token (claude_hook.go:41 dispara el grant en
// orchestrate_phase_result, flow_status y orchestrate_confirm): con una fila por emisión el
// agente chocaría con su propia fila anterior.
//
// El project_id NO viaja por parámetro: sale del flow_run en el mismo statement, así que no
// puede desincronizarse del flow ni permitir que un caller lo falsee. Si el flow no existe, el
// SELECT no devuelve filas y el INSERT no inserta nada — que es el fail-closed correcto.
func RegistrarScope(ctx context.Context, db ejecutorSQL, flowRunID uuid.UUID, agentID string, allowedPaths []string, expiresAt time.Time) error {
	if allowedPaths == nil {
		allowedPaths = []string{}
	}
	_, err := db.Exec(ctx, `
		INSERT INTO flow_agent_scopes (flow_run_id, agent_id, project_id, allowed_paths, expires_at)
		SELECT fr.id, $2, fr.project_id, $3, $4
		FROM flow_runs fr
		WHERE fr.id = $1
		ON CONFLICT (flow_run_id, agent_id) DO UPDATE
		SET allowed_paths = EXCLUDED.allowed_paths,
		    expires_at    = EXCLUDED.expires_at,
		    revoked_at    = NULL,
		    updated_at    = now()`, flowRunID, agentID, allowedPaths, expiresAt)
	if err != nil {
		return fmt.Errorf("registrar scope: %w", err)
	}
	return nil
}

// LiberarScopesDelFlow suelta el territorio de todos los agentes del flow sin esperar el TTL.
// Se llama al cancelar: sin esto, un re-grant tras un cancel chocaría contra los scopes de un
// flow que ya no corre.
func LiberarScopesDelFlow(ctx context.Context, db ejecutorSQL, flowRunID uuid.UUID) error {
	_, err := db.Exec(ctx, `
		UPDATE flow_agent_scopes
		SET revoked_at = now(), updated_at = now()
		WHERE flow_run_id = $1 AND revoked_at IS NULL`, flowRunID)
	if err != nil {
		return fmt.Errorf("liberar scopes: %w", err)
	}
	return nil
}
