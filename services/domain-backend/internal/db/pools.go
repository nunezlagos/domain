// Package db — separación tipada de connection pools.
//
// Domain usa DOS pools distintos por rol de seguridad. Esto es defense in
// depth, no optimización. Los handlers se vuelven explícitos sobre qué tipo
// de query están haciendo.
//
//	AppPool  → connection como app_user (NOBYPASSRLS).
//	           Para todas las queries de runtime sobre tablas con o sin RLS.
//	           Si la tabla tiene RLS habilitado, el handler DEBE usar
//	           txctx.WithOrgTx (o WithUserTx) antes de hacer queries.
//
//	AuthPool → connection como app_admin (BYPASSRLS).
//	           SOLO para queries del path de auth donde org_id aún no se conoce:
//	           - apikey.PGStore.Resolve (lookup api_keys por prefix)
//	           - apikey.PGStore.Issue   (INSERT api_keys post-verify-otp)
//	           - otp.Service.Request    (lookup users por email)
//	           - audit.PGRecorder       (audit_log INSERT — orto, INSERT policy
//	                                     ya es WITH CHECK true, pero por consistencia
//	                                     queries de SELECT cross-org las hace audit
//	                                     reader como app_admin)
//
// En PRODUCCIÓN: DSN de cada pool usa user= diferente (app_user vs app_admin).
// EN TESTS: ambos pools sobre el mismo container, configurados con SET ROLE
// via AfterConnect del pgxpool — equivalente funcional.
package db

import (
	"context"
	"errors"
	"fmt"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

// Pools agrupa los pools del proceso.
//
// - App: primary read-write, user app_user (NOBYPASSRLS).
// - Auth: primary read-write, user app_admin (BYPASSRLS) para auth+audit.
// - ReadOnly: opcional, apunta a read replica para queries pesadas tolerantes
//   de stale-read (issue-25.9). Si es nil, Read() fallback a App.
type Pools struct {
	App        *pgxpool.Pool
	Auth       *pgxpool.Pool
	ReadOnly   *pgxpool.Pool
	LagMonitor *LagMonitor
}

// Close cierra los pools de forma ordenada.
func (p *Pools) Close() {
	if p.App != nil {
		p.App.Close()
	}
	if p.Auth != nil && p.Auth != p.App {
		p.Auth.Close()
	}
	if p.ReadOnly != nil && p.ReadOnly != p.App {
		p.ReadOnly.Close()
	}
}

// Read retorna el pool preferido para queries SELECT pesadas.
// Si hay ReadOnly disponible y el LagMonitor no marca degraded, usa ReadOnly.
// Sino, fallback transparente a App. Las queries deben tolerar stale-read <2s.
func (p *Pools) Read() *pgxpool.Pool {
	if p.ReadOnly != nil && (p.LagMonitor == nil || !p.LagMonitor.IsDegraded()) {
		return p.ReadOnly
	}
	return p.App
}

// ReadFresh siempre retorna App (primary). Para queries que NO toleran stale-read.
func (p *Pools) ReadFresh() *pgxpool.Pool {
	return p.App
}

// OpenProduction abre dos pools desde DSNs separadas. En prod los DSN tienen
// user=app_user y user=app_admin respectivamente. Si authDSN está vacío, se
// asume que appDSN ya apunta a un user con BYPASSRLS (NO recomendado, solo
// para development local single-user).
func OpenProduction(ctx context.Context, appDSN, authDSN string) (*Pools, error) {
	if appDSN == "" {
		return nil, errors.New("appDSN required")
	}
	app, err := pgxpool.New(ctx, appDSN)
	if err != nil {
		return nil, fmt.Errorf("open app pool: %w", err)
	}
	if authDSN == "" {
		// dev fallback: reutilizar app pool. Documentado en .env.example.
		return &Pools{App: app, Auth: app}, nil
	}
	auth, err := pgxpool.New(ctx, authDSN)
	if err != nil {
		app.Close()
		return nil, fmt.Errorf("open auth pool: %w", err)
	}
	return &Pools{App: app, Auth: auth}, nil
}

// OpenProductionWithReplica extiende OpenProduction abriendo además un pool
// read-only contra la replica (issue-25.9). Si readDSN está vacío, ReadOnly=nil
// y Read() fallback a App.
func OpenProductionWithReplica(ctx context.Context, appDSN, authDSN, readDSN string) (*Pools, error) {
	pools, err := OpenProduction(ctx, appDSN, authDSN)
	if err != nil {
		return nil, err
	}
	if readDSN == "" {
		return pools, nil
	}
	ro, err := pgxpool.New(ctx, readDSN)
	if err != nil {
		pools.Close()
		return nil, fmt.Errorf("open readonly pool: %w", err)
	}
	pools.ReadOnly = ro
	return pools, nil
}

// OpenWithRoleOverride es el helper para tests: un solo container, dos pools
// con SET ROLE distinto. El bootstrapDSN debe poder ejecutar GRANT (i.e. ser
// el dueño del rol o superuser). En producción NO se usa.
func OpenWithRoleOverride(ctx context.Context, dsn, appRole, authRole string) (*Pools, error) {
	bootstrap, err := pgxpool.New(ctx, dsn)
	if err != nil {
		return nil, fmt.Errorf("bootstrap pool: %w", err)
	}
	// Asegurar membership (idempotente)
	for _, role := range []string{appRole, authRole} {
		if _, err := bootstrap.Exec(ctx,
			fmt.Sprintf(`GRANT %s TO CURRENT_USER`, pgIdent(role))); err != nil {
			bootstrap.Close()
			return nil, fmt.Errorf("grant %s: %w", role, err)
		}
	}
	bootstrap.Close()

	openWithRole := func(role string) (*pgxpool.Pool, error) {
		cfg, err := pgxpool.ParseConfig(dsn)
		if err != nil {
			return nil, err
		}
		boundRole := role
		cfg.AfterConnect = func(ctx context.Context, conn *pgx.Conn) error {
			_, err := conn.Exec(ctx, "SET ROLE "+pgIdent(boundRole))
			return err
		}
		return pgxpool.NewWithConfig(ctx, cfg)
	}
	app, err := openWithRole(appRole)
	if err != nil {
		return nil, fmt.Errorf("open app role pool: %w", err)
	}
	auth, err := openWithRole(authRole)
	if err != nil {
		app.Close()
		return nil, fmt.Errorf("open auth role pool: %w", err)
	}
	return &Pools{App: app, Auth: auth}, nil
}

// pgIdent escapa un identificador Postgres (rolname). Lista cerrada de roles
// del proyecto; solo letras+underscore aceptado para defensa.
func pgIdent(name string) string {
	for _, r := range name {
		if !(r >= 'a' && r <= 'z') && !(r >= 'A' && r <= 'Z') && !(r >= '0' && r <= '9') && r != '_' {
			panic("invalid pg ident: " + name)
		}
	}
	return `"` + name + `"`
}
