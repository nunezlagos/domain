package install

import (
	"errors"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

// === ValidateDSN ===

func TestValidateDSN_ValidLocal(t *testing.T) {
	cases := []string{
		"postgres://user:pass@localhost:5432/db?sslmode=disable",
		"postgresql://user:pass@localhost:5432/db",
		"postgres://u:p@host:5432/d",
		"postgres://u%40domain:p@host:5432/d", // user con @ encoded
	}
	for _, dsn := range cases {
		t.Run(dsn, func(t *testing.T) {
			require.NoError(t, ValidateDSN(dsn))
		})
	}
}

func TestValidateDSN_RejectsEmpty(t *testing.T) {
	require.ErrorIs(t, ValidateDSN(""), ErrInvalidDSN)
}

func TestValidateDSN_RejectsBadScheme(t *testing.T) {
	cases := []string{
		"mysql://user:pass@host/db",
		"http://user:pass@host",
		"file:///etc/passwd",
	}
	for _, dsn := range cases {
		t.Run(dsn, func(t *testing.T) {
			err := ValidateDSN(dsn)
			require.Error(t, err)
			require.True(t, errors.Is(err, ErrInvalidDSN),
				"err debe contener ErrInvalidDSN, got %v", err)
		})
	}
}

func TestValidateDSN_RejectsMissingPassword(t *testing.T) {
	err := ValidateDSN("postgres://user@host:5432/db")
	require.Error(t, err)
	require.True(t, errors.Is(err, ErrInvalidDSN),
		"err debe contener ErrInvalidDSN, got %v", err)
}

func TestValidateDSN_RejectsMissingUser(t *testing.T) {
	err := ValidateDSN("postgres://host:5432/db")
	require.Error(t, err)
}

func TestValidateDSN_RejectsMissingHost(t *testing.T) {
	err := ValidateDSN("postgres://user:pass@/db")
	require.Error(t, err)
}

// === Cloud provider detection ===

func TestValidateDSN_RejectsPlaintextInCloud(t *testing.T) {
	cases := []struct {
		dsn    string
		expect string
	}{
		{"postgres://u:p@db.neon.tech:5432/x?sslmode=disable", "neon"},
		{"postgres://u:p@saargo-pool.cluster-abc.rds.amazonaws.com:5432/x?sslmode=disable", "rds"},
		{"postgres://u:p@saargo.supabase.co:5432/x?sslmode=allow", "supabase"},
		{"postgres://u:p@saargo.heroku.com:5432/x?sslmode=prefer", "heroku"},
		{"postgres://u:p@db.neon.tech:5432/x", "neon (no sslmode)"},
	}
	for _, tc := range cases {
		t.Run(tc.expect, func(t *testing.T) {
			err := ValidateDSN(tc.dsn)
			require.Error(t, err)
			require.True(t, errors.Is(err, ErrPlaintextDSNInCloud),
				"dsn %q debe ser rechazada (cloud provider %s), got %v", tc.dsn, tc.expect, err)
		})
	}
}

func TestValidateDSN_AcceptsTLSInCloud(t *testing.T) {
	cases := []string{
		"postgres://u:p@db.neon.tech:5432/x?sslmode=require",
		"postgres://u:p@db.neon.tech:5432/x?sslmode=verify-full",
		"postgres://u:p@saargo-pool.cluster-abc.rds.amazonaws.com:5432/x?sslmode=require",
	}
	for _, dsn := range cases {
		t.Run(dsn, func(t *testing.T) {
			require.NoError(t, ValidateDSN(dsn))
		})
	}
}

// === Mode validation ===

func TestMode_IsValid(t *testing.T) {
	for _, m := range []Mode{ModeLocal, ModeCloud, ModeHybrid} {
		t.Run(string(m), func(t *testing.T) {
			require.NotEmpty(t, m)
		})
	}
}

// === HybridConfig ===

func TestHybridConfig_Plan(t *testing.T) {
	cases := []struct {
		name string
		cfg  HybridConfig
		want []string
	}{
		{
			"all local",
			HybridConfig{Postgres: SvcLocal, S3: SvcLocal, SMTP: SvcLocal},
			[]string{"postgres", "minio", "mailpit"},
		},
		{
			"all cloud",
			HybridConfig{Postgres: SvcCloud, S3: SvcCloud, SMTP: SvcCloud},
			[]string{},
		},
		{
			"postgres local, rest cloud",
			HybridConfig{Postgres: SvcLocal, S3: SvcCloud, SMTP: SvcCloud},
			[]string{"postgres"},
		},
		{
			"smtp local only",
			HybridConfig{Postgres: SvcCloud, S3: SvcCloud, SMTP: SvcLocal},
			[]string{"mailpit"},
		},
		{
			"none disables service",
			HybridConfig{Postgres: SvcNone, S3: SvcNone, SMTP: SvcNone},
			[]string{},
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := tc.cfg.Plan()
			require.Equal(t, tc.want, got)
		})
	}
}

func TestLocalServices(t *testing.T) {
	got := LocalServices()
	require.Len(t, got, 3)
	require.Contains(t, got, "postgres")
	require.Contains(t, got, "minio")
	require.Contains(t, got, "mailpit")
}

// === InstallState ===

func TestInstallState_Summary(t *testing.T) {
	s := &InstallState{
		CredentialsExist: true,
		DockerAvailable:  true,
		DockerRunning:    true,
		ServerReachable:  true,
		FirstRun:         false,
		UserCount:        3,
		BaseURL:          "http://localhost:8000",
	}
	summary := s.Summary()
	require.Contains(t, summary, "creds=yes")
	require.Contains(t, summary, "docker=running")
	require.Contains(t, summary, "server=reachable")
	require.Contains(t, summary, "users=3")
}

// === CredentialsPath ===

func TestCredentialsPath(t *testing.T) {
	p := CredentialsPath()
	require.Contains(t, p, ".config")
	require.Contains(t, p, "domain")
	require.Contains(t, p, "credentials.json")
}

// === WaitHealthy timeout ===

func TestWaitHealthy_EmptyServices_ReturnsImmediately(t *testing.T) {
	// Sin servicios, retorna nil inmediatamente (no hay nada que esperar).
	require.NoError(t, WaitHealthy(t.Context(), nil, 1*time.Second))
	require.NoError(t, WaitHealthy(t.Context(), []string{}, 1*time.Second))
}
