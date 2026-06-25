package lifecycle

import (
	"context"
	"errors"
	"fmt"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	"nunezlagos/domain/internal/audit"
	"nunezlagos/domain/internal/service/lifecycle/lifecycledb"
)

var (
	ErrAlreadyErased          = errors.New("already_erased")
	ErrUserNotFound           = errors.New("user_not_found")
	ErrTransferOwnershipFirst = errors.New("transfer_ownership_first")
)

// EraseResult cuenta rows afectadas por tabla.
type EraseResult struct {
	UserID         uuid.UUID        `json:"user_id"`
	UpdatedRows    map[string]int64 `json:"updated_rows"`
	RevokedAPIKeys int64            `json:"revoked_api_keys"`
}

// EraseUser ejecuta el derecho al olvido GDPR Art. 17 sobre un user.
//
// Lógica:
//   - Verifica que el user no esté ya erased
//   - (Fase D clean REQ-21.6) Se omite la validación de "owner de org
//     con otros miembros": la tabla `organizations` y `organization_members`
//     se dropean en Fase C. Single-org implica que no aplica esa lógica.
//   - En una transacción:
//   - users PII → NULL/anonimizado, is_erased=TRUE, erased_at=NOW
//   - observations/sessions/prompts/knowledge_docs/agent_runs created_by/user_id → NULL
//   - auth_api_keys revoked_at = NOW
//   - audit_log se mantiene intacto (legal hold compliance)
//   - actorID puede ser el mismo user (self-service) o admin
func (s *Service) EraseUser(ctx context.Context, userID, actorID uuid.UUID, reason string) (*EraseResult, error) {
	tx, err := s.Pool.BeginTx(ctx, pgx.TxOptions{})
	if err != nil {
		return nil, fmt.Errorf("begin tx: %w", err)
	}
	defer tx.Rollback(ctx)

	qtx := lifecycledb.New(tx)

	row, err := qtx.GetUserIsErased(ctx, userID)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrUserNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("lookup user: %w", err)
	}
	if row.IsErased {
		return nil, ErrAlreadyErased
	}

	res := &EraseResult{UserID: userID, UpdatedRows: make(map[string]int64)}

	usersAffected, err := qtx.EraseUserPII(ctx, userID)
	if err != nil {
		return nil, fmt.Errorf("update users: %w", err)
	}
	res.UpdatedRows["users"] = usersAffected

	if n, err := qtx.AnonymizeObservations(ctx, &userID); err == nil {
		res.UpdatedRows["observations"] = n
	}
	if n, err := qtx.AnonymizePrompts(ctx, &userID); err == nil {
		res.UpdatedRows["prompts"] = n
	}
	if n, err := qtx.AnonymizeKnowledgeDocs(ctx, &userID); err == nil {
		res.UpdatedRows["knowledge_docs"] = n
	}
	if n, err := qtx.AnonymizeAgentRuns(ctx, &userID); err == nil {
		res.UpdatedRows["agent_runs"] = n
	}

	if n, err := qtx.RevokeUserAPIKeys(ctx, userID); err == nil {
		res.RevokedAPIKeys = n
	}

	if err := tx.Commit(ctx); err != nil {
		return nil, fmt.Errorf("commit: %w", err)
	}


	if s.Audit != nil {
		audit.RecordOrLog(ctx, s.Audit, audit.Event{
			ActorID:    &actorID,
			ActorType:  audit.ActorUser,
			Action:     "user.erased",
			EntityType: "user",
			EntityID:   &userID,
			NewValues: map[string]any{
				"reason":       reason,
				"updated_rows": res.UpdatedRows,
				"revoked_keys": res.RevokedAPIKeys,
			},
		})
	}

	return res, nil
}
