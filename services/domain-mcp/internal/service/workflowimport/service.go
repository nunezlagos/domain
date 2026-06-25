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
	"github.com/jackc/pgx/v5/pgxpool"
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

		var existingHash string
		var existingStatus string
		err := s.Pool.QueryRow(ctx,
			`SELECT content_hash, status FROM project_imported_workflow_files
			 WHERE project_id = $1 AND rel_path = $2`,
			in.ProjectID, df.RelPath,
		).Scan(&existingHash, &existingStatus)
		if err == nil && existingHash == df.ContentHash {
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

		_, err = s.Pool.Exec(ctx, `
			INSERT INTO project_imported_workflow_files
			  (project_id, source_tool, rel_path, original_content,
			   content_hash, size_bytes, status, replaced_with, replaced_at)
			VALUES ($1,$2,$3,$4,$5,$6,$7::varchar,$8::text, CASE WHEN $7::varchar = 'replaced' THEN now() ELSE NULL END)
			ON CONFLICT (project_id, rel_path) DO UPDATE
			SET original_content = EXCLUDED.original_content,
			    content_hash     = EXCLUDED.content_hash,
			    size_bytes       = EXCLUDED.size_bytes,
			    status           = EXCLUDED.status,
			    replaced_with    = EXCLUDED.replaced_with,
			    replaced_at      = EXCLUDED.replaced_at,
			    updated_at       = now()`,
			in.ProjectID, df.SourceTool, df.RelPath, df.Content,
			df.ContentHash, df.SizeBytes, newStatus, stub,
		)
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
	var f ImportedFile
	err := s.Pool.QueryRow(ctx, `
		SELECT id, source_tool, rel_path, original_content, status, replaced_at
		FROM project_imported_workflow_files
		WHERE ($1::uuid IS NULL AND project_id IS NULL OR project_id = $1)
		  AND rel_path = $2`,
		projectID, relPath,
	).Scan(&f.ID, &f.SourceTool, &f.RelPath, &f.OriginalContent, &f.Status, &f.ReplacedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return ErrFileNotFound
	}
	if err != nil {
		return fmt.Errorf("lookup: %w", err)
	}

	if f.Status == StatusRestored {
		return nil
	}

	abs := filepath.Join(projectRoot, relPath)
	if err := os.WriteFile(abs, []byte(f.OriginalContent), 0o644); err != nil {
		return fmt.Errorf("write original: %w", err)
	}

	_, err = s.Pool.Exec(ctx,
		`UPDATE project_imported_workflow_files
		 SET status = $1, restored_at = now(), updated_at = now()
		 WHERE id = $2`,
		StatusRestored, f.ID,
	)
	return err
}

// List devuelve los archivos importados de un proyecto.
func (s *Service) List(ctx context.Context, projectID *uuid.UUID) ([]ImportedFile, error) {
	rows, err := s.Pool.Query(ctx, `
		SELECT id, project_id, source_tool, rel_path,
		       original_content, content_hash, size_bytes, status,
		       replaced_with, replaced_at, restored_at, created_at, updated_at
		FROM project_imported_workflow_files
		WHERE ($1::uuid IS NULL AND project_id IS NULL OR project_id = $1)
		ORDER BY rel_path ASC`,
		projectID,
	)
	if err != nil {
		return nil, fmt.Errorf("list: %w", err)
	}
	defer rows.Close()
	var out []ImportedFile
	for rows.Next() {
		var f ImportedFile
		if err := rows.Scan(&f.ID, &f.ProjectID, &f.SourceTool,
			&f.RelPath, &f.OriginalContent, &f.ContentHash, &f.SizeBytes, &f.Status,
			&f.ReplacedWith, &f.ReplacedAt, &f.RestoredAt, &f.CreatedAt, &f.UpdatedAt); err != nil {
			return nil, fmt.Errorf("scan: %w", err)
		}
		out = append(out, f)
	}
	return out, rows.Err()
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
