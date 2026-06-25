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

	if err := writeMetadata(ctx, zw, orgID, version, now); err != nil {
		return fmt.Errorf("metadata: %w", err)
	}

	tables := []exportTable{
		{"observations", observationsQuery, false},
		{"prompts", promptsQuery, false},
		{"knowledge_docs", knowledgeDocsQuery, false},
		{"skills", skillsQuery, false},
		{"agents", agentsQuery, false},
		{"flows", flowsQuery, false},
		{"flow_runs", flowRunsQuery, false},
		{"audit_log", auditLogQuery, true},
	}

	for _, tbl := range tables {
		var args []any
		if tbl.needsUser {
			args = []any{userID}
		}
		if err := streamTable(ctx, pool, zw, tbl.name, tbl.query, args...); err != nil {
			return fmt.Errorf("%s: %w", tbl.name, err)
		}
	}
	return nil
}

type exportTable struct {
	name      string
	query     string
	needsUser bool
}

const observationsQuery = `SELECT row_to_json(t) FROM knowledge_observations t WHERE deleted_at IS NULL`
const promptsQuery = `SELECT row_to_json(t) FROM prompts t`
const knowledgeDocsQuery = `SELECT row_to_json(t) FROM knowledge_docs t`
const skillsQuery = `SELECT row_to_json(t) FROM skills t`
const agentsQuery = `SELECT row_to_json(t) FROM agents t WHERE deleted_at IS NULL`
const flowsQuery = `SELECT row_to_json(t) FROM flows t WHERE deleted_at IS NULL`
const flowRunsQuery = `SELECT row_to_json(t) FROM flow_runs t`
const auditLogQuery = `SELECT row_to_json(t) FROM audit_log t WHERE actor_id = $1`

func streamTable(ctx context.Context, pool *pgxpool.Pool, zw *zip.Writer, name, query string, args ...any) error {
	fw, err := zw.CreateHeader(&zip.FileHeader{
		Name:   name + ".jsonl.gz",
		Method: zip.Deflate,
	})
	if err != nil {
		return fmt.Errorf("create zip entry: %w", err)
	}

	gz := gzip.NewWriter(fw)
	defer gz.Close()

	rows, err := pool.Query(ctx, query, args...)
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
		"domain_version":  version,
		"exported_at":     now.Format(time.RFC3339),
		"organization_id": orgID.String(),
		"schema_version":  "1",
		"tables_exported": []string{
			"observations", "prompts", "knowledge_docs",
			"skills", "agents", "flows", "flow_runs", "audit_log",
		},
		"format": "jsonl.gz",
	}
	return json.NewEncoder(fw).Encode(meta)
}
