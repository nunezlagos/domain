package lifecycle

import (
	"context"
	"errors"
	"fmt"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	"nunezlagos/domain/internal/audit"
)

var (
	ErrAlreadyErased         = errors.New("already_erased")
	ErrUserNotFound          = errors.New("user_not_found")
	ErrTransferOwnershipFirst = errors.New("transfer_ownership_first")
)

// EraseResult cuenta rows afectadas por tabla.
type EraseResult struct {
	UserID         uuid.UUID         `json:"user_id"`
	UpdatedRows    map[string]int64  `json:"updated_rows"`
	RevokedAPIKeys int64             `json:"revoked_api_keys"`
}

// EraseUser ejecuta el derecho al olvido GDPR Art. 17 sobre un user.
//
// Lógica:
//   - Verifica que el user no esté ya erased
//   - (Fase D clean REQ-21.6) Se omite la validación de "owner de org
//     con otros miembros": la tabla `organizations` y `organization_members`
//     se dropean en Fase C. Single-org implica que no aplica esa lógica.
//   - En una transacción:
//     * users PII → NULL/anonimizado, is_erased=TRUE, erased_at=NOW
//     * observations/sessions/prompts/knowledge_docs/agent_runs created_by/user_id → NULL
//     * api_keys revoked_at = NOW
//   - audit_log se mantiene intacto (legal hold compliance)
//   - actorID puede ser el mismo user (self-service) o admin
func (s *Service) EraseUser(ctx context.Context, userID, actorID uuid.UUID, reason string) (*EraseResult, error) {
	tx, err := s.Pool.BeginTx(ctx, pgx.TxOptions{})
	if err != nil {
		return nil, fmt.Errorf("begin tx: %w", err)
	}
	defer tx.Rollback(ctx)

	// 1. Verificar existencia + no-erased
	var isErased bool
	err = tx.QueryRow(ctx,
		`SELECT is_erased FROM users WHERE id = $1`, userID).Scan(&isErased)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrUserNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("lookup user: %w", err)
	}
	if isErased {
		return nil, ErrAlreadyErased
	}

	res := &EraseResult{UserID: userID, UpdatedRows: make(map[string]int64)}

	// 3. Anonimizar PII en users
	tag, err := tx.Exec(ctx, `
		UPDATE users SET
			email = 'erased+' || id::text || '@example.invalid',
			rut = NULL,
			phone = NULL,
			name = NULL,
			is_erased = TRUE,
			erased_at = NOW()
		WHERE id = $1
	`, userID)
	if err != nil {
		return nil, fmt.Errorf("update users: %w", err)
	}
	res.UpdatedRows["users"] = tag.RowsAffected()

	// 4. Cascade NULL en tablas con created_by/user_id
	for _, q := range []struct {
		table string
		sql   string
	}{
		{"observations", "UPDATE observations SET created_by = NULL WHERE created_by = $1"},
		{"sessions", "UPDATE sessions SET user_id = NULL WHERE user_id = $1"},
		{"prompts", "UPDATE prompts SET created_by = NULL WHERE created_by = $1"},
		{"knowledge_docs", "UPDATE knowledge_docs SET created_by = NULL WHERE created_by = $1"},
		{"agent_runs", "UPDATE agent_runs SET user_id = NULL WHERE user_id = $1"},
	} {
		tag, err := tx.Exec(ctx, q.sql, userID)
		if err != nil {
			// Si la tabla no existe (entorno incompleto), skip + cuenta 0.
			res.UpdatedRows[q.table] = 0
			continue
		}
		res.UpdatedRows[q.table] = tag.RowsAffected()
	}

	// 5. Revocar api_keys del user
	tag, err = tx.Exec(ctx,
		`UPDATE api_keys SET revoked_at = NOW() WHERE user_id = $1 AND revoked_at IS NULL`,
		userID)
	if err == nil {
		res.RevokedAPIKeys = tag.RowsAffected()
	}

	if err := tx.Commit(ctx); err != nil {
		return nil, fmt.Errorf("commit: %w", err)
	}

	// 6. Audit log fuera de la tx (best-effort, no bloquea el erase)
	if s.Audit != nil {
		audit.RecordOrLog(ctx, s.Audit, audit.Event{
			ActorID:    &actorID,
			ActorType:  audit.ActorUser,
			Action:     "user.erased",
			EntityType: "user",
			EntityID:   &userID,
			NewValues: map[string]any{
				"reason":        reason,
				"updated_rows":  res.UpdatedRows,
				"revoked_keys":  res.RevokedAPIKeys,
			},
		})
	}

	return res, nil
}
