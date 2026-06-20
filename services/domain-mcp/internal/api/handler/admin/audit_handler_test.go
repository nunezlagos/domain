package admin_test

import (
	"net/http/httptest"
	"testing"
	"time"

	"nunezlagos/domain/internal/api/handler/admin"

	"github.com/stretchr/testify/require"
)

func TestParseQueryParams_Defaults(t *testing.T) {
	r := httptest.NewRequest("GET", "/admin/audit?org_id=00000000-0000-0000-0000-000000000001", nil)
	params := admin.ParseQueryParams(r)
	require.Equal(t, "00000000-0000-0000-0000-000000000001", params.OrgID)
	require.Equal(t, 50, params.Limit)
}

func TestParseQueryParams_WithAll(t *testing.T) {
	r := httptest.NewRequest("GET", "/admin/audit?org_id=a&since=2026-01-01T00:00:00Z&until=2026-06-01T00:00:00Z&action=*.delete&resource=observation/abc&cursor=abc123&limit=100", nil)
	params := admin.ParseQueryParams(r)
	require.Equal(t, "a", params.OrgID)
	require.Equal(t, "2026-01-01T00:00:00Z", params.Since)
	require.Equal(t, "2026-06-01T00:00:00Z", params.Until)
	require.Equal(t, "*.delete", params.Action)
	require.Equal(t, "observation/abc", params.Resource)
	require.Equal(t, "abc123", params.Cursor)
	require.Equal(t, 100, params.Limit)
}

func TestCursor_EncodeDecode(t *testing.T) {
	ts := time.Date(2026, 6, 1, 12, 0, 0, 0, time.UTC)
	c := admin.AuditCursor{OccurredAt: ts, ID: 12345}
	encoded := admin.EncodeCursor(c)
	require.NotEmpty(t, encoded)

	decoded, err := admin.DecodeCursor(encoded)
	require.NoError(t, err)
	require.True(t, decoded.OccurredAt.Equal(ts), "timestamps should match")
	require.Equal(t, int64(12345), decoded.ID)
}

func TestCursor_DecodeInvalid(t *testing.T) {
	_, err := admin.DecodeCursor("not-base64!!!")
	require.Error(t, err)
}

func TestIntParam(t *testing.T) {
	tests := []struct {
		input string
		want  int
		err   bool
	}{
		{"", 0, true},
		{"abc", 0, true},
		{"0", 0, false},
		{"50", 50, false},
		{"200", 200, false},
	}
	for _, tt := range tests {
		got, err := admin.ParseIntParam(tt.input)
		if tt.err {
			require.Error(t, err, "input: %q", tt.input)
		} else {
			require.NoError(t, err, "input: %q", tt.input)
			require.Equal(t, tt.want, got)
		}
	}
}
