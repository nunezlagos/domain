package handler

import (
	"archive/zip"
	"bytes"
	"compress/gzip"
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/stretchr/testify/require"
)

var fixedTime = time.Date(2026, 6, 13, 12, 0, 0, 0, time.UTC)

func skipIfNoDB(t *testing.T) *pgxpool.Pool {
	t.Helper()
	pool, err := pgxpool.New(context.Background(), "postgres://test:test@localhost:5432/test?sslmode=disable")
	if err != nil {
		t.Skip("requires postgres: ", err)
	}
	if err := pool.Ping(context.Background()); err != nil {
		t.Skip("requires postgres: ", err)
	}
	return pool
}

func TestBuildMetadata_Structure(t *testing.T) {
	orgID := uuid.New()
	now := fixedTime
	buf := &bytes.Buffer{}
	zw := zip.NewWriter(buf)

	err := writeMetadata(context.Background(), zw, orgID, "0.1.0", now)
	require.NoError(t, err)
	zw.Close()

	r, err := zip.NewReader(bytes.NewReader(buf.Bytes()), int64(buf.Len()))
	require.NoError(t, err)
	require.Len(t, r.File, 1)
	require.Equal(t, "metadata.json", r.File[0].Name)

	f, err := r.File[0].Open()
	require.NoError(t, err)
	defer f.Close()
	var meta map[string]any
	require.NoError(t, json.NewDecoder(f).Decode(&meta))
	require.Equal(t, "0.1.0", meta["domain_version"])
	require.Equal(t, orgID.String(), meta["organization_id"])
	require.Equal(t, "1", meta["schema_version"])
	require.Equal(t, "jsonl.gz", meta["format"])
}

func TestStreamTable_Empty(t *testing.T) {
	pool := skipIfNoDB(t)
	ctx := context.Background()
	orgID := uuid.New()
	userID := uuid.New()

	buf := &bytes.Buffer{}
	zw := zip.NewWriter(buf)

	err := streamTable(ctx, pool, zw, "observations", observationsQuery, orgID, userID)
	require.NoError(t, err)
	zw.Close()

	r, err := zip.NewReader(bytes.NewReader(buf.Bytes()), int64(buf.Len()))
	require.NoError(t, err)
	require.Len(t, r.File, 1)
	require.Equal(t, "observations.jsonl.gz", r.File[0].Name)
	require.Less(t, r.File[0].CompressedSize64, uint64(100))
}

func TestStreamTable_ProducesValidGzip(t *testing.T) {
	pool := skipIfNoDB(t)
	ctx := context.Background()
	orgID := uuid.New()
	userID := uuid.New()

	buf := &bytes.Buffer{}
	zw := zip.NewWriter(buf)

	err := streamTable(ctx, pool, zw, "test_table", `SELECT row_to_json(t) FROM (SELECT 1 as id) t WHERE $1 = $1`, orgID, userID)
	require.NoError(t, err)
	zw.Close()

	r, err := zip.NewReader(bytes.NewReader(buf.Bytes()), int64(buf.Len()))
	require.NoError(t, err)
	require.Len(t, r.File, 1)

	f, err := r.File[0].Open()
	require.NoError(t, err)
	defer f.Close()
	gz, err := gzip.NewReader(f)
	require.NoError(t, err)
	defer gz.Close()

	data, err := io.ReadAll(gz)
	require.NoError(t, err)
	require.Contains(t, string(data), `"id":1`)
}

func TestExportHandler_Unauthenticated(t *testing.T) {
	h := &ExportHandler{}
	w := httptest.NewRecorder()
	r := httptest.NewRequest("GET", "/api/v1/export", nil)

	h.ExportGET(w, r)
	require.Equal(t, http.StatusUnauthorized, w.Code)
}
