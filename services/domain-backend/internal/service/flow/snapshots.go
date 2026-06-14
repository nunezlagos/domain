// issue-09.11 reproducibility-snapshots — captura input/output exacto de cada
// step para replay determinístico y debugging.
//
// Tabla flow_run_step_snapshots (migration 000064) guarda blob input + output
// + duration. Permite re-correr un run en modo dry-run con los mismos
// inputs y comparar outputs (regresión detector).
package flow

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

// DefaultSnapshotRetention define cuánto tiempo se conservan snapshots antes
// de ser podados por PruneSnapshots.
const DefaultSnapshotRetention = 30 * 24 * time.Hour // 30 días

// StepSnapshot captura el I/O exacto de un step ejecutado.
type StepSnapshot struct {
	ID         uuid.UUID       `json:"id"`
	StepID     uuid.UUID       `json:"step_id"`
	RunID      uuid.UUID       `json:"run_id"`
	StepKey    string          `json:"step_key"`
	Input      json.RawMessage `json:"input"`
	Output     json.RawMessage `json:"output,omitempty"`
	Error      *string         `json:"error,omitempty"`
	DurationMs int64           `json:"duration_ms"`
	CapturedAt time.Time       `json:"captured_at"`
}

// SnapshotStore persiste snapshots para replay/regresión.
type SnapshotStore struct {
	Pool *pgxpool.Pool
}

var ErrSnapshotNotFound = errors.New("snapshot not found")

// Save persiste el snapshot al completar (success o failure) de un step.
func (s *SnapshotStore) Save(ctx context.Context, snap *StepSnapshot) error {
	if s.Pool == nil {
		return nil // no-op para tests
	}
	_, err := s.Pool.Exec(ctx, `
		INSERT INTO flow_run_step_snapshots
		  (id, step_id, run_id, step_key, input, output, error, duration_ms)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8)
		ON CONFLICT (step_id) DO UPDATE SET
		  output = EXCLUDED.output,
		  error = EXCLUDED.error,
		  duration_ms = EXCLUDED.duration_ms,
		  captured_at = now()`,
		snap.ID, snap.StepID, snap.RunID, snap.StepKey,
		snap.Input, snap.Output, snap.Error, snap.DurationMs,
	)
	if err != nil {
		return fmt.Errorf("upsert snapshot: %w", err)
	}
	return nil
}

// GetByStep retorna el snapshot de un step específico.
func (s *SnapshotStore) GetByStep(ctx context.Context, stepID uuid.UUID) (*StepSnapshot, error) {
	var snap StepSnapshot
	err := s.Pool.QueryRow(ctx, `
		SELECT id, step_id, run_id, step_key, input, output, error, duration_ms, captured_at
		FROM flow_run_step_snapshots WHERE step_id = $1`,
		stepID,
	).Scan(&snap.ID, &snap.StepID, &snap.RunID, &snap.StepKey,
		&snap.Input, &snap.Output, &snap.Error, &snap.DurationMs, &snap.CapturedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrSnapshotNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("get snapshot: %w", err)
	}
	return &snap, nil
}

// ListByRun devuelve todos los snapshots de un run ordenados por captured_at.
func (s *SnapshotStore) ListByRun(ctx context.Context, runID uuid.UUID) ([]StepSnapshot, error) {
	rows, err := s.Pool.Query(ctx, `
		SELECT id, step_id, run_id, step_key, input, output, error, duration_ms, captured_at
		FROM flow_run_step_snapshots
		WHERE run_id = $1
		ORDER BY captured_at ASC`,
		runID,
	)
	if err != nil {
		return nil, fmt.Errorf("list snapshots: %w", err)
	}
	defer rows.Close()

	var out []StepSnapshot
	for rows.Next() {
		var snap StepSnapshot
		if err := rows.Scan(&snap.ID, &snap.StepID, &snap.RunID, &snap.StepKey,
			&snap.Input, &snap.Output, &snap.Error, &snap.DurationMs, &snap.CapturedAt); err != nil {
			return nil, fmt.Errorf("scan: %w", err)
		}
		out = append(out, snap)
	}
	return out, rows.Err()
}

// CompareForReplay compara dos snapshots del mismo step_key (uno guardado
// vs uno nuevo de un replay). Devuelve true si los outputs son idénticos.
// Usado por regression detector — alerta si el flow ahora produce output
// distinto con los mismos inputs (cambio de comportamiento).
func CompareForReplay(original, replay *StepSnapshot) (bool, string) {
	if original.StepKey != replay.StepKey {
		return false, "step_key differs"
	}
	if (original.Error == nil) != (replay.Error == nil) {
		return false, "error status differs"
	}
	if original.Error != nil && *original.Error != *replay.Error {
		return false, "error message differs: " + *original.Error + " vs " + *replay.Error
	}
	if string(original.Output) != string(replay.Output) {
		return false, "output differs"
	}
	return true, ""
}

// snapshotOutputCompressed es la estructura interna para output comprimido
// dentro del JSONB output.
type snapshotOutputCompressed struct {
	Compressed bool   `json:"compressed"`
	Data       string `json:"data"` // base64 del gzip comprimido
}

// SaveSnapshot guarda el snapshot comprimiendo el output con gzip.
// Reusa CompressOutput (definida en internal/runner/flow/durable.go).
// El output comprimido se almacena como base64 dentro del JSONB.
func (s *SnapshotStore) SaveSnapshot(ctx context.Context, snap *StepSnapshot, compressFn func(any) ([]byte, int, error)) error {
	if snap.Output != nil && compressFn != nil {
		var raw any
		if err := json.Unmarshal(snap.Output, &raw); err == nil {
			compressed, _, err := compressFn(raw)
			if err == nil {
				wrapped := snapshotOutputCompressed{
					Compressed: true,
					Data:       base64.StdEncoding.EncodeToString(compressed),
				}
				wrappedJSON, _ := json.Marshal(wrapped)
				snap.Output = wrappedJSON
			}
		}
	}
	return s.Save(ctx, snap)
}

// GetSnapshot recupera el snapshot y descomprime el output si estaba comprimido.
// Reusa DecompressOutput (definida en internal/runner/flow/durable.go).
func (s *SnapshotStore) GetSnapshot(ctx context.Context, stepID uuid.UUID, decompressFn func([]byte) ([]byte, error)) (*StepSnapshot, error) {
	snap, err := s.GetByStep(ctx, stepID)
	if err != nil {
		return nil, err
	}
	if snap.Output != nil && decompressFn != nil {
		var wrapped snapshotOutputCompressed
		if err := json.Unmarshal(snap.Output, &wrapped); err == nil && wrapped.Compressed {
			decoded, err := base64.StdEncoding.DecodeString(wrapped.Data)
			if err == nil {
				decompressed, err := decompressFn(decoded)
				if err == nil {
					snap.Output = decompressed
				}
			}
		}
	}
	return snap, nil
}

// ListSnapshots devuelve todos los snapshots de un run con outputs descomprimidos.
func (s *SnapshotStore) ListSnapshots(ctx context.Context, runID uuid.UUID, decompressFn func([]byte) ([]byte, error)) ([]StepSnapshot, error) {
	snapshots, err := s.ListByRun(ctx, runID)
	if err != nil {
		return nil, err
	}
	for i := range snapshots {
		if snapshots[i].Output != nil && decompressFn != nil {
			var wrapped snapshotOutputCompressed
			if err := json.Unmarshal(snapshots[i].Output, &wrapped); err == nil && wrapped.Compressed {
				decoded, err := base64.StdEncoding.DecodeString(wrapped.Data)
				if err == nil {
					decompressed, err := decompressFn(decoded)
					if err == nil {
						snapshots[i].Output = decompressed
					}
				}
			}
		}
	}
	return snapshots, nil
}

// PruneSnapshots elimina snapshots anteriores a la fecha dada.
// Retorna la cantidad de filas eliminadas.
func (s *SnapshotStore) PruneSnapshots(ctx context.Context, before time.Time) (int64, error) {
	if s.Pool == nil {
		return 0, nil // no-op para tests
	}
	tag, err := s.Pool.Exec(ctx, `
		DELETE FROM flow_run_step_snapshots
		WHERE captured_at < $1`,
		before,
	)
	if err != nil {
		return 0, fmt.Errorf("prune snapshots: %w", err)
	}
	return tag.RowsAffected(), nil
}
