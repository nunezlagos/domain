package handler

import (
	"archive/zip"
	"compress/gzip"
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
)

type ExportHandler struct {
	Pool    *pgxpool.Pool
	Logger  *slog.Logger
	Now     func() time.Time
	Version string
}

func (h *ExportHandler) ExportGET(w http.ResponseWriter, r *http.Request) {
	p, ok := principal(r)
	if !ok || p == nil {
		writeError(w, http.StatusUnauthorized, "unauthorized", "authentication required")
		return
	}
	orgID, err := uuid.Parse(p.OrganizationID)
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid_org", "invalid organization")
		return
	}
	userID, err := uuid.Parse(p.UserID)
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid_user", "invalid user")
		return
	}

	now := h.Now()
	filename := fmt.Sprintf("domain-export-%s-%s.zip", orgID.String()[:8], now.Format("20060102"))
	w.Header().Set("Content-Type", "application/zip")
	w.Header().Set("Content-Disposition", fmt.Sprintf(`attachment; filename="%s"`, filename))

	zw := zip.NewWriter(w)
	defer zw.Close()

	err = StreamOrgExport(r.Context(), h.Pool, orgID, userID, zw, h.Version, now)
	if err != nil {
		h.Logger.Error("export failed",
			slog.String("org_id", orgID.String()),
			slog.Any("error", err),
		)
		return
	}

	h.Logger.Info("export completed",
		slog.String("org_id", orgID.String()),
		slog.String("user_id", userID.String()),
	)
}

func StreamOrgExport(ctx context.Context, pool *pgxpool.Pool, orgID, userID uuid.UUID, zw *zip.Writer, version string, now time.Time) error {
	// Metadata first (logically; zip order doesn't matter)
	if err := writeMetadata(ctx, zw, orgID, version, now); err != nil {
		return fmt.Errorf("metadata: %w", err)
	}

	tables := []exportTable{
		{"observations", observationsQuery},
		{"prompts", promptsQuery},
		{"knowledge_docs", knowledgeDocsQuery},
		{"skills", skillsQuery},
		{"agents", agentsQuery},
		{"flows", flowsQuery},
		{"flow_runs", flowRunsQuery},
		{"audit_log", auditLogQuery},
	}

	for _, tbl := range tables {
		if err := streamTable(ctx, pool, zw, tbl.name, tbl.query, orgID, userID); err != nil {
			return fmt.Errorf("%s: %w", tbl.name, err)
		}
	}
	return nil
}

type exportTable struct {
	name  string
	query string
}

const observationsQuery = `SELECT row_to_json(t) FROM observations t WHERE organization_id = $1 AND deleted_at IS NULL`
const promptsQuery = `SELECT row_to_json(t) FROM prompts t WHERE organization_id = $1`
const knowledgeDocsQuery = `SELECT row_to_json(t) FROM knowledge_docs t WHERE organization_id = $1`
const skillsQuery = `SELECT row_to_json(t) FROM skills t WHERE organization_id = $1`
const agentsQuery = `SELECT row_to_json(t) FROM agents t WHERE organization_id = $1 AND deleted_at IS NULL`
const flowsQuery = `SELECT row_to_json(t) FROM flows t WHERE organization_id = $1 AND deleted_at IS NULL`
const flowRunsQuery = `SELECT row_to_json(t) FROM flow_runs t WHERE organization_id = $1`
const auditLogQuery = `SELECT row_to_json(t) FROM audit_log t WHERE actor_id = $1 OR organization_id = $2`

func streamTable(ctx context.Context, pool *pgxpool.Pool, zw *zip.Writer, name, query string, orgID, userID uuid.UUID) error {
	fw, err := zw.CreateHeader(&zip.FileHeader{
		Name:   name + ".jsonl.gz",
		Method: zip.Deflate,
	})
	if err != nil {
		return fmt.Errorf("create zip entry: %w", err)
	}

	gz := gzip.NewWriter(fw)
	defer gz.Close()

	rows, err := pool.Query(ctx, query, orgID, userID)
	if err != nil {
		return fmt.Errorf("query %s: %w", name, err)
	}
	defer rows.Close()

	var buf []byte
	for rows.Next() {
		if err := rows.Scan(&buf); err != nil {
			return fmt.Errorf("scan %s: %w", name, err)
		}
		if _, err := gz.Write(buf); err != nil {
			return fmt.Errorf("write %s: %w", name, err)
		}
		if _, err := gz.Write([]byte("\n")); err != nil {
			return err
		}
	}
	return rows.Err()
}

func writeMetadata(ctx context.Context, zw *zip.Writer, orgID uuid.UUID, version string, now time.Time) error {
	fw, err := zw.Create("metadata.json")
	if err != nil {
		return err
	}
	meta := map[string]any{
		"domain_version": version,
		"exported_at":    now.Format(time.RFC3339),
		"organization_id": orgID.String(),
		"schema_version": "1",
		"tables_exported": []string{
			"observations", "prompts", "knowledge_docs",
			"skills", "agents", "flows", "flow_runs", "audit_log",
		},
		"format": "jsonl.gz",
	}
	return json.NewEncoder(fw).Encode(meta)
}
