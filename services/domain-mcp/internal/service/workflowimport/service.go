//go:generate go run github.com/sqlc-dev/sqlc/cmd/sqlc@v1.31.1 generate
package workflowimport

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/jackc/pgx/v5/pgxpool"

	"nunezlagos/domain/internal/service/workflowimport/workflowimportdb"
	"nunezlagos/domain/internal/store/txctx"
)

// ImportedFile refleja una fila de imported_workflow_files.
type ImportedFile struct {
	ID              uuid.UUID  `json:"id"`
	ProjectID       *uuid.UUID `json:"project_id,omitempty"`
	SourceTool      string     `json:"source_tool"`
	RelPath         string     `json:"rel_path"`
	OriginalContent string     `json:"original_content"`
	ContentHash     string     `json:"content_hash"`
	SizeBytes       int64      `json:"size_bytes"`
	Status          string     `json:"status"`
	ReplacedWith    *string    `json:"replaced_with,omitempty"`
	ReplacedAt      *time.Time `json:"replaced_at,omitempty"`
	RestoredAt      *time.Time `json:"restored_at,omitempty"`
	CreatedAt       time.Time  `json:"created_at"`
	UpdatedAt       time.Time  `json:"updated_at"`
}

const (
	StatusDetected = "detected"
	StatusBackedUp = "backed_up"
	StatusReplaced = "replaced"
	StatusRestored = "restored"
)

var (
	ErrFileNotFound  = errors.New("imported file not found")
	ErrAlreadyImport = errors.New("file already imported (use re-import to refresh)")
)

// Service maneja el ciclo Import → BackUp → Replace → Restore.
type Service struct {
	Pool *pgxpool.Pool
}

// ImportInput parámetros de Import.
type ImportInput struct {
	ProjectID    *uuid.UUID
	OrgID        *uuid.UUID
	ProjectRoot  string
	StubTemplate string // contenido a escribir en el archivo .md tras backup
	WriteStub    bool   // si false, solo backup; el archivo original queda intacto
}

// ImportReport agrega counts del import.
type ImportReport struct {
	Detected []DetectedFile `json:"detected"`
	BackedUp int            `json:"backed_up"`
	Replaced int            `json:"replaced"`
	Skipped  int            `json:"skipped"`
	Errors   []string       `json:"errors,omitempty"`
}

func (s *Service) q(ctx context.Context) *workflowimportdb.Queries {
	if tx := txctx.TxFromContext(ctx); tx != nil {
		return workflowimportdb.New(tx)
	}
	return workflowimportdb.New(s.Pool)
}

func timestamptzToPtr(t pgtype.Timestamptz) *time.Time {
	if !t.Valid {
		return nil
	}
	return &t.Time
}

// Import escanea el proyecto y guarda los .md detectados en BD (status=
// backed_up). Si in.WriteStub=true, sobrescribe el .md con StubTemplate.
func (s *Service) Import(ctx context.Context, in ImportInput) (*ImportReport, error) {
	scanner := &Scanner{ProjectRoot: in.ProjectRoot}
	files, err := scanner.Detect(true)
	if err != nil {
		return nil, fmt.Errorf("detect: %w", err)
	}

	rep := &ImportReport{Detected: files}

	for _, df := range files {
		existing, err := s.q(ctx).GetFileByProjectAndPath(ctx, workflowimportdb.GetFileByProjectAndPathParams{
			ProjectID: in.ProjectID,
			RelPath:   df.RelPath,
		})
		if err == nil && existing.ContentHash == df.ContentHash {
			rep.Skipped++
			continue
		}

		newStatus := StatusBackedUp
		var stub *string
		if in.WriteStub && in.StubTemplate != "" {
			s2 := in.StubTemplate
			stub = &s2
			newStatus = StatusReplaced
		}

		_, err = s.q(ctx).UpsertFile(ctx, workflowimportdb.UpsertFileParams{
			ProjectID:       in.ProjectID,
			SourceTool:      df.SourceTool,
			RelPath:         df.RelPath,
			OriginalContent: df.Content,
			ContentHash:     df.ContentHash,
			SizeBytes:       df.SizeBytes,
			Status:          newStatus,
			ReplacedWith:    stub,
		})
		if err != nil {
			rep.Errors = append(rep.Errors, fmt.Sprintf("upsert %s: %v", df.RelPath, err))
			continue
		}
		rep.BackedUp++

		if in.WriteStub && in.StubTemplate != "" {
			abs := filepath.Join(in.ProjectRoot, df.RelPath)
			if err := os.WriteFile(abs, []byte(in.StubTemplate), 0o644); err != nil {
				rep.Errors = append(rep.Errors, fmt.Sprintf("write stub %s: %v", df.RelPath, err))
				continue
			}
			rep.Replaced++
		}
	}
	return rep, nil
}

// Restore reescribe en disco el original guardado para un archivo y marca
// status=restored. Idempotente: si ya está restored, no-op.
func (s *Service) Restore(ctx context.Context, projectID *uuid.UUID, relPath, projectRoot string) error {
	row, err := s.q(ctx).GetFileByRelPath(ctx, workflowimportdb.GetFileByRelPathParams{
		ProjectID: projectID,
		RelPath:   relPath,
	})
	if errors.Is(err, pgx.ErrNoRows) {
		return ErrFileNotFound
	}
	if err != nil {
		return fmt.Errorf("lookup: %w", err)
	}

	if row.Status == StatusRestored {
		return nil
	}

	abs := filepath.Join(projectRoot, relPath)
	if err := os.WriteFile(abs, []byte(row.OriginalContent), 0o644); err != nil {
		return fmt.Errorf("write original: %w", err)
	}

	return s.q(ctx).SetFileRestored(ctx, workflowimportdb.SetFileRestoredParams{
		ID:     row.ID,
		Status: StatusRestored,
	})
}

// List devuelve los archivos importados de un proyecto.
func (s *Service) List(ctx context.Context, projectID *uuid.UUID) ([]ImportedFile, error) {
	rows, err := s.q(ctx).ListProjectFiles(ctx, projectID)
	if err != nil {
		return nil, fmt.Errorf("list: %w", err)
	}

	out := make([]ImportedFile, 0, len(rows))
	for _, r := range rows {
		out = append(out, ImportedFile{
			ID:              r.ID,
			ProjectID:       r.ProjectID,
			SourceTool:      r.SourceTool,
			RelPath:         r.RelPath,
			OriginalContent: r.OriginalContent,
			ContentHash:     r.ContentHash,
			SizeBytes:       r.SizeBytes,
			Status:          r.Status,
			ReplacedWith:    r.ReplacedWith,
			ReplacedAt:      timestamptzToPtr(r.ReplacedAt),
			RestoredAt:      timestamptzToPtr(r.RestoredAt),
			CreatedAt:       r.CreatedAt,
			UpdatedAt:       r.UpdatedAt,
		})
	}
	return out, nil
}

// DefaultStub es el template estándar que se escribe en lugar de los
// archivos .md originales: instrucciones mínimas que apuntan al MCP de
// Domain.
const DefaultStub = "# Domain MCP\n\n" +
	"Este archivo fue reemplazado por `domain init` — el contexto del proyecto\n" +
	"para tu agente IA (Claude Code / OpenCode / Cursor) ahora vive en el MCP\n" +
	"de Domain, no acá.\n\n" +
	"El contenido original está backed up en BD (tabla `imported_workflow_files`).\n" +
	"Para restaurar: `domain workflow restore --path <este archivo>`.\n\n" +
	"## Cómo trabajar\n\n" +
	"1. Asegurate de que el MCP de Domain está conectado a tu agente IA:\n" +
	"   ```\n" +
	"   domain setup claude-code --api-key sk_... --base-url http://localhost:8000\n" +
	"   ```\n" +
	"2. Escribí tu prompt normalmente. Domain MCP intercepta:\n" +
	"   - Si es chat/idea: respondés directamente.\n" +
	"   - Si es feature/fix/refactor/doc/rfc: arranca el wizard interactivo\n" +
	"     `domain_hu_create_start` (issue-04.7) → confección HU + Gherkin con\n" +
	"     preguntas dirigidas + upload de screenshots opcional.\n" +
	"3. Tras confirmar el spec, el agente IA implementa siguiendo TDD strict\n" +
	"   (rules `.claude/rules/*` migrated a BD platform_policies).\n\n" +
	"## Recuperar conventions específicas\n\n" +
	"Las policies del proyecto viven en `platform_policies` (issue-01.8). Para\n" +
	"consultarlas:\n\n" +
	"```\n" +
	"domain policy list\n" +
	"domain policy get <slug>\n" +
	"```\n\n" +
	"O via MCP tool `domain_policy_get(slug)` desde tu agente.\n"

// MetadataJSON helper para serializar metadata.
func MetadataJSON(m map[string]any) []byte {
	if m == nil {
		return []byte("{}")
	}
	b, _ := json.Marshal(m)
	return b
}
