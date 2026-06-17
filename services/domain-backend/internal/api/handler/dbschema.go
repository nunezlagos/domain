package handler

import (
	"context"
	"fmt"
	"net/http"

	"github.com/jackc/pgx/v5/pgxpool"
)

// SchemaInfo: descripción del schema completo de la DB.
// HU-41.4: alimenta la página /admin/database (DB explorer) del admin.
type SchemaInfo struct {
	Tables     []TableInfo `json:"tables"`
	TotalCount int          `json:"total_count"`
}

type TableInfo struct {
	Name         string       `json:"name"`
	Schema       string       `json:"schema"`
	RowCount     int64        `json:"row_count"`
	SizeBytes    int64        `json:"size_bytes"`
	HasCreatedAt bool         `json:"has_created_at"`
	HasUpdatedAt bool         `json:"has_updated_at"`
	HasStatus    bool         `json:"has_status"`
	Columns      []ColumnInfo `json:"columns"`
	Indexes      []IndexInfo  `json:"indexes"`
	ForeignKeys  []FKInfo     `json:"foreign_keys"`
	Category     string       `json:"category"`
}

type ColumnInfo struct {
	Name         string `json:"name"`
	Type         string `json:"type"`
	IsNullable   bool   `json:"is_nullable"`
	DefaultValue string `json:"default_value,omitempty"`
	IsPrimaryKey bool   `json:"is_primary_key"`
	IsForeignKey bool   `json:"is_foreign_key"`
}

type IndexInfo struct {
	Name     string `json:"name"`
	Columns  string `json:"columns"`
	IsUnique bool   `json:"is_unique"`
	IsPrimary bool  `json:"is_primary"`
}

type FKInfo struct {
	Constraint string `json:"constraint"`
	Columns    string `json:"columns"`
	References  string `json:"references"`
}

// GET /api/v1/admin/db-schema — HU-41.4
// Retorna el schema completo: tablas, columnas, FKs, índices, row counts.
// Usa el pool AUTH (app_admin) para bypassear RLS y ver todas las tablas.
func (a *API) getDBSchema(w http.ResponseWriter, r *http.Request) {
	p, _ := principal(r)
	if p == nil {
		writeError(w, http.StatusUnauthorized, "unauthorized", "")
		return
	}

	pool := a.Pool

	schema, err := loadDBSchema(r.Context(), pool)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "schema_query_failed", err.Error())
		return
	}

	writeData(w, http.StatusOK, schema)
}

func loadDBSchema(ctx context.Context, pool *pgxpool.Pool) (*SchemaInfo, error) {
	info := &SchemaInfo{}

	// 1. Tablas operativas (excluyendo schema_migrations y pg_*)
	rows, err := pool.Query(ctx, `
		SELECT t.table_name
		FROM information_schema.tables t
		WHERE t.table_schema = 'public'
		  AND t.table_type = 'BASE TABLE'
		  AND t.table_name <> 'schema_migrations'
		ORDER BY t.table_name
	`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	for rows.Next() {
		var name string
		if err := rows.Scan(&name); err != nil {
			return nil, err
		}
		info.Tables = append(info.Tables, TableInfo{Name: name, Schema: "public", Category: categorize(name)})
	}
	info.TotalCount = len(info.Tables)

	// 2. Para cada tabla: columnas, FKs, índices, row count
	for i := range info.Tables {
		tbl := &info.Tables[i]
		if err := loadTableInfo(ctx, pool, tbl); err != nil {
			return nil, err
		}
	}

	return info, nil
}

func loadTableInfo(ctx context.Context, pool *pgxpool.Pool, tbl *TableInfo) error {
	// Columnas
	colRows, err := pool.Query(ctx, `
		SELECT
			c.column_name,
			c.data_type || COALESCE('(' || c.character_maximum_length || ')', ''),
			(c.is_nullable = 'YES'),
			COALESCE(c.column_default, ''),
			EXISTS (
				SELECT 1 FROM information_schema.table_constraints tc
				JOIN information_schema.key_column_usage kcu
					ON tc.constraint_name = kcu.constraint_name
				WHERE tc.table_schema = c.table_schema
				  AND tc.table_name = c.table_name
				  AND tc.constraint_type = 'PRIMARY KEY'
				  AND kcu.column_name = c.column_name
			) AS is_pk
		FROM information_schema.columns c
		WHERE c.table_schema = $1 AND c.table_name = $2
		ORDER BY c.ordinal_position
	`, tbl.Schema, tbl.Name)
	if err != nil {
		return err
	}
	defer colRows.Close()
	for colRows.Next() {
		var c ColumnInfo
		if err := colRows.Scan(&c.Name, &c.Type, &c.IsNullable, &c.DefaultValue, &c.IsPrimaryKey); err != nil {
			return err
		}
		tbl.Columns = append(tbl.Columns, c)
		if c.Name == "created_at" {
			tbl.HasCreatedAt = true
		}
		if c.Name == "updated_at" {
			tbl.HasUpdatedAt = true
		}
		if c.Name == "status" {
			tbl.HasStatus = true
		}
	}

	// Foreign keys
	fkRows, err := pool.Query(ctx, `
		SELECT
			tc.constraint_name,
			string_agg(kcu.column_name, ', ' ORDER BY kcu.ordinal_position),
			ccu.table_name || '.' || string_agg(kcu.referenced_column_name, ', ' ORDER BY kcu.ordinal_position)
		FROM information_schema.table_constraints tc
		JOIN information_schema.key_column_usage kcu
			ON tc.constraint_name = kcu.constraint_name
		JOIN information_schema.referential_constraints rc
			ON tc.constraint_name = rc.constraint_name
		JOIN information_schema.constraint_column_usage ccu
			ON tc.constraint_name = ccu.constraint_name AND kcu.referenced_column_name = ccu.column_name
		WHERE tc.table_schema = $1 AND tc.table_name = $2
		  AND tc.constraint_type = 'FOREIGN KEY'
		GROUP BY tc.constraint_name, ccu.table_name
	`, tbl.Schema, tbl.Name)
	if err == nil {
		defer fkRows.Close()
		for fkRows.Next() {
			var fk FKInfo
			if err := fkRows.Scan(&fk.Constraint, &fk.Columns, &fk.References); err == nil {
				tbl.ForeignKeys = append(tbl.ForeignKeys, fk)
			}
		}
		// marcar columnas FK
		for i := range tbl.Columns {
			for _, fk := range tbl.ForeignKeys {
				if containsWord(fk.Columns, tbl.Columns[i].Name) {
					tbl.Columns[i].IsForeignKey = true
				}
			}
		}
	}

	// Índices
	idxRows, err := pool.Query(ctx, `
		SELECT
			i.indexname,
			string_agg(a.attname, ', ' ORDER BY array_position(i.indexkeys, a.attnum)),
			(i.indexdef LIKE '%UNIQUE INDEX%'),
			(i.indexdef LIKE '%_pkey%')
		FROM pg_indexes i
		JOIN pg_class c ON c.relname = i.indexname
		JOIN pg_index ix ON ix.indexrelid = c.oid
		JOIN pg_attribute a ON a.attrelid = ix.indexrelid AND a.attnum = ANY(ix.indkey)
		WHERE i.schemaname = $1 AND i.tablename = $2
		GROUP BY i.indexname, i.indexdef
		ORDER BY i.indexname
	`, tbl.Schema, tbl.Name)
	if err == nil {
		defer idxRows.Close()
		for idxRows.Next() {
			var idx IndexInfo
			if err := idxRows.Scan(&idx.Name, &idx.Columns, &idx.IsUnique, &idx.IsPrimary); err == nil {
				tbl.Indexes = append(tbl.Indexes, idx)
			}
		}
	}

	// Row count
	if err := pool.QueryRow(ctx, fmt.Sprintf(`SELECT count(*) FROM %s.%s`, tbl.Schema, tbl.Name)).Scan(&tbl.RowCount); err != nil {
		tbl.RowCount = 0
	}

	return nil
}

// categorize agrupa tablas por nombre para la UI.
func categorize(name string) string {
	switch {
	case containsAny(name, "user", "auth", "session", "otp", "role", "client"):
		return "core"
	case containsAny(name, "agent", "skill", "prompt", "flow", "crons", "webhook", "mcp"):
		return "resources"
	case containsAny(name, "audit", "log", "observation", "prompt_captured", "dlq", "dead_letter", "activity"):
		return "observability"
	case containsAny(name, "project", "knowledge", "policy", "platform", "cost", "usage", "budget", "config", "system_state"):
		return "system"
	case containsAny(name, "requirement", "user_story", "hu_draft", "proposal", "design", "saga", "sabotage"):
		return "sdd"
	default:
		return "other"
	}
}

func containsAny(s string, subs ...string) bool {
	for _, sub := range subs {
		if contains(s, sub) {
			return true
		}
	}
	return false
}

func contains(s, sub string) bool {
	for i := 0; i+len(sub) <= len(s); i++ {
		if s[i:i+len(sub)] == sub {
			return true
		}
	}
	return false
}

func containsWord(s, word string) bool {
	// CSV split check
	for i := 0; i < len(s); i++ {
		j := i
		for j < len(s) && s[j] != ',' {
			j++
		}
		field := s[i:j]
		// trim spaces
		for len(field) > 0 && field[0] == ' ' {
			field = field[1:]
		}
		if field == word {
			return true
		}
		i = j
	}
	return false
}
